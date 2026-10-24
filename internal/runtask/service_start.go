// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Starting a new task execution.

package runtask

import (
	"context"
	"fmt"
	"time"
)

// StartWithPurpose creates the owner-bound task worktree through Worktrunk
// and records the ACTIVE execution. An unfinished execution for the same task
// is never handed back implicitly: owner identity is only actor/host/kind, so
// two concurrent sessions of one agent are indistinguishable and the second
// would silently adopt the first one's worktree. resume is the explicit
// statement that the caller owns the existing execution and wants it
// returned; it cannot change the recorded type or purpose. A missing purpose
// is accepted only as legacy delivery; an explicit invalid value is rejected
// before any runtime mutation.
//
//nolint:gocyclo // guard ladder with one BLOCKED answer per branch; extraction would scatter the pinned preflight order
func (s *Service) StartWithPurpose(ctx context.Context, task string, kind TaskExecutionType, purpose TaskExecutionPurpose, resume bool) TaskResponse {
	task = normalize(task)
	if task == "" || !kind.Valid() || (purpose != "" && !purpose.Valid()) {
		return response(TaskExecutionStatusBlocked, task, "task, a valid --type, and purpose delivery|experiment|review are required", "use feat|fix|refactor|docs|test|chore|perf with --purpose delivery|experiment|review")
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
	replace, resumed, refused := startExistingExecution(task, kind, purpose, resume, owner, executions, receipts)
	if resumed != nil {
		resumed.Warnings = identityWarnings
		return *resumed
	}
	if refused != nil {
		return *refused
	}
	if purpose == "" {
		purpose = TaskExecutionPurposeDelivery
	}
	source, err := s.sourceBranch(ctx)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "declare branch.integrationBranch in .gz-git.yaml")
	}
	// The base is origin's tip of the source, resolved to a commit. A local
	// source branch that is not checked out anywhere is never advanced by
	// finish, which integrates into origin/<source>, so basing on it started
	// every later run behind the tip and sent it straight into the freshness
	// gate. A commit, not the remote-tracking name, so no Git or Worktrunk
	// configuration can make origin/<source> the task branch's upstream.
	base, err := s.sourceTip(ctx, source)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "fetch origin "+source+" in the primary checkout and retry")
	}
	branch := fmt.Sprintf("%s/%s/%s/%s/%s", purpose.BranchPrefix(), owner.Actor, owner.Host, kind, task)
	worktree := expectedWorktreePath(worktreeRoot, branch)
	result, err := s.runner.Run(ctx, CommandRequest{Command: "wt", Args: []string{"--config-set", worktreePathOverride(worktree), "switch", "--create", "--base", base, "--no-cd", "--format=json", branch}, WorkDir: s.root})
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
	execution := TaskExecution{Task: task, Type: kind, Purpose: string(purpose), Source: source, Owner: owner, Branch: branch, Worktree: worktree, StartedAt: now, UpdatedAt: now}
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
	startReceipt := TaskReceipt{Task: task, Purpose: string(purpose), Operation: "start", Status: TaskExecutionStatusActive, Owner: owner, Branch: branch, Worktree: worktree, Reason: "worktree created and inventory verified", CreatedAt: now, Diagnostics: []CommandDiagnostics{diagnostic(result), diagnostic(inventory.Diagnostic)}}
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
	r := response(TaskExecutionStatusActive, task, "task execution started", "work in the task worktree; use run-status to evaluate finish readiness; run-finish runs from the primary checkout, after the repository lint passes and the branch is current with the source tip", "run-status")
	r.Execution, r.Diagnostics = &execution, []CommandDiagnostics{diagnostic(result), diagnostic(inventory.Diagnostic)}
	r.Warnings = identityWarnings
	return r
}

// startExistingExecution answers how a recorded execution for task meets a
// new start: the index to replace when the record is closed, a non-nil
// resumed response when --resume returns the caller's own matching
// execution, or a non-nil refused response when the record refuses the
// start. No record holding the task yields -1 with nil responses. An
// unfinished execution is never handed back implicitly: owner identity is
// only actor/host/kind, so two concurrent sessions of one agent are
// indistinguishable and the second would silently adopt the first one's
// worktree.
func startExistingExecution(task string, kind TaskExecutionType, purpose TaskExecutionPurpose, resume bool, owner TaskOwner, executions []TaskExecution, receipts map[string]TaskReceipt) (replace int, resumed, refused *TaskResponse) {
	replace = -1
	for i := range executions {
		if executions[i].Task != task {
			continue
		}
		if err := validateExecutionPurpose(executions[i]); err != nil {
			r := response(TaskExecutionStatusBlocked, task, err.Error(), "inspect execution metadata; gz-git will not resume a purpose/branch mismatch")
			r.Execution = &executions[i]
			return -1, nil, &r
		}
		previous := receipts[task]
		if !receiptPurposeValid(previous) || (previous.Task != "" && !purposeMatchesReceipt(executions[i], previous)) {
			r := response(TaskExecutionStatusBlocked, task, "receipt purpose does not match the execution", "inspect task runtime receipt evidence")
			r.Execution, r.Receipt = &executions[i], &previous
			return -1, nil, &r
		}
		validDone := previous.Status == TaskExecutionStatusDone && validateDoneReceipt(task, &executions[i], &previous) == nil
		if previous.Status == TaskExecutionStatusAborted || validDone {
			replace = i
			break
		}
		if previous.Status == TaskExecutionStatusDone {
			r := response(TaskExecutionStatusBlocked, task, "finish receipt does not prove this execution completed", "inspect task runtime receipt evidence", "run-status", "run-abort")
			r.Execution, r.Receipt = &executions[i], &previous
			return -1, nil, &r
		}
		if !sameOwner(executions[i].Owner, owner) {
			r := response(TaskExecutionStatusBlocked, task, "task is owned by "+executions[i].Owner.String(), "ask the owner to finish the task")
			return -1, nil, &r
		}
		if !resume {
			existing := executions[i]
			reason := fmt.Sprintf("task already has an unfinished execution (branch %s, worktree %s, started %s, owner %s); it may belong to another session of the same owner",
				existing.Branch, existing.Worktree, existing.StartedAt.UTC().Format(time.RFC3339), existing.Owner.String())
			r := response(TaskExecutionStatusBlocked, task, reason,
				"if this execution is yours, run gz-git run start "+task+" --type "+string(kind)+" --purpose "+string(executionPurpose(existing))+" --resume; otherwise inspect it with run-status or end it with run-abort",
				"run-status", "run-abort")
			r.Execution = &executions[i]
			return -1, nil, &r
		}
		if executions[i].Type != kind || (purpose != "" && executionPurpose(executions[i]) != purpose) {
			r := response(TaskExecutionStatusBlocked, task, "--resume cannot change the recorded execution type or purpose", "resume with --type "+string(executions[i].Type)+" --purpose "+string(executionPurpose(executions[i])))
			r.Execution = &executions[i]
			return -1, nil, &r
		}
		r := response(TaskExecutionStatusActive, task, "existing execution returned", "run gz-git run status "+task+" to evaluate finish readiness", "run-status")
		r.Execution = &executions[i]
		return -1, &r, nil
	}
	return replace, nil, nil
}
