// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Querying and completing task execution lifecycle.

package runtask

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Status derives one task's runtime state without acquiring the mutation
// lock; the empty task name asks for the repository-wide aggregate.
func (s *Service) Status(ctx context.Context, task string) TaskResponse {
	return s.status(ctx, task, true)
}

func (s *Service) status(ctx context.Context, task string, observeLock bool) TaskResponse {
	store, err := s.store(ctx)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor")
	}
	executions, err := store.load()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "inspect task runtime metadata")
	}
	receipts, err := store.latestReceipts()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, "read task receipts: "+err.Error(), "inspect task runtime receipt evidence")
	}
	inventory, err := s.worktrunkList(ctx)
	if err != nil {
		r := response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor")
		r.Diagnostics = []CommandDiagnostics{diagnostic(inventory.Diagnostic)}
		return r
	}
	locked := false
	if observeLock {
		_, lockErr := os.Stat(store.lockPath())
		locked = lockErr == nil
	}
	task = normalize(task)
	if task == "" {
		return s.listResponse(ctx, executions, receipts, inventory, locked)
	}
	for i := range executions {
		if executions[i].Task == task {
			receipt := receipts[task]
			result := stateResponse(s.verifiedExecutionState(ctx, executions[i], receipt, inventory, locked))
			if receipt.Task != "" {
				result.Receipt = &receipt
			}
			return result
		}
	}
	return response(TaskExecutionStatusUnknown, task, "task execution was not found", "gz-git run list", "run-list")
}

// List answers the repository-shared execution inventory without acquiring
// the mutation lock: every live execution keeps its own derived state, and
// terminal ones fold into the aggregate counts.
func (s *Service) List(ctx context.Context) TaskResponse {
	store, err := s.store(ctx)
	if err != nil {
		return response(TaskExecutionStatusBlocked, "", err.Error(), "run gz-git run doctor")
	}
	executions, err := store.load()
	if err != nil {
		return response(TaskExecutionStatusBlocked, "", err.Error(), "inspect task runtime metadata")
	}
	receipts, err := store.latestReceipts()
	if err != nil {
		return response(TaskExecutionStatusBlocked, "", "read task receipts: "+err.Error(), "inspect task runtime receipt evidence")
	}
	inventory, err := s.worktrunkList(ctx)
	if err != nil {
		r := response(TaskExecutionStatusBlocked, "", err.Error(), "run gz-git run doctor")
		r.Diagnostics = []CommandDiagnostics{diagnostic(inventory.Diagnostic)}
		return r
	}
	_, lockErr := os.Stat(store.lockPath())
	return s.listResponse(ctx, executions, receipts, inventory, lockErr == nil)
}

func (s *Service) listResponse(ctx context.Context, executions []TaskExecution, receipts map[string]TaskReceipt, inventory worktrunkInventory, locked bool) TaskResponse {
	sort.Slice(executions, func(i, j int) bool { return executions[i].Task < executions[j].Task })
	// READY means the list was answered, not that a run is alive. A live run
	// stays ACTIVE on its state; activeCount is how many states say that.
	r := response(TaskExecutionStatusReady, "", "task execution states returned", "select a task", "run-list", "run-status")
	r.Executions = executions
	for _, execution := range executions {
		state := s.verifiedExecutionState(ctx, execution, receipts[execution.Task], inventory, locked)
		r.States = append(r.States, state)
		if state.Status == TaskExecutionStatusActive {
			r.ActiveCount++
		}
		// One blocked record fails the whole aggregate, which is why a record
		// that can never be finished must have a terminal exit: `run-abort`
		// (Abort, service_abort.go) appends an ABORTED receipt, and the state
		// derived here stops being blocked. ADR-0006 deferred that verb;
		// ADR-0017 added it.
		if state.Status == TaskExecutionStatusBlocked {
			r.Status, r.Reason = TaskExecutionStatusBlocked, "one or more task executions are blocked"
		}
	}
	return r
}

