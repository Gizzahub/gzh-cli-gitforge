// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// Cleanup-only retry (TASK-302). CleanupRetry finishes the reclaim half of an
// evidenced run that crashed after its push. It deliberately does not call
// Check, Run, readiness, watchers, merge, or a target push: the integration
// itself already happened, and re-running any part of it could push work the
// caller never approved. Every fact is re-proved from Git and the durable
// receipt before anything is removed.
var (
	// ErrCleanupReceiptMissing reports no receipt for that execution-id.
	ErrCleanupReceiptMissing = errors.New("no integrate receipt for that execution-id")
	// ErrCleanupGenerationMismatch reports a generation that is not the
	// receipt's exact current generation.
	ErrCleanupGenerationMismatch = errors.New("generation does not match the receipt")
	// ErrCleanupOwnerMismatch reports a caller whose resolved work identity is
	// not the receipt's owner.
	ErrCleanupOwnerMismatch = errors.New("caller identity is not the receipt owner")
	// ErrCleanupClaimLive reports a receipt whose last holder is positively
	// alive; its run may still mutate.
	ErrCleanupClaimLive = errors.New("receipt holder is alive; refusing to clean up")
	// ErrCleanupClaimUnknown reports a holder whose liveness cannot be
	// established. Unknown is never permission.
	ErrCleanupClaimUnknown = errors.New("receipt holder liveness is unknown; refusing to clean up")
	// ErrCleanupProofUnknown reports that containment of the task SHA in the
	// remote target could not be proven. Success is never fabricated.
	ErrCleanupProofUnknown = errors.New("cannot prove the remote target contains the task commit")
	// ErrCleanupNotPushed reports a receipt whose push never landed; the run
	// must be re-executed, not cleaned up.
	ErrCleanupNotPushed = errors.New("run never pushed; re-run integrate run instead of cleaning up")
	// ErrCleanupRefMoved reports a residual ref that no longer points at the
	// captured task SHA. The residuals are preserved.
	ErrCleanupRefMoved = errors.New("residual ref no longer points at the captured task commit")
	// ErrCleanupRegistration reports a worktree whose registration marker does
	// not bind it to this execution — e.g. a branch reused at the same commit
	// by a newer generation.
	ErrCleanupRegistration = errors.New("worktree registration does not match this execution")
	// ErrCleanupDirtyWorktree reports a worktree with uncommitted changes.
	ErrCleanupDirtyWorktree = errors.New("worktree has uncommitted changes")
	// ErrCleanupGuard reports a task branch that protection, task-pattern, or
	// controller guards refuse to reclaim.
	ErrCleanupGuard = errors.New("task branch is guarded from cleanup")
	// ErrCleanupPartial reports a cleanup that removed some residuals but did
	// not finish; the receipt keeps the durable partial results.
	ErrCleanupPartial = errors.New("cleanup incomplete; residuals preserved")
)

// CleanupOptions configures a cleanup-only retry.
type CleanupOptions struct {
	// RepoPath is any directory of the repository the receipt belongs to.
	RepoPath string
	// ExecutionID and Generation must match the receipt exactly.
	ExecutionID string
	Generation  string
	// LockNotice receives the periodic lock notice while waiting; nil means
	// os.Stderr.
	LockNotice io.Writer
}

// CleanupReport is the cleanup result. Complete is true only when every
// residual is gone — removed now, or proven already absent.
type CleanupReport struct {
	Receipt                  *Receipt
	Caller                   OwnerIdentity
	RemoteTargetContainsTask bool
	Actions                  []string
	ResidualsPreserved       []string
	Complete                 bool
}

// cleanupRun carries one cleanup attempt's resolved state through its phases.
// Every phase re-proves facts before the next one acts; none re-integrates.
type cleanupRun struct {
	opts       CleanupOptions
	exec       *gitcmd.Executor
	g          gitRepo
	common     string
	caller     OwnerIdentity
	targetName string
	endpoint   string
	report     *CleanupReport
}

