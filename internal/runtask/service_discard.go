// Guarded reclamation of an unintegrated task execution.

package runtask

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Discard removes the exact clean worktree recorded for an unintegrated task.
// It deliberately uses Worktrunk, and writes an intent receipt before asking
// Worktrunk to mutate anything, so an interrupted reclaim remains auditable
// and can be retried from the recorded identity.
func (s *Service) Discard(ctx context.Context, task, reason, takeOverFrom string) (resp TaskResponse, code int) {
	task, reason = normalize(task), strings.TrimSpace(reason)
	if task == "" || reason == "" {
		return response(TaskExecutionStatusBlocked, task, "task and non-empty discard reason are required", "gz-git run discard <task> --reason R"), 1
	}
	store, err := s.store(ctx)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor"), 1
	}
	unlock, err := store.lock()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "inspect the mutation lock with run-doctor"), 1
	}
	defer unlock()
	executions, err := store.load()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "inspect task runtime metadata"), 1
	}
	receipts, err := store.latestReceipts()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, "read task receipts: "+err.Error(), "inspect task runtime receipt evidence"), 1
	}
	execution, found := findExecution(executions, task)
	if !found {
		return response(TaskExecutionStatusUnknown, task, "task execution was not found", "gz-git run list", "run-list"), 1
	}
	if resp, code, gated := discardReceiptGate(task, execution, receipts); gated {
		return resp, code
	}
	current, err := s.owner(ctx)
	if err != nil {
		res := response(TaskExecutionStatusBlocked, task, "resolve current task owner: "+err.Error(), identityNextAction(err))
		appendIdentityDiagnostic(&res, err)
		return res, 1
	}
	performedBy, err := discardPerformer(execution.Owner, current, takeOverFrom)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "run as the recorded owner or provide the exact --take-over-from actor/host"), 1
	}
	cfg, err := s.config()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "repair task runtime configuration"), 1
	}
	if cfg.IntegrationNetworkPolicy == IntegrationNetworkPolicyNoFetch {
		return response(TaskExecutionStatusBlocked, task, "run-discard requires integration-network-policy allow because no-fetch cannot prove remote branch absence", "set integration-network-policy to allow, then retry run-discard"), 1
	}
	worktree, blocked := s.resolveDiscardWorktree(cfg, executions, receipts, execution)
	if blocked != nil {
		return *blocked, 1
	}
	if resp, code, done := s.discardExistingWorktree(ctx, store, execution, task, reason, performedBy, cfg, worktree, receipts); done {
		return resp, code
	}
	inventory, err := s.worktrunkList(ctx)
	if err != nil {
		r := response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor")
		r.Diagnostics = []CommandDiagnostics{diagnostic(inventory.Diagnostic)}
		return r, 1
	}
	if actual, ok := inventory.Paths[execution.Branch]; !ok || actual != execution.Worktree {
		return response(TaskExecutionStatusBlocked, task, "Worktrunk inventory does not exactly match execution branch and path", "inspect Worktrunk and runtime metadata"), 1
	}
	diagnostics, err := s.discardPreconditions(ctx, execution, worktree, cfg)
	if err != nil {
		r := response(TaskExecutionStatusBlocked, task, err.Error(), "resolve the Git precondition and retry")
		r.Diagnostics = diagnostics
		return r, 1
	}
	intent := discardReceipt(execution, TaskExecutionStatusBlocked, reason, performedBy, diagnostics, "discard intent recorded before Worktrunk removal", s.now().UTC())
	if err := store.appendReceipt(intent); err != nil {
		return response(TaskExecutionStatusBlocked, task, "persist discard intent receipt failed: "+err.Error(), "repair task runtime storage and retry"), 1
	}
	removed, runErr := s.command(ctx, s.root, "wt", "remove", "--no-hooks", "--foreground", "--format=json", "-y", "-D", execution.Worktree)
	diagnostics = append(diagnostics, diagnostic(removed))
	if runErr != nil || removed.ExitCode != 0 {
		return s.discardRemovalFailure(store, execution, task, performedBy, diagnostics, runErr), 1
	}
	return s.finishDiscard(ctx, store, execution, task, reason, "discard completed", "discard completed but receipt persistence failed: ", "repair task runtime storage and retry gz-git run discard; it will re-verify removal", performedBy, cfg, diagnostics)
}

