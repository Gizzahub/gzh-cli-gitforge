// Closing a task execution that will never be integrated.

package runtask

import (
	"context"
	"os"
)

// Abort closes one execution as ABORTED by appending a terminal receipt.
//
// Without it BLOCKED is an absorbing state. A record whose worktree and branch
// were reclaimed outside gz-git -- by `gz-git integrate`, which is allowed
// to do exactly that -- derives BLOCKED forever, and listResponse rolls a
// single blocked record up into a repository-wide failure, so run list and
// run status exit 1 for as long as it exists.
//
// Evidence is appended, never rewritten. The execution stays in
// executions.json and receipts.jsonl only grows -- the same shape Finish uses
// to record a DONE outcome -- so aborting closes the derived state without
// erasing what happened. Abort touches no worktree, branch, or remote ref: a
// surviving worktree is reported as a warning for its owner to reclaim
// deliberately.
func (s *Service) Abort(ctx context.Context, task, reason string) (resp TaskResponse, code int) {
	task = normalize(task)
	if task == "" {
		return response(TaskExecutionStatusBlocked, task, "task is required", "gz-git run abort <task>"), 1
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
	// Already terminal exits 0. The caller asked for a closed record and a
	// closed record is what exists, so a recovery loop that aborts twice is
	// not a failure -- the same reading Finish gives an already-DONE task.
	previous := receipts[task]
	validDone := previous.Status == TaskExecutionStatusDone && validateDoneReceipt(task, &execution, &previous) == nil
	if previous.Status == TaskExecutionStatusAborted || validDone {
		r := response(previous.Status, task, "task execution is already "+previous.Status+": "+previous.Reason, "no action required", "run-status")
		r.Execution, r.Receipt = &execution, &previous
		return r, 0
	}
	owner, err := s.owner(ctx)
	if err != nil || !sameOwner(execution.Owner, owner) {
		res := response(TaskExecutionStatusBlocked, task, "only "+execution.Owner.String()+" may abort this task", "run as the recorded owner")
		appendIdentityDiagnostic(&res, err)
		return res, 1
	}
	receipt := TaskReceipt{
		Task: task, Operation: "abort", Status: TaskExecutionStatusAborted,
		Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree,
		Reason: abortReason(reason), CreatedAt: s.now().UTC(),
	}
	_, statErr := os.Stat(execution.Worktree)
	receipt.WorktreeRemoved = os.IsNotExist(statErr)
	if err := store.appendReceipt(receipt); err != nil {
		return response(TaskExecutionStatusBlocked, task, "persist abort receipt failed: "+err.Error(), "repair task runtime storage and retry"), 1
	}
	r := response(TaskExecutionStatusAborted, task, receipt.Reason, "no action required", "run-status")
	r.Execution, r.Receipt = &execution, &receipt
	if !receipt.WorktreeRemoved {
		r.Warnings = append(r.Warnings, "task worktree still exists at "+execution.Worktree+"; abort does not remove worktrees, branches, or remote refs")
	}
	return r, 0
}

func findExecution(executions []TaskExecution, task string) (TaskExecution, bool) {
	for i := range executions {
		if executions[i].Task == task {
			return executions[i], true
		}
	}
	return TaskExecution{}, false
}

// abortReason keeps a caller's words distinguishable from gz-git's own,
// because the receipt log is read later by someone who was not there. The
// prefix says only what gz-git knows: Abort touches no worktree, branch, or
// remote ref (see the doc comment above), so gz-git cannot say whether the
// branch landed elsewhere -- the designed abort path is exactly a branch that
// landed outside gz-git's view. A prefix that asserted the work was never
// integrated would be false on that path.
func abortReason(reason string) string {
	if reason == "" {
		return "execution closed by run-abort; gz-git did not integrate it"
	}
	return "execution closed by run-abort; gz-git did not integrate it: " + reason
}

// terminalAbortReceipt accepts a plain abort and a fully verified discard.
// The latter must prove all local cleanup and name the actor who performed it;
// a BLOCKED intent or partial Worktrunk removal never closes an execution.
func terminalAbortReceipt(receipt TaskReceipt) bool {
	if receipt.Status != TaskExecutionStatusAborted {
		return false
	}
	if receipt.Operation == "abort" {
		return true
	}
	return receipt.Operation == "discard" && receipt.PerformedBy != nil && receipt.PerformedBy.Valid() &&
		receipt.PerformedBy.Host == receipt.Owner.Host && receipt.WorktreeRemoved &&
		receipt.LocalBranchRemoved && receipt.RemoteBranchRemoved
}