// CleanupRetry reclaims residuals of an evidenced run without re-integrating.
// The invariant is that a refusal leaves every residual exactly as it was.
// Validation, proof, probing, and reclaim run as separate phases, but the
// ordering is fixed: every preflight — identity, claim, guards, remote proof,
// residual validation — happens before the first deletion, and each removal
// is followed by its durable receipt write.
func CleanupRetry(ctx context.Context, exec *gitcmd.Executor, opts CleanupOptions) (*CleanupReport, error) {
	run, err := newCleanupRun(ctx, exec, opts)
	if err != nil {
		return nil, err
	}
	taskFence, err := acquireTaskResourceFence(ctx, exec, run.g.dir, run.report.Receipt.TaskBranch, opts.LockNotice)
	if err != nil {
		return nil, err
	}
	defer taskFence.release()
	// The existing repo/target lock is the mutation authority for this
	// destination; the receipt CAS happens under it.
	lock, err := acquireTargetLock(ctx, run.g, run.report.Receipt.TargetRef, opts.LockNotice)
	if err != nil {
		return nil, err
	}
	defer lock.release()
	if err := run.revalidateUnderLock(ctx); err != nil {
		return nil, err
	}
	if err := run.proveRemoteTarget(ctx); err != nil {
		return nil, err
	}
	presence, err := run.validateResiduals(ctx)
	if err != nil {
		return nil, err
	}
	if run.report.Receipt.Phase == evidencePhaseComplete && (presence.worktree || presence.localBranch || presence.remoteBranch) {
		return run.report, fmt.Errorf("%w: completed execution resources reappeared; refusing to delete them", ErrCleanupGuard)
	}
	// From the first durable reconciliation onward a failure still returns
	// the partial report, so exit 3 callers can read what was already done.
	if err := run.reconcileAbsentResiduals(ctx, presence); err != nil {
		return run.report, err
	}
	if err := run.removeResiduals(ctx, presence); err != nil {
		return run.report, err
	}
	return run.finish(ctx, presence)
}

// newCleanupRun validates options, resolves the repository, and authorizes
// the caller against the durable receipt — all before any lock or mutation.
func newCleanupRun(ctx context.Context, exec *gitcmd.Executor, opts CleanupOptions) (*cleanupRun, error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
	}
	if err := validateOpaqueToken(opts.ExecutionID, "execution-id"); err != nil {
		return nil, err
	}
	if err := validateOpaqueToken(opts.Generation, "generation"); err != nil {
		return nil, err
	}
	dir := strings.TrimSpace(opts.RepoPath)
	if dir == "" {
		dir = "."
	}
	g := newGitRepo(exec, dir)
	if !g.isRepo(ctx) {
		return nil, fmt.Errorf("not a git repository: %s", dir)
	}
	root, err := g.toplevel(ctx)
	if err != nil {
		return nil, err
	}
	g.dir = root
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common == "" {
		if err == nil {
			err = fmt.Errorf("git common directory is empty")
		}
		return nil, fmt.Errorf("resolve common directory: %w", err)
	}
	// Owner is verified before any lock or mutation: cleanup is authorized by
	// the caller being the recorded owner, resolved through the native work
	// identity chain — never a guessed hostname.
	caller, err := ResolveCallerOwner(ctx, exec)
	if err != nil {
		return nil, fmt.Errorf("resolve caller identity: %w", err)
	}
	receipt, ok, err := readReceipt(common, opts.ExecutionID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrCleanupReceiptMissing, opts.ExecutionID)
	}
	if receipt.SchemaVersion != receiptSchemaVersion {
		return nil, fmt.Errorf("%w: schema_version %d permits receipt inspection only; cleanup requires v2 captured protections", ErrReceiptReadOnly, receipt.SchemaVersion)
	}
	if err := cleanupIdentityCAS(receipt, opts, caller); err != nil {
		return nil, err
	}
	return &cleanupRun{
		opts:   opts,
		exec:   exec,
		g:      g,
		common: common,
		caller: caller,
		report: &CleanupReport{Receipt: receipt, Caller: caller},
	}, nil
}