func (s *Service) verifiedExecutionState(ctx context.Context, execution TaskExecution, receipt TaskReceipt, inventory worktrunkInventory, locked bool) DerivedTaskState {
	if err := validateExecutionPurpose(execution); err != nil {
		return DerivedTaskState{
			Execution: execution, Status: TaskExecutionStatusBlocked, AllowedActions: []string{"run-status", "run-abort"},
			Reason: err.Error(), NextAction: "inspect task runtime metadata",
		}
	}
	if !receiptPurposeMatchesOrAbsent(execution, receipt) {
		return DerivedTaskState{
			Execution: execution, Status: TaskExecutionStatusBlocked, AllowedActions: []string{"run-status", "run-abort"},
			Reason: "receipt purpose is invalid or does not match the execution", NextAction: "inspect task runtime receipt evidence",
		}
	}
	if receipt.Status == TaskExecutionStatusDone {
		if err := validateDoneReceipt(execution.Task, &execution, &receipt); err != nil {
			return DerivedTaskState{
				Execution: execution, Status: TaskExecutionStatusBlocked, AllowedActions: []string{"run-status", "run-abort"},
				Reason: "finish receipt does not prove this execution completed: " + err.Error(), NextAction: "inspect task runtime receipt evidence",
			}
		}
	}
	return s.deriveState(ctx, execution, receipt, inventory, locked)
}

// Finish integrates a clean, fully pushed task branch into its configured
// source branch and reclaims the worktree, local branch, and remote branch
// in the same step, answering DONE only from receipt evidence that proves
// all three.
func (s *Service) Finish(ctx context.Context, task string) (resp TaskResponse, code int) {
	store, err := s.store(ctx)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "run gz-git run doctor"), 1
	}
	plan, refused, code := s.prepareFinish(ctx, store, task)
	if plan == nil {
		return refused, code
	}
	// The claim is on disk and the lock is released: the provider may wait on
	// the host integration slot for as long as it likes without stopping other
	// tasks' mutations here. The claim keeps every verb off this execution.
	//
	// From here until the outcome is appended, SIGINT and SIGTERM only stop
	// the relock wait: a finish they ended would leave the claim with no
	// outcome. The wait is bounded by finishRelockWait; a failed relock leaves
	// the provider's actual result unrecorded and never runs the provider
	// again.
	interrupted, stopInterrupts := finishInterruptContext(ctx)
	defer stopInterrupts()
	result, runErr := plan.provider.Run(ctx, plan.execution.Worktree, integrationTarget(plan.execution), plan.noFetch)
	unlock, lockErr := store.lockWait(interrupted, finishRelockWait, finishRelockPoll)
	if lockErr != nil {
		reason := "integration provider exited but the mutation lock could not be reacquired: " + lockErr.Error()
		if interrupted.Err() != nil && ctx.Err() == nil {
			reason += " (a SIGINT or SIGTERM received during or after the run ended the wait)"
		}
		return unrecordedFinishOutcome(plan.execution, plan.execution.Task, plan.diagnostics, result, runErr, reason)
	}
	defer unlock()
	if change := finishClaimChange(store, plan.execution, plan.claim); change != "" {
		return unrecordedFinishOutcome(plan.execution, plan.execution.Task, plan.diagnostics, result, runErr, "integration provider exited but task runtime state changed while the mutation lock was released: "+change)
	}
	return s.recordFinish(ctx, store, plan.execution, plan.execution.Task, plan.noFetch, plan.diagnostics, result, runErr, plan.preSourceHead, plan.baseHead, plan.preTaskHead)
}

// finishPlan carries what the locked half of run-finish proved to the
// unlocked provider run and the relocked recording half.
type finishPlan struct {
	execution                            TaskExecution
	provider                             integrationProvider
	noFetch                              bool
	diagnostics                          []CommandDiagnostics
	preSourceHead, baseHead, preTaskHead string
	claim                                TaskReceipt
}

