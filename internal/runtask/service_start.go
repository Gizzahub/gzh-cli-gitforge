// Starting a new task execution.

package runtask

import (
	"context"
	"fmt"
)

// Start creates the owner-bound task worktree through Worktrunk and records
// the ACTIVE execution, returning an existing live execution instead of
// starting twice and replacing a closed record in place.
func (s *Service) Start(ctx context.Context, task string, kind TaskExecutionType) TaskResponse {
	task = normalize(task)
	if task == "" || !kind.Valid() {
		return response(TaskExecutionStatusBlocked, task, "task and a valid --type are required", "use feat|fix|refactor|docs|test|chore|perf")
	}
	owner, err := s.resolveOwner(ctx)
	if err != nil {
		res := response(TaskExecutionStatusBlocked, task, err.Error(), identityNextAction(err))
		appendIdentityDiagnostic(&res, err)
		return res
	}
	cfg, err := s.config()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor")
	}
	doctor := s.Doctor(ctx)
	if doctor.Status == TaskExecutionStatusBlocked {
		doctor.Task = task
		return doctor
	}
	identityWarnings := doctor.Warnings
	worktreeRoot, err := s.worktreeRoot(cfg, owner.Host)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "declare this host in worktree-roots")
	}
	store, err := s.store(ctx)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor")
	}
	unlock, err := store.lock()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "retry after the current mutation completes")
	}
	defer unlock()
	executions, err := store.load()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "inspect task runtime metadata")
	}
	receipts, err := store.latestReceipts()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "inspect task runtime metadata")
	}
	// A closed execution does not hold its task. Finish (DONE) and Abort
	// (ABORTED) append a terminal receipt but leave the record in
	// executions.json, so without this check a task ID could be started
	// exactly once: every later run-start handed back the dead record with
	// its reclaimed worktree, and the ID was lost for follow-up work such as
	// archiving the card it landed. The closed record is replaced in place;
	// its receipts stay, so the earlier execution remains readable there.
	// Any owner may restart a closed task: a terminal execution holds no
	// worktree, branch, or lock, so there is nothing for ownership to guard,
	// and refusing would strand the ID whenever the closer's device is gone.
	replace, refused := startExistingExecution(task, owner, executions, receipts, identityWarnings)
	if refused != nil {
		return *refused
	}
	source, err := s.sourceBranch(ctx)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "declare branch.integrationBranch in .gz-git.yaml")
	}
	branch := fmt.Sprintf("dev/%s/%s/%s/%s", owner.Actor, owner.Host, kind, task)
	worktree := expectedWorktreePath(worktreeRoot, branch)
	result, err := s.runner.Run(ctx, CommandRequest{Command: "wt", Args: []string{"--config-set", worktreePathOverride(worktree), "switch", "--create", "--base", source, "--no-cd", "--format=json", branch}, WorkDir: s.root})
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor")
	}
	if result.ExitCode != 0 {
		r := response(TaskExecutionStatusBlocked, task, "Worktrunk could not create the worktree", "resolve Worktrunk diagnostics and retry")
		r.Diagnostics = []CommandDiagnostics{diagnostic(result)}
		return r
	}
	inventory, inventoryErr := s.worktrunkList(ctx)
	if inventoryErr != nil {
		r := response(TaskExecutionStatusBlocked, task, "worktree was created but its Worktrunk inventory could not be verified: "+inventoryErr.Error(), "inspect the orphaned worktree before retrying")
		r.Diagnostics = []CommandDiagnostics{diagnostic(result), diagnostic(inventory.Diagnostic)}
		return r
	}
	// Three states, not one. A bare map read cannot tell an absent branch
	// from one listed at the wrong place -- absence reads out as "", which
	// then fails the path comparison and gets reported as a worktree returned
	// somewhere wrong. Nothing was returned in that case, and the operator
	// was being sent to delete something that does not exist.
	actual, listed := inventory.Paths[branch]
	switch {
	case !listed:
		return response(TaskExecutionStatusBlocked, task,
			"Worktrunk did not list the worktree it just created for "+branch,
			"inspect "+worktree+" and the Worktrunk inventory; an item with no branch, such as a detached worktree, is skipped")
	case !within(actual, worktreeRoot):
		return response(TaskExecutionStatusBlocked, task,
			"Worktrunk returned a worktree outside gz-git's configured path: "+actual,
			"remove the unexpected worktree manually and inspect Worktrunk configuration")
	case actual != worktree:
		return response(TaskExecutionStatusBlocked, task,
			"Worktrunk created the worktree at "+actual+", not at the configured "+worktree,
			"remove the unexpected worktree manually and inspect Worktrunk configuration")
	}
	now := s.now().UTC()
	execution := TaskExecution{Task: task, Type: kind, Source: source, Owner: owner, Branch: branch, Worktree: worktree, StartedAt: now, UpdatedAt: now}
	// The record is replaced before the start receipt is appended, on
	// purpose. If the append fails, the new record sits under the old
	// terminal receipt and derives DONE/ABORTED until run-start is re-run,
	// which then takes this branch again and retries the receipt. The other
	// order would leave the old record under a fresh ACTIVE receipt, and a
	// re-run would hand that dead record back as "existing execution
	// returned" -- the state this change removes.
	if replace >= 0 {
		executions[replace] = execution
	} else {
		executions = append(executions, execution)
	}
	startReceipt := TaskReceipt{Task: task, Operation: "start", Status: TaskExecutionStatusActive, Owner: owner, Branch: branch, Worktree: worktree, Reason: "worktree created and inventory verified", CreatedAt: now, Diagnostics: []CommandDiagnostics{diagnostic(result), diagnostic(inventory.Diagnostic)}}
	if err := store.save(executions); err != nil {
		startReceipt.Status, startReceipt.Reason, startReceipt.CreatedAt = TaskExecutionStatusBlocked, "worktree was created but execution metadata persistence failed: "+err.Error(), s.now().UTC()
		if receiptErr := store.appendReceipt(startReceipt); receiptErr != nil {
			startReceipt.Reason += "; persist failure receipt failed: " + receiptErr.Error()
		}
		r := response(TaskExecutionStatusBlocked, task, startReceipt.Reason, "inspect the orphaned worktree and runtime evidence before retrying")
		r.Receipt, r.Diagnostics = &startReceipt, startReceipt.Diagnostics
		return r
	}
	if err := store.appendReceipt(startReceipt); err != nil {
		r := response(TaskExecutionStatusBlocked, task, "execution metadata was saved but start receipt persistence failed: "+err.Error(), "retry run-start to recover the existing execution")
		r.Execution, r.Receipt, r.Diagnostics = &execution, &startReceipt, startReceipt.Diagnostics
		return r
	}
	r := response(TaskExecutionStatusActive, task, "task execution started", "work in the task worktree; use run-status to evaluate finish readiness", "run-status")
	r.Execution, r.Diagnostics = &execution, []CommandDiagnostics{diagnostic(result), diagnostic(inventory.Diagnostic)}
	r.Warnings = identityWarnings
	return r
}