// revalidateUnderLock re-reads the receipt after target ownership is held:
// a run may have rewritten it while cleanup waited. The re-read must still be
// the exact identity the lock was taken for — the same target ref and every
// immutable field; a receipt rebound to another destination means this lock
// guards the wrong ref. The claim check and guards follow; a live or
// unknowable holder refuses before any proof or deletion.
func (r *cleanupRun) revalidateUnderLock(ctx context.Context) error {
	lockedRead := r.report.Receipt
	receipt, ok, err := readReceipt(r.common, r.opts.ExecutionID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrCleanupReceiptMissing, r.opts.ExecutionID)
	}
	if err := cleanupRereadGuard(lockedRead, receipt); err != nil {
		return err
	}
	if err := cleanupIdentityCAS(receipt, r.opts, r.caller); err != nil {
		return err
	}
	targetName, err := receiptTargetBranch(receipt)
	if err != nil {
		return err
	}
	if err := refuseLiveOrUnknownClaim(receipt); err != nil {
		return err
	}
	if err := cleanupGuards(receipt, targetName); err != nil {
		return err
	}
	r.targetName = targetName
	r.endpoint, err = receiptPushEndpoint(ctx, r.g, receipt)
	if err != nil {
		return fmt.Errorf("%w: resolve push endpoint: %w", ErrCleanupProofUnknown, err)
	}
	r.report.Receipt = receipt
	return nil
}

// proveRemoteTarget proves the live remote target contains the captured task
// SHA before anything is removed. A definitive negative on an unpushed
// receipt is the crash window before the push: the residuals hold the only
// copy of the work, so cleanup refuses and the run must be re-executed. A
// crash between the push and its receipt update left the durable evidence
// behind the fact; that flag is reconciled here under the lock, without
// re-pushing.
func (r *cleanupRun) proveRemoteTarget(ctx context.Context) error {
	contains, err := proveRemoteTargetContainsTask(ctx, r.g, r.report.Receipt, r.targetName, r.endpoint)
	if err != nil {
		if !r.report.Receipt.SourcePushed && errors.Is(err, errTargetNotContaining) {
			return fmt.Errorf("%w: %s phase=%s: %w", ErrCleanupNotPushed, r.opts.ExecutionID, r.report.Receipt.Phase, err)
		}
		return err
	}
	r.report.RemoteTargetContainsTask = contains
	if r.report.Receipt.SourcePushed {
		return nil
	}
	r.report.Receipt.SourcePushed = true
	r.report.Receipt.Phase = evidencePhasePushed
	if err := writeReceipt(ctx, r.common, r.report.Receipt); err != nil {
		return err
	}
	r.report.Actions = append(r.report.Actions, "reconciled source_pushed from remote proof")
	return nil
}

// residualPresence records which residuals still exist after validation.
type residualPresence struct {
	worktree, localBranch, remoteBranch bool
}

// validateResiduals proves every residual identity before removing anything:
// a mismatch preserves all residuals, including the ones already proven
// removable.
func (r *cleanupRun) validateResiduals(ctx context.Context) (residualPresence, error) {
	receipt := r.report.Receipt
	localSHA, localPresent, err := probeLocalBranch(ctx, r.g, receipt.TaskBranch)
	if err != nil {
		return residualPresence{}, err
	}
	if localPresent && localSHA != receipt.TaskSHA {
		return residualPresence{}, fmt.Errorf("%w: local %s is %s", ErrCleanupRefMoved, receipt.TaskBranch, localSHA)
	}
	worktreePresent, err := validateWorktreeResidual(ctx, r.exec, r.g, receipt)
	if err != nil {
		return residualPresence{}, err
	}
	remoteSHA, remotePresent, err := lsRemoteHead(ctx, r.g, r.endpoint, receipt.TaskBranch)
	if err != nil {
		return residualPresence{}, fmt.Errorf("%w: query remote task branch: %w", ErrCleanupProofUnknown, err)
	}
	if remotePresent && remoteSHA != receipt.TaskSHA {
		return residualPresence{}, fmt.Errorf("%w: remote %s is %s", ErrCleanupRefMoved, receipt.TaskBranch, remoteSHA)
	}
	return residualPresence{worktree: worktreePresent, localBranch: localPresent, remoteBranch: remotePresent}, nil
}

