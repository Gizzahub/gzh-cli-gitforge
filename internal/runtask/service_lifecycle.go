// Querying and completing task execution lifecycle.

package runtask

import (
	"context"
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
	unlock, err := store.lock()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "inspect the mutation lock with run-doctor"), 1
	}
	defer unlock()
	status := s.status(ctx, task, false)
	if status.Execution == nil {
		// ISSUE-069 divergence from CE, kept deliberate: CE reuses the list
		// reader for a no-arg finish and exits 1 through the nil-execution
		// guard even when the aggregate is READY. With exactly one ACTIVE
		// execution the answer is unambiguous, so the port answers the same
		// READY document with exit 0; zero or several keep the refusal.
		if normalize(task) == "" && status.Status == TaskExecutionStatusReady && status.ActiveCount == 1 {
			return status, 0
		}
		return status, 1
	}
	if status.Status != TaskExecutionStatusActive {
		if status.Status == TaskExecutionStatusDone {
			return status, 0
		}
		return status, 1
	}
	if !status.FinishReady {
		return status, 1
	}
	execution := *status.Execution
	task = execution.Task
	owner, err := s.owner(ctx)
	if err != nil || !sameOwner(execution.Owner, owner) {
		res := response(TaskExecutionStatusBlocked, task, "only "+execution.Owner.String()+" may finish this task", "run as the recorded owner")
		appendIdentityDiagnostic(&res, err)
		return res, 1
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return response(TaskExecutionStatusBlocked, task, "resolve working directory: "+cwdErr.Error(), "run from the primary/control checkout"), 1
	}
	if within(cwd, execution.Worktree) {
		return response(TaskExecutionStatusBlocked, task, "run-finish refuses to remove its current worktree", "run from the primary/control checkout"), 1
	}
	diagnostics, preTaskHead, preSourceHead, baseHead, refused := s.finishGitEvidence(ctx, execution, task)
	if refused != nil {
		return *refused, 1
	}
	provider, noFetch, diagnostics, refused := s.finishProvider(ctx, store, execution, task, diagnostics, preSourceHead, baseHead, preTaskHead)
	if refused != nil {
		return *refused, 1
	}
	return s.finishIntegrate(ctx, store, execution, task, provider, noFetch, diagnostics, baseHead, preSourceHead, preTaskHead)
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
	sourceHeadResult, sourceHeadErr := s.command(ctx, s.root, "git", "rev-parse", execution.Source)
	if taskHeadErr != nil || sourceHeadErr != nil || taskHeadResult.ExitCode != 0 || sourceHeadResult.ExitCode != 0 {
		return diagnostics, "", "", "", refusedFinish(task, "capture pre-integration Git evidence failed", "inspect task and source branches", diagnostics)
	}
	diagnostics = append(diagnostics, diagnostic(taskHeadResult), diagnostic(sourceHeadResult))
	preTaskHead, preSourceHead := strings.TrimSpace(taskHeadResult.Stdout), strings.TrimSpace(sourceHeadResult.Stdout)
	baseHeadResult, baseHeadErr := s.command(ctx, execution.Worktree, "git", "merge-base", execution.Source, "HEAD")
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
			diagnostics = append(diagnostics, CommandDiagnostics{Command: integrationProviderGZGit, Args: []string{"integrate", "check", integrationNoFetchFlag}, Cwd: execution.Worktree, Error: reason, NotStarted: true})
			receipt := TaskReceipt{Task: execution.Task, Operation: "finish", Status: TaskExecutionStatusBlocked, Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, Reason: reason, CreatedAt: s.now().UTC(), Diagnostics: diagnostics, SourceHeadBefore: preSourceHead, BaseHead: baseHead, TaskHead: preTaskHead, ToolVersion: toolVersion(), ToolRevision: toolRevision()}
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