// discardReceiptGate refuses a discard a receipt already answers: an
// idempotent re-discard of a proven discard exits 0, while a DONE receipt or
// a recorded cleanup failure makes removal either unnecessary or destructive.
func discardReceiptGate(task string, execution TaskExecution, receipts map[string]TaskReceipt) (TaskResponse, int, bool) {
	if previous := receipts[task]; terminalAbortReceipt(previous) && previous.Operation == "discard" && discardIdentityMatches(execution, previous) {
		r := response(TaskExecutionStatusAborted, task, "task execution is already discarded: "+previous.Reason, "no action required", "run-status")
		r.Execution, r.Receipt = &execution, &previous
		return r, 0, true
	}
	if previous := receipts[task]; previous.Status == TaskExecutionStatusDone {
		return response(TaskExecutionStatusBlocked, task, "run-discard refuses an execution with a DONE receipt", "inspect task runtime receipt evidence"), 1, true
	}
	if previous := receipts[task]; isPartialCleanupReceipt(execution, previous) {
		return response(TaskExecutionStatusBlocked, task, "run-discard refuses an execution with recorded integration cleanup failure", "use gz-git run recover to preserve the integrated outcome"), 1, true
	}
	return TaskResponse{}, 0, false
}

// resolveDiscardWorktree re-checks the recorded path against configuration,
// live executions, and the control working directory before anything
// mutates. A non-nil response is the refusal to return instead.
func (s *Service) resolveDiscardWorktree(cfg TaskRuntimeConfig, executions []TaskExecution, receipts map[string]TaskReceipt, execution TaskExecution) (string, *TaskResponse) {
	worktreeRoot, err := s.worktreeRoot(cfg, execution.Owner.Host)
	if err != nil {
		r := response(TaskExecutionStatusBlocked, execution.Task, "resolve recorded owner worktree root: "+err.Error(), "repair task runtime configuration")
		return "", &r
	}
	worktree, err := canonicalizePotentialPath(execution.Worktree)
	if err != nil {
		r := response(TaskExecutionStatusBlocked, execution.Task, "resolve recorded worktree path: "+err.Error(), "inspect task runtime metadata")
		return "", &r
	}
	expected := expectedWorktreePath(worktreeRoot, execution.Branch)
	if worktree != expected || !within(worktree, worktreeRoot) {
		r := response(TaskExecutionStatusBlocked, execution.Task, "execution worktree is outside its configured owner path", "inspect stale or tampered execution metadata")
		return "", &r
	}
	if conflict := conflictingDiscardExecution(executions, receipts, execution); conflict != "" {
		r := response(TaskExecutionStatusBlocked, execution.Task, "task execution conflicts with live execution "+conflict, "resolve duplicate runtime metadata before discarding")
		return "", &r
	}
	cwd, err := os.Getwd()
	if err != nil {
		r := response(TaskExecutionStatusBlocked, execution.Task, "resolve control working directory: "+err.Error(), "run from the primary/control checkout")
		return "", &r
	}
	cwd, err = canonicalizePotentialPath(cwd)
	if err != nil {
		r := response(TaskExecutionStatusBlocked, execution.Task, "resolve control working directory: "+err.Error(), "run from the primary/control checkout")
		return "", &r
	}
	if within(cwd, worktree) {
		r := response(TaskExecutionStatusBlocked, execution.Task, "run-discard refuses to remove its current worktree", "run from the primary/control checkout")
		return "", &r
	}
	return worktree, nil
}

// discardExistingWorktree inspects the recorded worktree path and answers
// for every state where no fresh removal should run: gone under a live
// intent receipt finishes the discard from that intent, an unexplained stat
// failure refuses, and a gone path without an intent receipt is unavailable
// to this verb. done=false means the worktree is present and fresh removal
// proceeds.
func (s *Service) discardExistingWorktree(ctx context.Context, store stateStore, execution TaskExecution, task, reason string, performedBy *TaskOwner, cfg TaskRuntimeConfig, worktree string, receipts map[string]TaskReceipt) (resp TaskResponse, code int, done bool) {
	_, worktreeErr := os.Stat(worktree)
	if worktreeErr != nil && !os.IsNotExist(worktreeErr) {
		return response(TaskExecutionStatusBlocked, task, "inspect recorded worktree: "+worktreeErr.Error(), "inspect the recorded worktree before retrying"), 1, true
	}
	if os.IsNotExist(worktreeErr) && isDiscardIntent(execution, receipts[task]) {
		resp, code = s.finishDiscard(ctx, store, execution, task, reason, "discard completion recovered from prior intent", "discard cleanup is proven but receipt persistence failed: ", "repair task runtime storage and retry", performedBy, cfg, nil)
		return resp, code, true
	}
	if worktreeErr != nil {
		return response(TaskExecutionStatusBlocked, task, "recorded worktree is unavailable: "+worktreeErr.Error(), "inspect the recorded worktree before retrying"), 1, true
	}
	return TaskResponse{}, 0, false
}

