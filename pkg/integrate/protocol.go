// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// Machine JSON protocol (TASK-302). These documents are the published
// contract for callers that drive integrate run/receipt/cleanup — snake_case
// field names, schema_version 1, and an exit_code mirror so a caller can
// correlate the document with the process status without parsing prose.

// RunProtocol is the integrate run --json document.
type RunProtocol struct {
	SchemaVersion       int              `json:"schema_version"`
	Command             string           `json:"command"`
	Integrated          bool             `json:"integrated"`
	ExecutionID         string           `json:"execution_id"`
	Generation          string           `json:"generation"`
	Owner               *OwnerIdentity   `json:"owner,omitempty"`
	TaskSHA             string           `json:"task_sha"`
	TargetRef           string           `json:"target_ref"`
	SourcePushed        bool             `json:"source_pushed"`
	ReceiptPhase        string           `json:"receipt_phase"`
	WorktreeRemoved     bool             `json:"worktree_removed"`
	LocalBranchRemoved  bool             `json:"local_branch_removed"`
	RemoteBranchRemoved bool             `json:"remote_branch_removed"`
	ReclaimSkipped      string           `json:"reclaim_skipped"`
	ReclaimDone         []string         `json:"reclaim_done"`
	ReclaimFailed       []string         `json:"reclaim_failed"`
	GitReclaim          *GitReclaimProof `json:"git_reclaim,omitempty"`
	// ResourceWaitDurationMS and ValidationDurationMS separate the check's
	// slot-admission wait from its gate runtime as explicit integer
	// milliseconds; both render even when zero, so an absent gate is
	// distinguishable from a missing field.
	ResourceWaitDurationMS int `json:"resource_wait_duration_ms"`
	ValidationDurationMS   int `json:"validation_duration_ms"`
	// ReadinessFailures carries typed timeout evidence captured at the process
	// boundary. It is absent when every readiness subprocess finished within
	// its budget, preserving the v1 success shape for existing consumers.
	ReadinessFailures []ReadinessFailure `json:"readiness_failures,omitempty"`
	// PreMutationOutcome is a separately durable, context-bound refusal that
	// this invocation recorded before it reached the target push. Its absence
	// is never evidence that a push did not happen.
	PreMutationOutcome *PreMutationOutcome `json:"pre_mutation_outcome,omitempty"`
	ExitCode           int                 `json:"exit_code"`
	Error              string              `json:"error,omitempty"`
}

// ReadinessFailure identifies one readiness subprocess that exceeded its
// declared budget. Nullable fields render as JSON null when a process was
// killed before it could report an exit code or no cancellation request ran.
type ReadinessFailure struct {
	Stage              string              `json:"stage"`
	Class              string              `json:"class"`
	BudgetMS           int                 `json:"budget_ms"`
	ElapsedMS          int                 `json:"elapsed_ms"`
	LeafExitCode       *int                `json:"leaf_exit_code"`
	TerminationRequest *TerminationRequest `json:"termination_request"`
	DiagnosticPath     *string             `json:"diagnostic_path"`
}

// TerminationRequest records a cancellation request actually made by the
// process supervisor. It is a request, not proof that the OS delivered it.
type TerminationRequest struct {
	Method string  `json:"method"`
	Signal *string `json:"signal"`
}

// BuildRunProtocol renders a run result. A nil report with an error is still
// a document: the protocol never hides a failure behind an empty reply.
func BuildRunProtocol(report *RunReport, err error, exitCode int) *RunProtocol {
	p := &RunProtocol{
		SchemaVersion: evidenceSchemaVersion,
		Command:       "integrate-run",
		ExitCode:      exitCode,
	}
	if report != nil {
		p.Integrated = report.Integrated
		p.TaskSHA = report.SHA
		p.ReclaimSkipped = report.Reclaim.Skipped
		p.ReclaimDone = report.Reclaim.Done
		p.ReclaimFailed = report.Reclaim.Failed
		p.GitReclaim = report.Reclaim.GitProof
		if report.Check != nil {
			p.TargetRef = "refs/heads/" + targetBranchName(report.Check.Plan.Target, report.Check.Plan.Remote)
			p.ResourceWaitDurationMS = int(report.Check.ResourceWaitDuration.Milliseconds())
			p.ValidationDurationMS = int(report.Check.ValidationDuration.Milliseconds())
			p.ReadinessFailures = report.Check.ReadinessFailures
		}
		if report.Receipt != nil {
			p.ExecutionID = report.Receipt.ExecutionID
			p.Generation = report.Receipt.Generation
			owner := report.Receipt.Owner
			p.Owner = &owner
			p.SourcePushed = report.Receipt.SourcePushed
			p.ReceiptPhase = report.Receipt.Phase
			p.WorktreeRemoved = report.Receipt.WorktreeRemoved
			p.LocalBranchRemoved = report.Receipt.LocalBranchRemoved
			p.RemoteBranchRemoved = report.Receipt.RemoteBranchRemoved
			if p.TaskRefEmpty() {
				p.TargetRef = report.Receipt.TargetRef
			}
		}
		if report.PreMutationOutcome != nil {
			outcome := *report.PreMutationOutcome
			p.PreMutationOutcome = &outcome
			p.ExecutionID = outcome.ExecutionID
			p.Generation = outcome.Generation
			owner := outcome.Owner
			p.Owner = &owner
			p.TaskSHA = outcome.TaskSHA
			p.TargetRef = outcome.TargetRef
		}
	}
	if err != nil {
		p.Error = err.Error()
	}
	return p
}