// reconcileAbsentResiduals turns successful absence probes into removal
// proof, so a receipt whose flags were never written can still complete
// instead of deadlocking as a partial with nothing left to remove.
func (r *cleanupRun) reconcileAbsentResiduals(ctx context.Context, presence residualPresence) error {
	receipt := r.report.Receipt
	reconciled := false
	if !presence.worktree && !receipt.WorktreeRemoved {
		receipt.WorktreeRemoved = true
		r.report.Actions = append(r.report.Actions, "worktree already absent")
		reconciled = true
	}
	if !presence.localBranch && !receipt.LocalBranchRemoved {
		receipt.LocalBranchRemoved = true
		r.report.Actions = append(r.report.Actions, "local-branch already absent")
		reconciled = true
	}
	if !presence.remoteBranch && !receipt.RemoteBranchRemoved {
		receipt.RemoteBranchRemoved = true
		r.report.Actions = append(r.report.Actions, "remote-branch already absent")
		reconciled = true
	}
	if !reconciled {
		return nil
	}
	if err := writeReceipt(ctx, r.common, receipt); err != nil {
		return fmt.Errorf("%w: persist reconciled residuals: %w", ErrCleanupPartial, err)
	}
	return nil
}

// removeResiduals removes each validated residual, recording every durable
// result as it happens. A failure stops the cleanup with the partial state
// recorded (exit 3) and residuals intact.
func (r *cleanupRun) removeResiduals(ctx context.Context, presence residualPresence) error {
	receipt := r.report.Receipt
	if presence.worktree {
		if _, err := validateWorktreeResidual(ctx, r.exec, r.g, receipt); err != nil {
			return fmt.Errorf("%w: revalidate worktree before removal: %w", ErrCleanupPartial, err)
		}
		res, err := r.g.run(ctx, "worktree", "remove", receipt.TaskWorktree)
		if err != nil || (res != nil && res.ExitCode != 0) {
			return fmt.Errorf("%w: worktree remove %s: %s", ErrCleanupPartial, receipt.TaskWorktree, gitCmdDetail(res, err))
		}
		receipt.WorktreeRemoved = true
		if err := writeReceipt(ctx, r.common, receipt); err != nil {
			return fmt.Errorf("%w: persist worktree removal: %w", ErrCleanupPartial, err)
		}
		r.report.Actions = append(r.report.Actions, "worktree("+receipt.TaskWorktree+")")
	}
	if presence.localBranch {
		if err := refuseCheckedOutTaskBranch(ctx, r.g, receipt.TaskBranch); err != nil {
			return fmt.Errorf("%w: refuse checked-out local delete: %w", ErrCleanupPartial, err)
		}
		ref := "refs/heads/" + receipt.TaskBranch
		if err := refuseSymbolicTaskRef(ctx, r.g, ref); err != nil {
			return fmt.Errorf("%w: refuse symbolic local delete: %w", ErrCleanupPartial, err)
		}
		res, err := r.g.run(ctx, "update-ref", "-d", "--no-deref", ref, receipt.TaskSHA)
		if err != nil || (res != nil && res.ExitCode != 0) {
			return fmt.Errorf("%w: expected-old local delete %s: %s", ErrCleanupPartial, receipt.TaskBranch, gitCmdDetail(res, err))
		}
		receipt.LocalBranchRemoved = true
		if err := writeReceipt(ctx, r.common, receipt); err != nil {
			return fmt.Errorf("%w: persist local branch removal: %w", ErrCleanupPartial, err)
		}
		r.report.Actions = append(r.report.Actions, "local-branch")
	}
	if presence.remoteBranch {
		ref := "refs/heads/" + receipt.TaskBranch
		lease := "--force-with-lease=" + ref + ":" + receipt.TaskSHA
		del, err := r.g.run(ctx, "push", lease, r.endpoint, ":"+ref)
		if err != nil || (del != nil && del.ExitCode != 0) {
			return fmt.Errorf("%w: leased remote delete %s: %s", ErrCleanupPartial, receipt.TaskBranch, gitCmdDetail(del, err))
		}
		receipt.RemoteBranchRemoved = true
		if err := writeReceipt(ctx, r.common, receipt); err != nil {
			return fmt.Errorf("%w: persist remote branch removal: %w", ErrCleanupPartial, err)
		}
		r.report.Actions = append(r.report.Actions, "remote-branch")
	}
	return nil
}

