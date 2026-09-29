// Closing an execution record whose worktree was reclaimed outside gz-git.
//
// Split from state.go deliberately. deriveState is a pure read: it observes
// Git and the Worktrunk inventory and returns a verdict. This path writes --
// it appends a terminal receipt and takes the mutation lock to do it. Keeping
// the only write on the status path in its own file keeps that difference
// visible to the next reader.
//
// ADR-0055 puts runtime state on gz-git's side of the boundary, next to the
// Git state it describes. `gz-git integrate` is still allowed to remove the
// worktree, the local branch and the remote branch without telling the task
// runtime anything, and it does. The record it leaves behind derives BLOCKED
// forever (state.go), and listResponse rolls one blocked record up into a
// repository-wide failure. The fix is the runtime recognizing its own
// reclaimed record. service_abort.go's doc comment already named this exact
// scenario -- the code knew the situation and left the exit manual.

package runtask

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// landing is the positive evidence that an orphaned record's work reached its
// source branch. Absence of a worktree is never part of it: a person who
// deletes a worktree by mistake produces exactly that absence, which is why
// the missing directory is only the trigger for looking, never the answer.
type landing struct {
	reason      string
	diagnostics []CommandDiagnostics
}

// declined is the state returned alongside a false verdict. It carries no
// status -- the caller derives that itself -- but it does carry the probes
// that were run, so that a record which stays BLOCKED can be asked why
// reconciliation declined it. Without this the safety condition is the one
// branch of the feature that leaves no trace when it fires.
func declined(evidence landing) DerivedTaskState {
	return DerivedTaskState{Diagnostics: evidence.diagnostics}
}

// reconcileReclaimed closes a record whose worktree is gone and whose work is
// provably accounted for, and reports whether it decided anything.
//
// Ownership is not checked, unlike Abort. Abort records a human judgement, so
// it must be the owner's judgement; this records a fact about Git that reads
// the same for everyone, and the cost of an unreconciled record -- run-list
// failing repository-wide -- falls on everyone too.
//
// A false return means "no verdict here", and the caller derives state exactly
// as before. That covers both the no-evidence case, which is the safety
// condition doing its job, and the mutation lock already being held: Finish
// calls into status while holding it, and a record whose worktree is missing
// cannot finish anyway.
func (s *Service) reconcileReclaimed(ctx context.Context, execution TaskExecution, cfg TaskRuntimeConfig) (DerivedTaskState, bool) {
	if _, err := os.Stat(execution.Worktree); !os.IsNotExist(err) {
		return DerivedTaskState{}, false
	}
	evidence, landed := s.landing(ctx, execution, cfg)
	if !landed {
		return declined(evidence), false
	}
	state := DerivedTaskState{Execution: execution, Diagnostics: evidence.diagnostics}
	store, err := s.store(ctx)
	if err != nil {
		return declined(evidence), false
	}
	unlock, err := store.lock()
	if err != nil {
		return declined(evidence), false
	}
	defer unlock()
	// The landing evidence above was gathered outside the lock, so it is a
	// statement about a moment that has already passed. Abort orders the same
	// two steps the other way -- lock, then read, then decide
	// (service_abort.go) -- and this path must match it: a concurrent run
	// abort that lands between the evidence and the lock has already
	// written the terminal receipt, and appending a second one would record
	// the same closure twice under two different reasons. Re-read here and
	// leave the decision to whoever got there first.
	receipts, err := store.latestReceipts()
	if err != nil {
		return declined(evidence), false
	}
	if previous, recorded := receipts[execution.Task]; recorded && terminalReceiptStatus(previous.Status) {
		return declined(evidence), false
	}
	// The terminal receipt asks "has someone already closed this"; this asks
	// the other question, "is the premise still true". run-start recreates the
	// worktree and appends an ACTIVE receipt, which is not terminal, so the
	// check above waves it through -- and the record it would then close is a
	// live execution whose worktree is back.
	if _, err := os.Stat(execution.Worktree); !os.IsNotExist(err) {
		return declined(evidence), false
	}
	receipt := TaskReceipt{
		Task: execution.Task, Operation: "abort", Status: TaskExecutionStatusAborted,
		Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree,
		Reason: evidence.reason, CreatedAt: s.now().UTC(), WorktreeRemoved: true,
		Diagnostics: evidence.diagnostics,
	}
	if err := store.appendReceipt(receipt); err != nil {
		state.Status = TaskExecutionStatusBlocked
		state.AllowedActions = []string{"run-status", "run-abort"}
		state.Reason = "reconciliation evidence is complete but the receipt could not be persisted: " + err.Error()
		state.NextAction = "repair task runtime storage and retry"
		return state, true
	}
	state.Status = TaskExecutionStatusAborted
	state.AllowedActions = []string{"run-status"}
	state.Reason, state.NextAction = evidence.reason, "no action required"
	return state, true
}