// prepareFinish runs every precondition under the mutation lock and, when all
// hold, appends the FINISHING claim before the lock is released. The lock and
// its defer live only here, so the provider run and the relock in Finish can
// never inherit this function's unlock closure: a relock failure removes the
// lock this function took, and nothing else. A nil plan means the returned
// response is the answer.
func (s *Service) prepareFinish(ctx context.Context, store stateStore, task string) (*finishPlan, TaskResponse, int) {
	unlock, err := store.lock()
	if err != nil {
		return nil, response(TaskExecutionStatusBlocked, task, err.Error(), "inspect the mutation lock with run-doctor"), 1
	}
	defer unlock()
	status := s.status(ctx, task, false)
	if status.Execution == nil && normalize(task) == "" {
		if status.Status == TaskExecutionStatusBlocked && len(status.States) == 0 {
			return nil, status, 1
		}
		owner, ownerErr := s.owner(ctx)
		if ownerErr != nil {
			res := response(TaskExecutionStatusBlocked, "", "cannot resolve the current owner for bare run-finish: "+ownerErr.Error(), "set the actor and host, then retry")
			appendIdentityDiagnostic(&res, ownerErr)
			return nil, res, 1
		}
		_, refusal := selectOwnedActiveTask(status.States, owner)
		if refusal != nil {
			return nil, *refusal, 1
		}
		// Bare finish is a readiness query, never an implicit integration.
		// Keep owner/purpose and ambiguity checks, then return the aggregate
		// without creating a receipt or changing the execution's resources.
		if status.Status == TaskExecutionStatusReady && status.ActiveCount == 1 {
			return nil, status, 0
		}
		return nil, status, 1
	}
	if status.Execution == nil {
		return nil, status, 1
	}
	if status.Status != TaskExecutionStatusActive {
		if status.Status == TaskExecutionStatusDone {
			return nil, status, 0
		}
		return nil, status, 1
	}
	if purpose := status.Execution.Purpose; purpose != "" && purpose != "delivery" {
		return nil, response(TaskExecutionStatusBlocked, status.Execution.Task,
			"run-finish is only available for delivery executions; "+purpose+" must end with run-abort or run-discard",
			"record a preservation reason with gz-git run abort "+status.Execution.Task+" --reason R, or reclaim the clean worktree with run discard"), 1
	}
	if !status.FinishReady {
		return nil, status, 1
	}
	execution := *status.Execution
	task = execution.Task
	owner, err := s.owner(ctx)
	if err != nil || !sameOwner(execution.Owner, owner) {
		res := response(TaskExecutionStatusBlocked, task, "only "+execution.Owner.String()+" may finish this task", "run as the recorded owner")
		appendIdentityDiagnostic(&res, err)
		return nil, res, 1
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return nil, response(TaskExecutionStatusBlocked, task, "resolve working directory: "+cwdErr.Error(), "run from the primary/control checkout"), 1
	}
	if within(cwd, execution.Worktree) {
		return nil, response(TaskExecutionStatusBlocked, task, "run-finish refuses to remove its current worktree", "run from the primary/control checkout"), 1
	}
	diagnostics, preTaskHead, preSourceHead, baseHead, refused := s.finishGitEvidence(ctx, execution, task)
	if refused != nil {
		return nil, *refused, 1
	}
	provider, noFetch, diagnostics, refused := s.finishProvider(ctx, store, execution, task, diagnostics, preSourceHead, baseHead, preTaskHead)
	if refused != nil {
		return nil, *refused, 1
	}
	// The integration gate can take tens of minutes. A base that is already
	// stale would only be refused after it, so refuse before spending it.
	if s.branchBaseIsStale(ctx, execution) {
		r, code := s.refuseStaleFinish(ctx, store, execution, diagnostics, preSourceHead, baseHead, preTaskHead, noFetch)
		return nil, r, code
	}
	plan := &finishPlan{execution: execution, provider: provider, noFetch: noFetch, diagnostics: diagnostics, preSourceHead: preSourceHead, baseHead: baseHead, preTaskHead: preTaskHead}
	claim, claimErr := s.appendFinishClaim(store, execution, preSourceHead, baseHead, preTaskHead)
	if claimErr != nil {
		return nil, response(TaskExecutionStatusBlocked, task, "persist finish claim failed: "+claimErr.Error(), "repair task runtime storage and retry"), 1
	}
	plan.claim = claim
	return plan, TaskResponse{}, 0
}

