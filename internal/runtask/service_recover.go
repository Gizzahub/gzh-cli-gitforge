// Recovering an integration that landed before provider cleanup failed.

package runtask

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const partialCleanupReason = "integration succeeded but task recovery cleanup failed"

// Recover closes the narrow exit-3 outcome from run finish. It never invokes
// the integration engine and never removes a worktree or a ref: it only
// verifies that the recorded task commit is in the pushed source branch, then
// appends evidence naming whatever engine cleanup left behind.
func (s *Service) Recover(ctx context.Context, task string) (resp TaskResponse, code int) {
	task = normalize(task)
	if task == "" {
		return response(TaskExecutionStatusBlocked, task, "task is required", "gz-git run recover <task>"), 1
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
	previous := receipts[task]
	if previous.Status == TaskExecutionStatusDone && validateDoneReceipt(task, &execution, &previous) == nil {
		r := response(TaskExecutionStatusDone, task, "task execution is already DONE: "+previous.Reason, "no action required", "run-status")
		r.Execution, r.Receipt = &execution, &previous
		return r, 0
	}
	if !isPartialCleanupReceipt(execution, previous) {
		return response(TaskExecutionStatusBlocked, task, "run-recover requires the recorded exit-3 provider cleanup failure", "inspect task runtime receipt evidence"), 1
	}
	owner, err := s.owner(ctx)
	if err != nil || !sameOwner(execution.Owner, owner) {
		res := response(TaskExecutionStatusBlocked, task, "only "+execution.Owner.String()+" may recover this task", "run as the recorded owner")
		appendIdentityDiagnostic(&res, err)
		return res, 1
	}
	cfg, err := s.config()
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, err.Error(), "repair task runtime configuration"), 1
	}
	receipt, err := s.recoveryReceipt(ctx, execution, previous, cfg)
	if err != nil {
		return response(TaskExecutionStatusBlocked, task, "partial cleanup recovery is not proven: "+err.Error(), "inspect source delivery and residual task paths"), 1
	}
	if err := store.appendReceipt(receipt); err != nil {
		return response(TaskExecutionStatusBlocked, task, "persist recovery receipt failed: "+err.Error(), "repair task runtime storage and retry"), 1
	}
	r := response(TaskExecutionStatusDone, task, receipt.Reason, "no action required", "run-status")
	r.Execution, r.Receipt, r.Diagnostics = &execution, &receipt, receipt.Diagnostics
	return r, 0
}

func isPartialCleanupReceipt(execution TaskExecution, receipt TaskReceipt) bool {
	if receipt.Task != execution.Task || receipt.Owner != execution.Owner || receipt.Branch != execution.Branch || receipt.Worktree != execution.Worktree ||
		receipt.Operation != "finish" || receipt.Status != TaskExecutionStatusBlocked || receipt.Reason != partialCleanupReason ||
		strings.TrimSpace(receipt.SourceHeadBefore) == "" || strings.TrimSpace(receipt.TaskHead) == "" || !receipt.SourcePushed {
		return false
	}
	for _, diagnostic := range receipt.Diagnostics {
		args := strings.Join(diagnostic.Args, " ")
		if diagnostic.Command == integrationProviderGZGit && (args == "integrate run" || args == "integrate run --no-fetch") && diagnostic.ExitCode == 3 {
			return true
		}
	}
	return false
}