// landing looks for proof that the branch behind a vanished worktree is
// accounted for in its source branch.
//
// Two shapes count, and they are not interchangeable:
//
//   - A branch ref still resolves -- locally, or as origin's remote-tracking
//     ref. Then the question has a direct answer, and it is asked directly:
//     is that tip contained in the source branch. A tip that is NOT contained
//     is the mistaken-deletion case, and it returns no landing, so the record
//     stays BLOCKED for a person to judge.
//
//     Be honest about how often this fires: this repository lands work by
//     rebase, which rewrites the hashes, so a surviving tip is not an
//     ancestor of the source even when its content is there. Containment
//     answers for merge landings and for a ref left behind by an integration
//     that did not rewrite it; the rebase case falls through to the shape
//     below, where the missing refs are the evidence. That makes the second
//     shape the working path here, not the exception.
//
//   - No branch ref resolves anywhere, and origin does not have the branch
//     either. Then worktree, local branch and remote branch are all gone --
//     the same triple the integration check demands before an execution may
//     be called DONE, and precisely what `gz-git integrate` leaves. There is
//     no tip left to test containment against, and that is not a gap in the
//     evidence: a human deleting a worktree by mistake does not also delete
//     the branch from two places, and if one did, nothing recoverable is being
//     closed over.
//
// Any other shape -- origin still carries the branch, a probe fails, git is
// unavailable -- yields nothing, which leaves the record exactly as blocked as
// it is today. The default is always to not close.
func (s *Service) landing(ctx context.Context, execution TaskExecution, cfg TaskRuntimeConfig) (landing, bool) {
	evidence := landing{}
	probe := func(args ...string) (CommandResult, bool) {
		result, err := s.command(ctx, s.root, "git", args...)
		evidence.diagnostics = append(evidence.diagnostics, diagnostic(result))
		return result, err == nil
	}
	resolves := func(ref string) bool {
		result, ok := probe("rev-parse", "--verify", "--quiet", ref)
		return ok && result.ExitCode == 0 && strings.TrimSpace(result.Stdout) != ""
	}
	source, err := s.landingSource(ctx, execution)
	if err != nil {
		return landing{diagnostics: evidence.diagnostics}, false
	}
	for _, ref := range []string{"refs/heads/" + execution.Branch, "refs/remotes/origin/" + execution.Branch} {
		if !resolves(ref) {
			continue
		}
		// Every source ref that resolves is asked, not just the first. A
		// checkout that never pulls keeps a local trunk behind origin's, and
		// stopping at it would answer "not contained" about a ref that has
		// merely not caught up -- leaving the orphan blocked forever in
		// exactly the control checkouts that cannot notice.
		for _, sourceRef := range []string{"refs/heads/" + source, "refs/remotes/origin/" + source} {
			if !resolves(sourceRef) {
				continue
			}
			result, ok := probe("merge-base", "--is-ancestor", ref, sourceRef)
			if !ok || result.ExitCode != 0 {
				continue
			}
			evidence.reason = fmt.Sprintf("execution %s closed by gz-git reconciliation: its worktree %s is gone and %s is contained in %s",
				execution.Task, execution.Worktree, ref, sourceRef)
			return evidence, true
		}
		return landing{diagnostics: evidence.diagnostics}, false
	}
	// Asking origin is a network call, and a repository that declared
	// no-fetch said not to make one. Under that policy the only admissible
	// evidence is the local-ref branch above, so an orphan whose refs are all
	// gone stays blocked rather than being closed on a probe gz-git was told
	// not to run.
	if cfg.IntegrationNetworkPolicy == IntegrationNetworkPolicyNoFetch {
		return landing{diagnostics: evidence.diagnostics}, false
	}
	// --exit-code makes "no such head" exit 2, distinct from the 1 that a
	// genuine failure would give; 0 means origin still carries the branch,
	// so the work is still out there and this is not a completed reclaim.
	result, ok := probe("ls-remote", "--exit-code", "--heads", "origin", execution.Branch)
	if !ok || result.ExitCode != 2 {
		return landing{diagnostics: evidence.diagnostics}, false
	}
	evidence.reason = fmt.Sprintf("execution %s closed by gz-git reconciliation: its worktree %s, local branch %s and origin branch were all reclaimed outside gz-git",
		execution.Task, execution.Worktree, execution.Branch)
	return evidence, true
}

// landingSource names the branch the work had to reach. The record carries it,
// because run-start wrote down what it branched from; the repository-level
// declaration is the fallback for a record written before that field existed.
func (s *Service) landingSource(ctx context.Context, execution TaskExecution) (string, error) {
	if source := strings.TrimSpace(execution.Source); source != "" {
		return source, nil
	}
	source, err := s.sourceBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve source branch for execution %s: %w", execution.Task, err)
	}
	return source, nil
}