// finishDiscard proves the recorded removal and appends the terminal receipt.
// It serves both the intent-recovery path (nothing left to remove; the prior
// intent receipt said why) and the fresh removal path; completion names the
// proven outcome, and persistFailed and persistNext keep the two callers'
// storage-failure sentences distinct for whoever reads the receipt log later.
func (s *Service) finishDiscard(ctx context.Context, store stateStore, execution TaskExecution, task, reason, completion, persistFailed, persistNext string, performedBy *TaskOwner, cfg TaskRuntimeConfig, diagnostics []CommandDiagnostics) (resp TaskResponse, code int) {
	verified, verifyErr := s.discardRemovalEvidence(ctx, execution, diagnostics, cfg)
	if verifyErr != nil {
		return s.discardVerifyFailure(store, execution, task, performedBy, verified, verifyErr)
	}
	final := discardReceipt(execution, TaskExecutionStatusAborted, reason, performedBy, verified, completion, s.now().UTC())
	final.WorktreeRemoved, final.LocalBranchRemoved, final.RemoteBranchRemoved = true, true, true
	if err := store.appendReceipt(final); err != nil {
		r := response(TaskExecutionStatusBlocked, task, persistFailed+err.Error(), persistNext)
		r.Execution, r.Receipt, r.Diagnostics = &execution, &final, verified
		return r, 1
	}
	r := response(TaskExecutionStatusAborted, task, final.Reason, "no action required", "run-status")
	r.Execution, r.Receipt, r.Diagnostics = &execution, &final, verified
	return r, 0
}

// discardVerifyFailure records a blocked receipt for removal evidence that
// did not prove out, and answers with it.
func (s *Service) discardVerifyFailure(store stateStore, execution TaskExecution, task string, performedBy *TaskOwner, verified []CommandDiagnostics, verifyErr error) (resp TaskResponse, code int) {
	blocked := discardReceipt(execution, TaskExecutionStatusBlocked, verifyErr.Error(), performedBy, verified, "discard removal is incomplete or unproven", s.now().UTC())
	if err := store.appendReceipt(blocked); err != nil {
		verifyErr = fmt.Errorf("%w; persist failure evidence failed: %w", verifyErr, err)
	}
	r := response(TaskExecutionStatusBlocked, task, verifyErr.Error(), "inspect residual worktree or refs before retrying")
	r.Execution, r.Receipt, r.Diagnostics = &execution, &blocked, verified
	return r, 1
}

// discardRemovalFailure records the blocked receipt that keeps the intent
// alive after Worktrunk did not complete, and answers with it.
func (s *Service) discardRemovalFailure(store stateStore, execution TaskExecution, task string, performedBy *TaskOwner, diagnostics []CommandDiagnostics, runErr error) TaskResponse {
	removeReason := "Worktrunk removal did not complete"
	if runErr != nil {
		removeReason += ": " + runErr.Error()
	}
	blocked := discardReceipt(execution, TaskExecutionStatusBlocked, removeReason, performedBy, diagnostics, "discard intent retained after Worktrunk removal failure", s.now().UTC())
	if err := store.appendReceipt(blocked); err != nil {
		removeReason += "; persist failure evidence failed: " + err.Error()
	}
	r := response(TaskExecutionStatusBlocked, task, removeReason, "inspect Worktrunk diagnostics and retry")
	r.Execution, r.Receipt, r.Diagnostics = &execution, &blocked, diagnostics
	return r
}

func discardPerformer(recorded, current TaskOwner, takeOverFrom string) (*TaskOwner, error) {
	if sameOwner(recorded, current) {
		if strings.TrimSpace(takeOverFrom) != "" {
			return nil, fmt.Errorf("--take-over-from is only valid for a different actor")
		}
		performed := current
		return &performed, nil
	}
	if current.Host != recorded.Host || strings.TrimSpace(takeOverFrom) != recorded.String() {
		return nil, fmt.Errorf("only %s may discard this task; cross-actor takeover requires exact --take-over-from %s on host %s", recorded.String(), recorded.String(), recorded.Host)
	}
	performed := current
	return &performed, nil
}

func isDiscardIntent(execution TaskExecution, receipt TaskReceipt) bool {
	return receipt.Operation == "discard" && receipt.Status == TaskExecutionStatusBlocked && discardIdentityMatches(execution, receipt)
}

func conflictingDiscardExecution(executions []TaskExecution, receipts map[string]TaskReceipt, target TaskExecution) string {
	for _, other := range executions {
		if other.Task == target.Task || IsTerminalExecutionStatus(receipts[other.Task].Status) {
			continue
		}
		if other.Branch == target.Branch || filepath.Clean(other.Worktree) == filepath.Clean(target.Worktree) {
			return other.Task
		}
	}
	return ""
}