// TaskRefEmpty reports that the protocol document carries no target ref yet,
// so receipt-backed fields must still be filled from the durable evidence.
func (p *RunProtocol) TaskRefEmpty() bool { return p.TargetRef == "" }

// CleanupProtocol is the integrate cleanup --json document.
type CleanupProtocol struct {
	SchemaVersion            int            `json:"schema_version"`
	Command                  string         `json:"command"`
	ExecutionID              string         `json:"execution_id"`
	Generation               string         `json:"generation"`
	Owner                    *OwnerIdentity `json:"owner,omitempty"`
	TaskSHA                  string         `json:"task_sha"`
	TargetRef                string         `json:"target_ref"`
	SourcePushed             bool           `json:"source_pushed"`
	RemoteTargetContainsTask bool           `json:"remote_target_contains_task"`
	WorktreeRemoved          bool           `json:"worktree_removed"`
	LocalBranchRemoved       bool           `json:"local_branch_removed"`
	RemoteBranchRemoved      bool           `json:"remote_branch_removed"`
	Phase                    string         `json:"phase"`
	Complete                 bool           `json:"complete"`
	Actions                  []string       `json:"actions"`
	ResidualsPreserved       []string       `json:"residuals_preserved"`
	ExitCode                 int            `json:"exit_code"`
	Error                    string         `json:"error,omitempty"`
}

// BuildCleanupProtocol renders a cleanup result.
func BuildCleanupProtocol(report *CleanupReport, err error, exitCode int) *CleanupProtocol {
	p := &CleanupProtocol{
		SchemaVersion: evidenceSchemaVersion,
		Command:       "integrate-cleanup",
		ExitCode:      exitCode,
	}
	if report != nil && report.Receipt != nil {
		p.ExecutionID = report.Receipt.ExecutionID
		p.Generation = report.Receipt.Generation
		owner := report.Receipt.Owner
		p.Owner = &owner
		p.TaskSHA = report.Receipt.TaskSHA
		p.TargetRef = report.Receipt.TargetRef
		p.SourcePushed = report.Receipt.SourcePushed
		p.WorktreeRemoved = report.Receipt.WorktreeRemoved
		p.LocalBranchRemoved = report.Receipt.LocalBranchRemoved
		p.RemoteBranchRemoved = report.Receipt.RemoteBranchRemoved
		p.Phase = report.Receipt.Phase
	}
	if report != nil {
		p.RemoteTargetContainsTask = report.RemoteTargetContainsTask
		p.Actions = report.Actions
		p.ResidualsPreserved = report.ResidualsPreserved
		p.Complete = report.Complete
	}
	if err != nil {
		p.Error = err.Error()
	}
	return p
}

// ReceiptOptions configures a noncreating receipt query.
type ReceiptOptions struct {
	RepoPath    string
	ExecutionID string
	Generation  string
}

// QueryReceipt reads one durable receipt. It creates nothing — not even the
// gz-integrate directory — and verifies the exact generation when given.
func QueryReceipt(ctx context.Context, exec *gitcmd.Executor, opts ReceiptOptions) (*Receipt, error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
	}
	if err := validateOpaqueToken(opts.ExecutionID, "execution-id"); err != nil {
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
	receipt, ok, err := readReceipt(common, opts.ExecutionID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrCleanupReceiptMissing, opts.ExecutionID)
	}
	if opts.Generation != "" && receipt.Generation != opts.Generation {
		return receipt, fmt.Errorf("%w: receipt is %q, got %q", ErrCleanupGenerationMismatch, receipt.Generation, opts.Generation)
	}
	return receipt, nil
}