// finishIntegrate hands the task branch to the provider's check and run
// passes, records every outcome as a receipt, and on success validates and
// persists the DONE evidence.
func (s *Service) finishIntegrate(ctx context.Context, store stateStore, execution TaskExecution, task string, provider integrationProvider, noFetch bool, diagnostics []CommandDiagnostics, baseHead, preSourceHead, preTaskHead string) (resp TaskResponse, code int) {
	for index, call := range []func(context.Context, string, bool) (CommandResult, error){provider.Check, provider.Run} {
		result, runErr := call(ctx, execution.Worktree, noFetch)
		diagnostics = append(diagnostics, diagnostic(result))
		if runErr != nil {
			reason := "integration provider invocation failed: " + runErr.Error()
			receipt, evidenceErr := s.receipt(ctx, execution, TaskExecutionStatusBlocked, reason, diagnostics, preSourceHead, baseHead, preTaskHead, noFetch)
			if evidenceErr != nil {
				reason += "; post-operation evidence failed: " + evidenceErr.Error()
			}
			if err := store.appendReceipt(receipt); err != nil {
				reason += "; persist receipt failed: " + err.Error()
			}
			r := response(TaskExecutionStatusBlocked, task, reason, "inspect integration provider diagnostics and retry")
			r.Receipt, r.Diagnostics = &receipt, receipt.Diagnostics
			return r, 1
		}
		if result.ExitCode != 0 {
			reason, exitCode := "integration did not complete", result.ExitCode
			next := "resolve gz-git diagnostics and retry"
			if index == 1 && result.ExitCode == 3 {
				reason = "integration succeeded but task recovery cleanup failed"
			} else if exitCode < 1 || exitCode > 2 {
				exitCode = 2
			}
			// A stale base is a fact git can answer without reading gz-git's
			// prose. Name it, and do not bury it under the recovery sentence
			// that only describes the worktree still being there.
			stale := index == 0 && s.branchBaseIsStale(ctx, execution)
			if stale {
				reason = fmt.Sprintf("task branch is based on %s, but the current source tip is %s; the branch is based on a commit that is no longer the source tip", baseHead, preSourceHead)
				next = "rebase onto the source tip in the worktree, then retry run-finish"
			}
			receipt, evidenceErr := s.receipt(ctx, execution, TaskExecutionStatusBlocked, reason, diagnostics, preSourceHead, baseHead, preTaskHead, noFetch)
			if evidenceErr != nil && !stale {
				reason += "; post-operation evidence failed: " + evidenceErr.Error()
			}
			if err := store.appendReceipt(receipt); err != nil {
				reason += "; persist receipt failed: " + err.Error()
			}
			r := response(TaskExecutionStatusBlocked, task, reason, next)
			r.Receipt, r.Diagnostics = &receipt, diagnostics
			return r, exitCode
		}
	}
	return s.finishDone(ctx, store, execution, task, noFetch, diagnostics, baseHead, preSourceHead, preTaskHead)
}

// finishDone captures the DONE receipt after both provider passes succeeded:
// the evidence must verify, validate against the execution, and persist,
// or the answer is the narrow exit-3 outcome Recover exists to close.
func (s *Service) finishDone(ctx context.Context, store stateStore, execution TaskExecution, task string, noFetch bool, diagnostics []CommandDiagnostics, baseHead, preSourceHead, preTaskHead string) (resp TaskResponse, code int) {
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

// branchBaseIsStale reports whether the source tip is absent from the task
// branch. Exit 1 from merge-base --is-ancestor is that answer. Any other
// failure is not evidence of a stale base, so the caller keeps the generic
// integration message.
func (s *Service) branchBaseIsStale(ctx context.Context, execution TaskExecution) bool {
	if execution.Worktree == "" || execution.Source == "" {
		return false
	}
	result, err := s.command(ctx, execution.Worktree, "git", "merge-base", "--is-ancestor", execution.Source, "HEAD")
	if err != nil {
		return false
	}
	return result.ExitCode == 1
}

func (s *Service) receipt(ctx context.Context, execution TaskExecution, status, reason string, diagnostics []CommandDiagnostics, sourceHeadBefore, baseHead, taskHead string, noFetch bool) (TaskReceipt, error) {
	r := TaskReceipt{Task: execution.Task, Operation: "finish", Status: status, Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, Reason: reason, CreatedAt: s.now().UTC(), Diagnostics: diagnostics, SourceHeadBefore: sourceHeadBefore, BaseHead: baseHead, TaskHead: taskHead, ToolVersion: toolVersion(), ToolRevision: toolRevision()}
	probe := func(dir string, args ...string) (CommandResult, error) {
		result, err := s.command(ctx, dir, "git", args...)
		r.Diagnostics = append(r.Diagnostics, diagnostic(result))
		if err != nil {
			return result, err
		}
		return result, nil
	}
	source, err := probe(s.root, "rev-parse", execution.Source)
	if err != nil || source.ExitCode != 0 {
		return r, fmt.Errorf("verify source head")
	}
	r.SourceHead = strings.TrimSpace(source.Stdout)
	pushed, err := probe(s.root, "rev-list", "--count", "origin/"+execution.Source+".."+execution.Source)
	if err != nil || pushed.ExitCode != 0 {
		return r, fmt.Errorf("verify source push")
	}
	r.SourcePushed = strings.TrimSpace(pushed.Stdout) == "0"
	if !r.SourcePushed {
		return r, fmt.Errorf("source branch is not pushed")
	}
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
	// Source push above already reads only the local origin/<source> ref.
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
		return r, fmt.Errorf("task recovery is incomplete")
	}
	return r, nil
}