func (s *Service) discardPreconditions(ctx context.Context, execution TaskExecution, worktree string, cfg TaskRuntimeConfig) ([]CommandDiagnostics, error) {
	checks := []struct {
		args   []string
		reason string
	}{{[]string{"status", "--porcelain", "--untracked-files=all"}, "task worktree is dirty"}, {[]string{"branch", "--show-current"}, "recorded branch does not match the worktree branch"}, {discardRemoteProbe(execution.Branch, cfg), "task branch still exists on origin"}}
	diagnostics := make([]CommandDiagnostics, 0, len(checks))
	for i, check := range checks {
		dir := worktree
		if i == 2 {
			dir = s.root
		}
		result, err := s.command(ctx, dir, "git", check.args...)
		diagnostics = append(diagnostics, diagnostic(result))
		if err != nil {
			return diagnostics, fmt.Errorf("%s: %w", check.reason, err)
		}
		switch i {
		case 0:
			if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "" {
				return diagnostics, errors.New(check.reason)
			}
		case 1:
			if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != execution.Branch {
				return diagnostics, errors.New(check.reason)
			}
		case 2:
			if result.ExitCode != discardRemoteAbsentExit(cfg) {
				return diagnostics, errors.New(check.reason)
			}
		}
	}
	return diagnostics, nil
}

func (s *Service) discardRemovalEvidence(ctx context.Context, execution TaskExecution, diagnostics []CommandDiagnostics, cfg TaskRuntimeConfig) ([]CommandDiagnostics, error) {
	if _, err := os.Stat(execution.Worktree); err == nil || !os.IsNotExist(err) {
		if err != nil {
			return diagnostics, fmt.Errorf("inspect removed worktree: %w", err)
		}
		return diagnostics, fmt.Errorf("recorded worktree still exists after Worktrunk removal")
	}
	checks := [][]string{{"show-ref", "--verify", "--quiet", "refs/heads/" + execution.Branch}, discardRemoteProbe(execution.Branch, cfg)}
	for index, args := range checks {
		result, err := s.command(ctx, s.root, "git", args...)
		diagnostics = append(diagnostics, diagnostic(result))
		if err != nil {
			return diagnostics, fmt.Errorf("verify discarded refs: %w", err)
		}
		wanted := 1
		if args[0] != "show-ref" {
			wanted = discardRemoteAbsentExit(cfg)
		} else if strings.Contains(args[len(args)-1], "refs/remotes/") {
			wanted = discardRemoteAbsentExit(cfg)
		}
		if result.ExitCode != wanted {
			name := "remote branch"
			if index == 0 {
				name = "local branch"
			}
			return diagnostics, fmt.Errorf("discarded %s still exists or could not be verified", name)
		}
	}
	inventory, err := s.worktrunkList(ctx)
	diagnostics = append(diagnostics, diagnostic(inventory.Diagnostic))
	if err != nil {
		return diagnostics, fmt.Errorf("verify Worktrunk removal: %w", err)
	}
	if _, exists := inventory.Paths[execution.Branch]; exists {
		return diagnostics, fmt.Errorf("worktrunk still lists the discarded branch")
	}
	return diagnostics, nil
}

func discardRemoteProbe(branch string, cfg TaskRuntimeConfig) []string {
	if cfg.IntegrationNetworkPolicy == IntegrationNetworkPolicyNoFetch {
		return []string{"show-ref", "--verify", "--quiet", "refs/remotes/origin/" + branch}
	}
	return []string{"ls-remote", "--exit-code", "--heads", "origin", branch}
}

func discardRemoteAbsentExit(cfg TaskRuntimeConfig) int {
	if cfg.IntegrationNetworkPolicy == IntegrationNetworkPolicyNoFetch {
		return 1
	}
	return 2
}

func discardReceipt(execution TaskExecution, status, reason string, performedBy *TaskOwner, diagnostics []CommandDiagnostics, prefix string, createdAt time.Time) TaskReceipt {
	return TaskReceipt{Task: execution.Task, Operation: "discard", Status: status, Owner: execution.Owner, PerformedBy: performedBy,
		Branch: execution.Branch, Worktree: execution.Worktree, Reason: prefix + ": " + reason, CreatedAt: createdAt, Diagnostics: diagnostics,
		ToolVersion: toolVersion(), ToolRevision: toolRevision()}
}

func discardIdentityMatches(execution TaskExecution, receipt TaskReceipt) bool {
	return receipt.Task == execution.Task && receipt.Owner == execution.Owner && receipt.Branch == execution.Branch && receipt.Worktree == execution.Worktree &&
		!receipt.CreatedAt.Before(execution.StartedAt)
}