// refusedFinish marks a helper outcome that already carries the response to
// answer with; callers return it with exit 1.
func refusedFinish(task, reason, next string, diagnostics []CommandDiagnostics) *TaskResponse {
	r := response(TaskExecutionStatusBlocked, task, reason, next)
	r.Diagnostics = diagnostics
	return &r
}

// finishGitEvidence runs the pre-integration Git checks and captures the
// heads a receipt must name. A non-nil response is the refusal to return.
func (s *Service) finishGitEvidence(ctx context.Context, execution TaskExecution, task string) (diagnostics []CommandDiagnostics, taskHead, sourceHead, baseHead string, refused *TaskResponse) {
	checks := []struct {
		args   []string
		reason string
	}{{[]string{"status", "--porcelain"}, "task worktree is dirty"}, {[]string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"}, "task branch has no upstream"}, {[]string{"rev-list", "--left-right", "--count", "@{u}...HEAD"}, "task branch must be fully pushed"}}
	diagnostics = []CommandDiagnostics{}
	for i, check := range checks {
		result, runErr := s.command(ctx, execution.Worktree, "git", check.args...)
		if runErr != nil {
			return diagnostics, "", "", "", refusedFinish(task, runErr.Error(), "inspect the task worktree", nil)
		}
		diagnostics = append(diagnostics, diagnostic(result))
		bad := result.ExitCode != 0
		if i == 0 {
			bad = bad || strings.TrimSpace(result.Stdout) != ""
		}
		if i == 2 {
			fields := strings.Fields(result.Stdout)
			bad = bad || len(fields) != 2 || fields[0] != "0" || fields[1] != "0"
		}
		if bad {
			return diagnostics, "", "", "", refusedFinish(task, check.reason, "resolve the Git precondition and retry", diagnostics)
		}
	}
	taskHeadResult, taskHeadErr := s.command(ctx, execution.Worktree, "git", "rev-parse", "HEAD")
	sourceHeadResult, sourceHeadErr := s.command(ctx, s.root, "git", "rev-parse", "--verify", sourceTrackingRef(execution.Source))
	if taskHeadErr != nil || sourceHeadErr != nil || taskHeadResult.ExitCode != 0 || sourceHeadResult.ExitCode != 0 {
		return diagnostics, "", "", "", refusedFinish(task, "capture pre-integration Git evidence failed", "inspect task and source branches", diagnostics)
	}
	diagnostics = append(diagnostics, diagnostic(taskHeadResult), diagnostic(sourceHeadResult))
	preTaskHead, preSourceHead := strings.TrimSpace(taskHeadResult.Stdout), strings.TrimSpace(sourceHeadResult.Stdout)
	baseHeadResult, baseHeadErr := s.command(ctx, execution.Worktree, "git", "merge-base", sourceTrackingRef(execution.Source), "HEAD")
	diagnostics = append(diagnostics, diagnostic(baseHeadResult))
	baseHead = strings.TrimSpace(baseHeadResult.Stdout)
	if baseHeadErr != nil || baseHeadResult.ExitCode != 0 {
		return diagnostics, "", "", "", refusedFinish(task, "capture task branch base failed", "inspect task and source branches", diagnostics)
	}
	return diagnostics, preTaskHead, preSourceHead, baseHead, nil
}

// finishProvider resolves the integration provider and, under no-fetch,
// proves the capability before anything mutates, recording the refusal
// receipt when the declaration is missing. A non-nil response is the
// refusal to return.
func (s *Service) finishProvider(ctx context.Context, store stateStore, execution TaskExecution, task string, diagnostics []CommandDiagnostics, preSourceHead, baseHead, preTaskHead string) (integrationProvider, bool, []CommandDiagnostics, *TaskResponse) {
	cfg, configErr := s.config()
	if configErr != nil {
		return nil, false, diagnostics, refusedFinish(task, configErr.Error(), "configure "+TaskRuntimeConfigFile, nil)
	}
	provider, providerErr := integrationProviderFor(cfg.IntegrationProvider, s.engine)
	if providerErr != nil {
		return nil, false, diagnostics, refusedFinish(task, providerErr.Error(), "configure a supported integration-provider", nil)
	}
	noFetch := cfg.IntegrationNetworkPolicy == IntegrationNetworkPolicyNoFetch
	if noFetch {
		// The engine is this binary, so the capability is a build fact rather
		// than something a help probe could discover; the call below keeps the
		// provider contract honest without running any operation.
		_, probeDiagnostics, probeErr := provider.DiagnoseNoFetch(ctx, execution.Worktree)
		diagnostics = append(diagnostics, probeDiagnostics...)
		if probeErr != nil {
			reason := "integration provider gz-git has no declared no-fetch finish capability: " + probeErr.Error()
			diagnostics = append(diagnostics, CommandDiagnostics{Command: integrationProviderGZGit, Args: integrateArgs(integrationTarget(execution), true), Cwd: execution.Worktree, Error: reason, NotStarted: true})
			receipt := TaskReceipt{Task: execution.Task, Purpose: string(executionPurpose(execution)), Operation: "finish", Status: TaskExecutionStatusBlocked, Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, Reason: reason, CreatedAt: s.now().UTC(), Diagnostics: diagnostics, SourceHeadBefore: preSourceHead, BaseHead: baseHead, TaskHead: preTaskHead, ToolVersion: toolVersion(), ToolRevision: toolRevision()}
			if err := store.appendReceipt(receipt); err != nil {
				reason += "; persist receipt failed: " + err.Error()
			}
			r := response(TaskExecutionStatusBlocked, task, reason, "use a provider with a declared no-fetch finish capability or change integration-network-policy")
			r.Receipt, r.Diagnostics = &receipt, receipt.Diagnostics
			return nil, false, diagnostics, &r
		}
	}
	return provider, noFetch, diagnostics, nil
}

// finishDone captures the DONE receipt after the provider run succeeded:
// the evidence must verify, validate against the execution, and persist,
// or the answer is the narrow exit-3 outcome Recover exists to close.
func (s *Service) finishDone(ctx context.Context, store stateStore, execution TaskExecution, task string, noFetch bool, diagnostics []CommandDiagnostics, preSourceHead, baseHead, preTaskHead string) (resp TaskResponse, code int) {
	receipt, evidenceErr := s.receipt(ctx, execution, TaskExecutionStatusDone, "integration and recovery completed", diagnostics, preSourceHead, baseHead, preTaskHead, noFetch)
	if evidenceErr != nil {
		r := response(TaskExecutionStatusBlocked, task, "integration completed but evidence verification failed: "+evidenceErr.Error(), "inspect source push and recovery state")
		r.Receipt, r.Diagnostics = &receipt, receipt.Diagnostics
		return r, 3
	}
	if err := validateIntegratedFinish(task, &execution, &receipt); err != nil {
		r := response(TaskExecutionStatusBlocked, task, "integration completed but finish receipt validation failed: "+err.Error(), "inspect task runtime receipt evidence")
		r.Receipt, r.Diagnostics = &receipt, receipt.Diagnostics
		return r, 3
	}
	if err := store.appendReceipt(receipt); err != nil {
		r := response(TaskExecutionStatusBlocked, task, "integration completed but receipt persistence failed: "+err.Error(), "preserve diagnostics and repair runtime storage")
		r.Receipt, r.Diagnostics = &receipt, receipt.Diagnostics
		return r, 3
	}
	r := response(TaskExecutionStatusDone, task, receipt.Reason, "no action required")
	r.Execution, r.Receipt, r.Diagnostics = &execution, &receipt, diagnostics
	return r, 0
}

func validateIntegratedFinish(task string, execution *TaskExecution, receipt *TaskReceipt) error {
	if normalize(task) == "" {
		return fmt.Errorf("requested task is empty")
	}
	if execution == nil {
		return fmt.Errorf("execution is missing")
	}
	if execution.Task == "" {
		return fmt.Errorf("execution task is empty")
	}
	if execution.Task != task {
		return fmt.Errorf("execution task %q does not match requested task %q", execution.Task, task)
	}
	if receipt == nil {
		return fmt.Errorf("receipt is missing")
	}
	if receipt.Task == "" {
		return fmt.Errorf("receipt task is empty")
	}
	if receipt.Task != execution.Task {
		return fmt.Errorf("receipt task %q does not match execution task %q", receipt.Task, execution.Task)
	}
	if receipt.Owner != execution.Owner || receipt.Branch != execution.Branch || receipt.Worktree != execution.Worktree {
		return fmt.Errorf("receipt identity does not match execution identity")
	}
	if receipt.Operation != "finish" || receipt.Status != TaskExecutionStatusDone {
		return fmt.Errorf("receipt is not a successful terminal finish")
	}
	if strings.TrimSpace(receipt.SourceHeadBefore) == "" || strings.TrimSpace(receipt.TaskHead) == "" || strings.TrimSpace(receipt.SourceHead) == "" {
		return fmt.Errorf("receipt Git head evidence is incomplete")
	}
	if !receipt.SourcePushed || !receipt.WorktreeRemoved || !receipt.LocalBranchRemoved || !receipt.RemoteBranchRemoved {
		return fmt.Errorf("receipt integration or recovery evidence is incomplete")
	}
	return nil
}

// integrationTarget is the remote-tracking ref of the source recorded at
// run-start, the same ref finish compares, pins the provider to, and pushes.
func integrationTarget(execution TaskExecution) string {
	return "origin/" + execution.Source
}

// sourceTrackingRef is integrationTarget spelled as a full ref, for the Git
// evidence the lifecycle reads itself. The provider integrates into
// origin/<source> and moves a local <source> only where it is checked out, so
// a local branch sitting in no worktree is absent or behind and cannot stand
// for the tip. The full spelling keeps a local branch literally named
// origin/<source> from shadowing it.
func sourceTrackingRef(source string) string {
	return "refs/remotes/origin/" + source
}

// sourceTip resolves origin's tip of source to a commit in the primary
// checkout. It reads the local remote-tracking ref only; no fetch is made.
func (s *Service) sourceTip(ctx context.Context, source string) (string, error) {
	ref := sourceTrackingRef(source) + "^{commit}"
	result, err := s.command(ctx, s.root, "git", "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return "", fmt.Errorf("resolve source tip %s: %w", sourceTrackingRef(source), err)
	}
	tip := strings.TrimSpace(result.Stdout)
	if result.ExitCode != 0 || tip == "" {
		return "", fmt.Errorf("source tip %s does not resolve in the primary checkout", sourceTrackingRef(source))
	}
	return tip, nil
}

// branchBaseIsStale reports whether the source tip is absent from the task
// branch. Exit 1 from merge-base --is-ancestor is that answer. Any other
// failure is not evidence of a stale base, so the caller keeps the generic
// integration message.
func (s *Service) branchBaseIsStale(ctx context.Context, execution TaskExecution) bool {
	if execution.Worktree == "" || execution.Source == "" {
		return false
	}
	result, err := s.command(ctx, execution.Worktree, "git", "merge-base", "--is-ancestor", sourceTrackingRef(execution.Source), "HEAD")
	if err != nil {
		return false
	}
	return result.ExitCode == 1
}

// errRecoveryIncomplete is the expected consequence of a refused finish: the
// worktree and branches are still there. Other evidence errors are independent
// faults and must stay visible.
var errRecoveryIncomplete = errors.New("task recovery is incomplete")

func (s *Service) receipt(ctx context.Context, execution TaskExecution, status, reason string, diagnostics []CommandDiagnostics, sourceHeadBefore, baseHead, taskHead string, noFetch bool) (TaskReceipt, error) {
	r := TaskReceipt{Task: execution.Task, Purpose: string(executionPurpose(execution)), Operation: "finish", Status: status, Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, Reason: reason, CreatedAt: s.now().UTC(), Diagnostics: diagnostics, SourceHeadBefore: sourceHeadBefore, BaseHead: baseHead, TaskHead: taskHead, ToolVersion: toolVersion(), ToolRevision: toolRevision()}
	probe := func(dir string, args ...string) (CommandResult, error) {
		probeCtx := ctx
		if len(args) > 0 && args[0] != "ls-remote" {
			probeCtx = offlineObservationContext(ctx)
		}
		result, err := s.command(probeCtx, dir, "git", args...)
		r.Diagnostics = append(r.Diagnostics, diagnostic(result))
		if err != nil {
			return result, err
		}
		return result, nil
	}
	policy := IntegrationNetworkPolicyAllow
	if noFetch {
		policy = IntegrationNetworkPolicyNoFetch
	}
	evidence, err := verifySourcePush(probe, s.root, execution.Source, policy)
	if err != nil {
		return r, err
	}
	r.SourceHead = evidence.head
	if err := verifySourceContainsTask(probe, s.root, taskHead, evidence.head); err != nil {
		return r, err
	}
	r.SourcePushed = true
	_, err = os.Stat(execution.Worktree)
	r.WorktreeRemoved = os.IsNotExist(err)
	if err != nil && !os.IsNotExist(err) {
		return r, fmt.Errorf("verify worktree removal: %w", err)
	}
	local, err := probe(s.root, "show-ref", "--verify", "--quiet", "refs/heads/"+execution.Branch)
	if err != nil {
		return r, fmt.Errorf("verify local branch removal: %w", err)
	}
	if local.ExitCode != 0 && local.ExitCode != 1 {
		return r, fmt.Errorf("verify local branch removal: exit %d", local.ExitCode)
	}
	r.LocalBranchRemoved = local.ExitCode == 1
	// Under no-fetch gz-git must not read origin either. The provider deletes
	// the remote branch with a push to the named remote, which also drops the
	// local remote-tracking ref, so that ref is the admissible evidence; a
	// stale one, or one left by a push to a raw URL, fails closed.
	// Under no-fetch, source push above reads only local refs: the
	// refs/remotes/origin/<source> tracking ref, and refs/heads/<source> when
	// that branch exists.
	remoteArgs, removedExit := []string{"ls-remote", "--exit-code", "--heads", "origin", execution.Branch}, 2
	if noFetch {
		remoteArgs, removedExit = []string{"show-ref", "--verify", "--quiet", "refs/remotes/origin/" + execution.Branch}, 1
	}
	remote, err := probe(s.root, remoteArgs...)
	if err != nil {
		return r, fmt.Errorf("verify remote branch removal: %w", err)
	}
	if remote.ExitCode != 0 && remote.ExitCode != removedExit {
		return r, fmt.Errorf("verify remote branch removal: exit %d", remote.ExitCode)
	}
	r.RemoteBranchRemoved = remote.ExitCode == removedExit
	if !r.WorktreeRemoved || !r.LocalBranchRemoved || !r.RemoteBranchRemoved {
		return r, errRecoveryIncomplete
	}
	return r, nil
}