// startExistingExecution answers how a recorded execution for task meets a
// new start: the index to replace when the record is closed, or a non-nil
// response when the record refuses the start or returns the live execution.
// No record holding the task yields -1 with a nil response.
func startExistingExecution(task string, owner TaskOwner, executions []TaskExecution, receipts map[string]TaskReceipt, identityWarnings []string) (int, *TaskResponse) {
	replace := -1
	for i := range executions {
		if executions[i].Task != task {
			continue
		}
		previous := receipts[task]
		validDone := previous.Status == TaskExecutionStatusDone && validateDoneReceipt(task, &executions[i], &previous) == nil
		if previous.Status == TaskExecutionStatusAborted || validDone {
			replace = i
			break
		}
		if previous.Status == TaskExecutionStatusDone {
			r := response(TaskExecutionStatusBlocked, task, "finish receipt does not prove this execution completed", "inspect task runtime receipt evidence", "run-status", "run-abort")
			r.Execution, r.Receipt = &executions[i], &previous
			return -1, &r
		}
		if sameOwner(executions[i].Owner, owner) {
			r := response(TaskExecutionStatusActive, task, "existing execution returned", "run gz-git run status "+task+" to evaluate finish readiness", "run-status")
			r.Execution = &executions[i]
			r.Warnings = identityWarnings
			return -1, &r
		}
		r := response(TaskExecutionStatusBlocked, task, "task is owned by "+executions[i].Owner.String(), "ask the owner to finish the task")
		return -1, &r
	}
	return replace, nil
}