// finish completes the attempt: every removal proven records phase=complete;
// anything left is a durable partial naming the preserved residuals.
func (r *cleanupRun) finish(ctx context.Context, presence residualPresence) (*CleanupReport, error) {
	receipt := r.report.Receipt
	after, failures := verifyReclaimAbsence(ctx, r.g, receipt.TaskWorktree, receipt.TaskBranch, r.endpoint, nil)
	if len(failures) != 0 {
		r.report.ResidualsPreserved = residualNames(receipt, !after.WorktreeAbsent, !after.LocalBranchAbsent, !after.RemoteBranchAbsent)
		return r.report, fmt.Errorf("%w: %s", ErrCleanupPartial, strings.Join(failures, "; "))
	}
	if receipt.WorktreeRemoved && receipt.LocalBranchRemoved && receipt.RemoteBranchRemoved {
		if receipt.Phase != evidencePhaseComplete {
			receipt.Phase = evidencePhaseComplete
			if err := writeReceipt(ctx, r.common, receipt); err != nil {
				return r.report, fmt.Errorf("%w: persist cleanup completion: %w", ErrCleanupPartial, err)
			}
		}
		r.report.Complete = true
		return r.report, nil
	}
	r.report.ResidualsPreserved = residualNames(receipt, presence.worktree, presence.localBranch, presence.remoteBranch)
	return r.report, fmt.Errorf("%w: %s", ErrCleanupPartial, strings.Join(r.report.ResidualsPreserved, ", "))
}

// CleanupExitCode maps a cleanup error to the process exit contract: 0 only
// for a complete cleanup, 3 for partial removal or failed receipt persistence,
// and 2 for preflight refusals and execution errors. A write can fail after
// rename; retry reloads the durable receipt and re-proves actual Git absence.
func CleanupExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrCleanupPartial):
		return 3
	default:
		return 2
	}
}

// cleanupIdentityCAS is the exact generation/owner comparison cleanup runs
// both before and under the target lock.
func cleanupIdentityCAS(receipt *Receipt, opts CleanupOptions, caller OwnerIdentity) error {
	if receipt.Generation != opts.Generation {
		return fmt.Errorf("%w: receipt is %q, got %q", ErrCleanupGenerationMismatch, receipt.Generation, opts.Generation)
	}
	if !receipt.Owner.sameAs(caller) {
		return fmt.Errorf("%w: receipt is %s, caller resolved as %s", ErrCleanupOwnerMismatch, receipt.Owner, caller)
	}
	return nil
}

// cleanupRereadGuard proves the receipt re-read under the target lock is
// still the exact identity the lock was taken for: the same target ref, and
// no immutable field (generation, owner, repository, task, branch) rewritten
// while cleanup waited for ownership. A receipt rebound to another
// destination means the held lock guards the wrong ref, so cleanup refuses
// before any proof or deletion.
func cleanupRereadGuard(locked, reread *Receipt) error {
	if locked.TargetRef != reread.TargetRef {
		return fmt.Errorf("%w: receipt target_ref moved from %s to %s while waiting for target ownership", ErrEvidenceReplay, locked.TargetRef, reread.TargetRef)
	}
	if conflicts := locked.identityConflicts(reread); len(conflicts) > 0 {
		return fmt.Errorf("%w: receipt %s changed while waiting for target ownership", ErrEvidenceReplay, strings.Join(conflicts, ", "))
	}
	return nil
}
