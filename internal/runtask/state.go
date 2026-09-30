package runtask

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func (s *Service) deriveState(ctx context.Context, execution TaskExecution, receipt TaskReceipt, inventory worktrunkInventory, locked bool) DerivedTaskState {
	state := DerivedTaskState{
		Execution: execution, Status: TaskExecutionStatusActive,
		AllowedActions: []string{"run-status", "run-abort"}, Reason: "task execution is active", NextAction: "continue in the task worktree",
	}
	// A blocked record advertises run-abort because it is otherwise absorbing:
	// every other verb refuses it, and listResponse rolls one blocked record up
	// into a repository-wide failure. ADR-0006 deferred abort; ADR-0017 adds it
	// as the terminal exit.
	block := func(reason, next string) DerivedTaskState {
		state.Status, state.Reason, state.NextAction = TaskExecutionStatusBlocked, reason, next
		state.AllowedActions = []string{"run-status", "run-abort"}
		return state
	}
	if terminalReceiptStatus(receipt.Status) {
		state.Status, state.Reason, state.NextAction = receipt.Status, receipt.Reason, "no action required"
		state.AllowedActions = []string{"run-status"}
		return state
	}
	// Exit 3 means gz-git integrated the task but could not finish its own
	// cleanup. That exact receipt has a safe, separate closing action: it
	// verifies delivery again and records residuals without integrating again
	// or removing anything. Other BLOCKED receipts remain ordinary failures.
	if isPartialCleanupReceipt(execution, receipt) {
		state.Status = TaskExecutionStatusBlocked
		state.AllowedActions = []string{"run-status", "run-recover", "run-abort"}
		state.Reason = receipt.Reason
		state.NextAction = "run gz-git run recover " + execution.Task
		return state
	}
	if locked {
		return block("repository task runtime mutation is in progress or stale", "inspect the mutation lock with run-doctor")
	}
	cfg, err := s.config()
	if err != nil {
		return block("load task runtime configuration: "+err.Error(), "repair task runtime configuration")
	}
	root, err := s.worktreeRoot(cfg, execution.Owner.Host)
	if err != nil || execution.Worktree != expectedWorktreePath(root, execution.Branch) {
		return block("execution worktree is outside its configured owner path", "inspect stale or tampered execution metadata")
	}
	// Before reading the inventory, because a reclaimed record is absent from
	// it and would block here on "inventory does not match" -- the very reason
	// that never reads as the reclaim it is. reconcileReclaimed closes the
	// record only on positive evidence that the work landed; without it,
	// execution falls through to the blocks below unchanged.
	// The declined half still carries its probes: a record that stays blocked
	// below should be able to show which reconciliation checks were run and
	// what they answered, not only that they did not add up.
	reconciled, decided := s.reconcileReclaimed(ctx, execution, cfg)
	if decided {
		return reconciled
	}
	state.Diagnostics = append(state.Diagnostics, reconciled.Diagnostics...)
	actual, ok := inventory.Paths[execution.Branch]
	if !ok || actual != execution.Worktree {
		return block("Worktrunk inventory does not match execution metadata", "inspect Worktrunk and runtime metadata")
	}
	if _, err := os.Stat(execution.Worktree); err != nil {
		return block("execution metadata points to an unavailable worktree", "inspect the recorded worktree path")
	}
	probes := []struct {
		args []string
		name string
	}{
		{[]string{"branch", "--show-current"}, "branch identity"},
		{[]string{"status", "--porcelain", "--untracked-files=all"}, "clean worktree"},
		{[]string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"}, "upstream"},
		{[]string{"rev-list", "--left-right", "--count", "@{u}...HEAD"}, "upstream parity"},
	}
	for index, probe := range probes {
		result, err := s.command(ctx, execution.Worktree, "git", probe.args...)
		if err != nil {
			return block(fmt.Sprintf("derive %s: %v", probe.name, err), "inspect Git state")
		}
		state.Diagnostics = append(state.Diagnostics, diagnostic(result))
		if result.ExitCode != 0 {
			if index == 2 {
				state.Reason, state.NextAction = "task branch has no upstream", "configure and push the task branch upstream"
				return state
			}
			return block(probe.name+" is unresolved", "repair the task branch Git state")
		}
		switch index {
		case 0:
			if strings.TrimSpace(result.Stdout) != execution.Branch {
				return block("recorded branch does not match the worktree branch", "inspect stale execution metadata")
			}
		case 1:
			if strings.TrimSpace(result.Stdout) != "" {
				state.Reason, state.NextAction = "task worktree has uncommitted changes", "commit or remove task-owned changes"
				return state
			}
			// Discard remains guarded by owner, remote-ref, and exact-path checks.
			// Advertise it only once this read has proved a clean live worktree.
			state.AllowedActions = append(state.AllowedActions, "run-discard")
		case 3:
			fields := strings.Fields(result.Stdout)
			if len(fields) != 2 || fields[0] != "0" || fields[1] != "0" {
				state.Reason, state.NextAction = "task branch is ahead of or behind its upstream", "synchronize and push the task branch"
				return state
			}
		}
	}
	state.FinishReady = true
	state.AllowedActions = []string{"run-status", "run-finish", "run-abort", "run-discard"}
	state.Reason, state.NextAction = "task execution is ready to finish", "run gz-git run finish "+execution.Task
	return state
}

// terminalReceiptStatus reports whether a receipt closes its execution for
// good. DONE is the integrated outcome; ABORTED is the abandoned one. Neither
// derives further Git state, and neither is blocked, so neither reaches the
// aggregate rollup in listResponse.
func terminalReceiptStatus(status string) bool {
	return IsTerminalExecutionStatus(status)
}

func stateResponse(state DerivedTaskState) TaskResponse {
	r := response(state.Status, state.Execution.Task, state.Reason, state.NextAction, state.AllowedActions...)
	r.Execution, r.Diagnostics, r.FinishReady = &state.Execution, state.Diagnostics, state.FinishReady
	return r
}
