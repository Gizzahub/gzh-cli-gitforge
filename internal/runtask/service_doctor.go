package runtask

import (
	"context"
	"fmt"
	"strings"
)

// Doctor answers whether the task runtime dependencies of this checkout are
// ready, without acquiring the mutation lock: configuration, integration
// provider presence and capabilities, lock, and receipt readability.
func (s *Service) Doctor(ctx context.Context) TaskResponse {
	res := response(TaskExecutionStatusActive, "", "task runtime dependencies are ready", "gz-git run start <task> --type <type>", "run-start", "run-status", "run-list")
	cfg, err := s.config()
	if err != nil {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, err.Error(), "configure "+TaskRuntimeConfigFile
		return res
	}
	host, hostErr := s.resolveHost(ctx)
	if hostErr != nil {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, hostErr.Error(), identityNextAction(hostErr)
		appendIdentityDiagnostic(&res, hostErr)
		return res
	}
	if _, err := s.worktreeRoot(cfg, host); err != nil {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, err.Error(), "declare this host in worktree-roots"
		return res
	}
	if cfg.SchemaVersion == 0 {
		res.Warnings = append(res.Warnings, "task runtime config has no schema-version; add schema-version: 1")
	}
	provider, err := integrationProviderFor(cfg.IntegrationProvider, s.engine)
	if err != nil {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, err.Error(), "configure a supported integration-provider"
		return res
	}
	providerReport, providerDiagnostics, providerErr := provider.Diagnose(ctx, s.root)
	res.Provider = &providerReport
	res.Diagnostics = append(res.Diagnostics, providerDiagnostics...)
	if providerErr != nil {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, providerErr.Error(), "install or repair the configured integration provider and its integrate check/run capabilities"
		return res
	}
	store, err := s.store(ctx)
	if err != nil {
		res.Status, res.Reason = TaskExecutionStatusBlocked, err.Error()
		return res
	}
	if lock, err := store.lockInfo(s.now()); err == nil {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, "stale or active mutation lock: "+store.lockPath(), "inspect the lock owner; do not delete it automatically"
		res.Lock = &lock
		return res
	}
	if _, err := store.latestReceipts(); err != nil {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, "read task receipts: "+err.Error(), "inspect task runtime receipt evidence"
		return res
	}
	version, err := s.command(ctx, s.root, "wt", "--version")
	if err != nil {
		res.Status, res.Reason = TaskExecutionStatusBlocked, err.Error()
		return res
	}
	res.Diagnostics = append(res.Diagnostics, diagnostic(version))
	if version.ExitCode != 0 || !strings.Contains(version.Stdout+version.Stderr, worktrunkVersion) {
		res.Status, res.Reason, res.NextAction = TaskExecutionStatusBlocked, "Worktrunk "+worktrunkVersion+" is required", "activate the repository's Worktrunk pilot"
		return res
	}
	listed, err := s.worktrunkList(ctx)
	if err != nil {
		res.Status, res.Reason = TaskExecutionStatusBlocked, err.Error()
		res.Diagnostics = append(res.Diagnostics, diagnostic(listed.Diagnostic))
		return res
	}
	if listed.SkippedNoBranch > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("Worktrunk listed %d item(s) with no branch (a detached worktree lists this way); they were skipped and no task lookup is affected", listed.SkippedNoBranch))
	}
	if listed.SkippedNoPath > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("Worktrunk listed %d item(s) that name a branch but no worktree.path; they were skipped, so a task on those branches is reported missing", listed.SkippedNoPath))
	}
	res.Diagnostics = append(res.Diagnostics, diagnostic(listed.Diagnostic))
	return res
}