func (s *Service) recoveryReceipt(ctx context.Context, execution TaskExecution, previous TaskReceipt, cfg TaskRuntimeConfig) (TaskReceipt, error) {
	r := TaskReceipt{Task: execution.Task, Operation: "recover", Status: TaskExecutionStatusDone,
		Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree,
		CreatedAt: s.now().UTC(), SourceHeadBefore: previous.SourceHeadBefore, BaseHead: previous.BaseHead,
		TaskHead: previous.TaskHead, ToolVersion: toolVersion(), ToolRevision: toolRevision()}
	probe := func(args ...string) (CommandResult, error) {
		result, err := s.command(ctx, s.root, "git", args...)
		r.Diagnostics = append(r.Diagnostics, diagnostic(result))
		if err != nil {
			return result, err
		}
		return result, nil
	}
	source, err := probe("rev-parse", execution.Source)
	if err != nil || source.ExitCode != 0 || strings.TrimSpace(source.Stdout) == "" {
		return r, fmt.Errorf("resolve current source head")
	}
	r.SourceHead = strings.TrimSpace(source.Stdout)
	contained, err := probe("merge-base", "--is-ancestor", previous.TaskHead, r.SourceHead)
	if err != nil || contained.ExitCode != 0 {
		return r, fmt.Errorf("recorded task head is not contained in the current source")
	}
	pushed, err := probe("rev-list", "--count", "origin/"+execution.Source+".."+execution.Source)
	if err != nil || pushed.ExitCode != 0 || strings.TrimSpace(pushed.Stdout) != "0" {
		return r, fmt.Errorf("current source branch is not pushed")
	}
	r.SourcePushed = true
	_, statErr := os.Stat(execution.Worktree)
	if statErr != nil && !os.IsNotExist(statErr) {
		return r, fmt.Errorf("inspect residual worktree: %w", statErr)
	}
	r.WorktreeRemoved = os.IsNotExist(statErr)
	local, err := probe("show-ref", "--verify", "--quiet", "refs/heads/"+execution.Branch)
	if err != nil || (local.ExitCode != 0 && local.ExitCode != 1) {
		return r, fmt.Errorf("inspect residual local branch")
	}
	r.LocalBranchRemoved = local.ExitCode == 1
	remoteArgs, removedExit := []string{"ls-remote", "--exit-code", "--heads", "origin", execution.Branch}, 2
	remoteName := "origin branch " + execution.Branch
	if cfg.IntegrationNetworkPolicy == IntegrationNetworkPolicyNoFetch {
		remoteArgs, removedExit = []string{"show-ref", "--verify", "--quiet", "refs/remotes/origin/" + execution.Branch}, 1
		remoteName = "origin tracking ref refs/remotes/origin/" + execution.Branch
	}
	remote, err := probe(remoteArgs...)
	if err != nil || (remote.ExitCode != 0 && remote.ExitCode != removedExit) {
		return r, fmt.Errorf("inspect residual remote branch")
	}
	r.RemoteBranchRemoved = remote.ExitCode == removedExit
	residuals := make([]string, 0, 3)
	if !r.WorktreeRemoved {
		residuals = append(residuals, "worktree "+execution.Worktree)
	}
	if !r.LocalBranchRemoved {
		residuals = append(residuals, "local branch refs/heads/"+execution.Branch)
	}
	if !r.RemoteBranchRemoved {
		residuals = append(residuals, remoteName)
	}
	if len(residuals) == 0 {
		return r, fmt.Errorf("provider cleanup left no residual path or ref to recover")
	}
	r.Reason = "integration confirmed after partial provider cleanup; residual " + strings.Join(residuals, ", ") + " left untouched"
	return r, nil
}

// validateDoneReceipt accepts both normal run-finish receipts and the narrow
// recovery receipt that proves integration while deliberately retaining a
// provider-cleanup residual.
func validateDoneReceipt(task string, execution *TaskExecution, receipt *TaskReceipt) error {
	if err := validateIntegratedFinish(task, execution, receipt); err == nil {
		return nil
	}
	if execution == nil || receipt == nil || normalize(task) == "" || execution.Task != task || receipt.Task != execution.Task ||
		receipt.Owner != execution.Owner || receipt.Branch != execution.Branch || receipt.Worktree != execution.Worktree ||
		receipt.Operation != "recover" || receipt.Status != TaskExecutionStatusDone ||
		strings.TrimSpace(receipt.SourceHeadBefore) == "" || strings.TrimSpace(receipt.TaskHead) == "" || strings.TrimSpace(receipt.SourceHead) == "" || !receipt.SourcePushed ||
		(receipt.WorktreeRemoved && receipt.LocalBranchRemoved && receipt.RemoteBranchRemoved) {
		return fmt.Errorf("receipt is not a verified terminal completion")
	}
	return nil
}
