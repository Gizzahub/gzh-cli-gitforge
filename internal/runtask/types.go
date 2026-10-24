// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Package runtask implements the repository task-run lifecycle that ADR-0055
// moves from ce-agent-kit into gz-git: run identity, owner, lock, and receipt
// state live here, and task-manager consumes them through the JSON surface.
//
// The behavioral reference is the parity fixture suite in tests/parity/ —
// golden files there are machine-captured from the autonomous CE binary
// (master f927ae5d). When this implementation and the fixtures disagree, the
// fixtures win; docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md describes the
// same surface at the source level.
package runtask

import (
	"fmt"
	"strings"
	"time"
)

const (
	// TaskRuntimeConfigFile holds this checkout's runtime identity --
	// repository-id, worktree-roots, integration-provider. Those keys name
	// machine-local absolute paths, which is why the repository's .gitignore
	// should exclude it.
	TaskRuntimeConfigFile = ".gz-git-task.yaml"
	// RuntimeStateDir names the directory the run lifecycle keeps its state
	// in, rooted at the git common dir: `<git-common-dir>/gz-git/`. The
	// common dir is shared by a repository and every linked worktree and is
	// never part of any working tree, so state written there outlives a
	// reclaimed worktree without leaving a path that could recreate one.
	RuntimeStateDir = "gz-git"
)

// TaskExecutionType classifies a task run session.
type TaskExecutionType string

// Task execution types, matching the commit type the task branch name and
// its worktree directory were created with.
const (
	TaskExecutionTypeFeat     TaskExecutionType = "feat"
	TaskExecutionTypeFix      TaskExecutionType = "fix"
	TaskExecutionTypeRefactor TaskExecutionType = "refactor"
	TaskExecutionTypeDocs     TaskExecutionType = "docs"
	TaskExecutionTypeTest     TaskExecutionType = "test"
	TaskExecutionTypeChore    TaskExecutionType = "chore"
	TaskExecutionTypePerf     TaskExecutionType = "perf"
)

// Valid reports whether t is one of the declared execution types.
func (t TaskExecutionType) Valid() bool {
	switch t {
	case TaskExecutionTypeFeat, TaskExecutionTypeFix, TaskExecutionTypeRefactor,
		TaskExecutionTypeDocs, TaskExecutionTypeTest, TaskExecutionTypeChore, TaskExecutionTypePerf:
		return true
	}
	return false
}

// TaskOwner expresses who controls one execution.
type TaskOwner struct {
	Actor string `json:"actor"`
	Host  string `json:"host"`
	Kind  string `json:"kind"`
}

// Valid reports whether the owner names both an actor and a host.
func (o TaskOwner) Valid() bool { return o.Actor != "" && o.Host != "" }

// String renders the owner as "actor/host", the form reasons quote.
func (o TaskOwner) String() string {
	return o.Actor + "/" + o.Host
}

// TaskExecution is the current execution identity from metadata.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract; parity goldens pin them.
type TaskExecution struct {
	Task      string            `json:"task"`
	Type      TaskExecutionType `json:"type"`
	Purpose   string            `json:"purpose,omitempty"`
	Source    string            `json:"source"`
	Owner     TaskOwner         `json:"owner"`
	Branch    string            `json:"branch"`
	Worktree  string            `json:"worktree"`
	StartedAt time.Time         `json:"startedAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// CommandDiagnostics records one executed or not-started command's evidence
// for receipt diagnostics.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract; parity goldens pin them.
type CommandDiagnostics struct {
	Command    string   `json:"command,omitempty"`
	Executable string   `json:"executable,omitempty"`
	Args       []string `json:"args,omitempty"`
	Cwd        string   `json:"cwd,omitempty"`
	ExitCode   int      `json:"exitCode"`
	Stdout     string   `json:"stdout,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
	Error      string   `json:"error,omitempty"`
	NotStarted bool     `json:"notStarted,omitempty"`
}

// IntegrationProviderDiagnostics records the read-only contract probe for the
// integration engine selected by the repository. When the engine is this
// binary the probe reports capabilities from in-process knowledge; the
// executable and version still name this build.
type IntegrationProviderDiagnostics struct {
	Name         string          `json:"name"`
	Present      bool            `json:"present"`
	Version      string          `json:"version,omitempty"`
	Executable   string          `json:"executable,omitempty"`
	Capabilities map[string]bool `json:"capabilities"`
}

// TaskReceipt is the durable evidence one lifecycle operation wrote: the
// decision it made, the Git heads it observed, and the recovery state it
// proved before answering DONE.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract; parity goldens pin them.
type TaskReceipt struct {
	Task                string     `json:"task"`
	Operation           string     `json:"operation"`
	Status              string     `json:"status"`
	Purpose             string     `json:"purpose,omitempty"`
	Owner               TaskOwner  `json:"owner"`
	PerformedBy         *TaskOwner `json:"performedBy,omitempty"`
	Branch              string     `json:"branch"`
	Worktree            string     `json:"worktree"`
	Reason              string     `json:"reason,omitempty"`
	ReasonCode          string     `json:"reasonCode,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	SourceHead          string     `json:"sourceHead,omitempty"`
	SourceHeadBefore    string     `json:"sourceHeadBefore,omitempty"`
	BaseHead            string     `json:"baseHead,omitempty"`
	TaskHead            string     `json:"taskHead,omitempty"`
	SourcePushed        bool       `json:"sourcePushed"`
	WorktreeRemoved     bool       `json:"worktreeRemoved"`
	LocalBranchRemoved  bool       `json:"localBranchRemoved"`
	RemoteBranchRemoved bool       `json:"remoteBranchRemoved"`
	// DiscardScope names the reclamation a discard receipt performed. Empty is
	// the default scope, which verified remote branch absence; "local-only"
	// reclaimed local resources without network access, leaving remote refs
	// unchecked and unchanged.
	DiscardScope       string               `json:"discardScope,omitempty"`
	RemoteBranchStatus string               `json:"remoteBranchStatus,omitempty"`
	ToolVersion        string               `json:"tool_version,omitempty"`
	ToolRevision       string               `json:"tool_revision,omitempty"`
	Diagnostics        []CommandDiagnostics `json:"diagnostics,omitempty"`
	// ProcessID and ProcessHost name the run-finish process that wrote a
	// FINISHING claim, so a claim whose process died can be told apart from
	// one still waiting on the integration provider.
	ProcessID   int    `json:"processId,omitempty"`
	ProcessHost string `json:"processHost,omitempty"`
}

// Task execution status tokens as they appear in response documents and
// receipt records.
const (
	TaskExecutionStatusActive    = "ACTIVE"
	TaskExecutionStatusReady     = "READY"
	TaskExecutionStatusFinishing = "FINISHING"
	TaskExecutionStatusAborted   = "ABORTED"
	TaskExecutionStatusBlocked   = "BLOCKED"
	TaskExecutionStatusDone      = "DONE"
	TaskExecutionStatusUnknown   = "UNKNOWN"
)

// TaskExecutionPurpose is the lifecycle purpose a start declared. Delivery
// lands on the integration branch; experiment and review end without
// integrating.
type TaskExecutionPurpose string

// Task execution purposes. Delivery is the default: run-start without an
// explicit purpose starts a delivery execution, whose branches carry the
// dev/ prefix and whose end state is run-finish.
const (
	TaskExecutionPurposeDelivery   TaskExecutionPurpose = "delivery"
	TaskExecutionPurposeExperiment TaskExecutionPurpose = "experiment"
	TaskExecutionPurposeReview     TaskExecutionPurpose = "review"
)

// Valid reports whether p is one of the declared purposes.
func (p TaskExecutionPurpose) Valid() bool {
	switch p {
	case TaskExecutionPurposeDelivery, TaskExecutionPurposeExperiment, TaskExecutionPurposeReview:
		return true
	}
	return false
}

// BranchPrefix is the task-branch namespace a purpose owns.
func (p TaskExecutionPurpose) BranchPrefix() string {
	switch p.Effective() {
	case TaskExecutionPurposeExperiment:
		return "exp"
	case TaskExecutionPurposeReview:
		return "review"
	case TaskExecutionPurposeDelivery:
		return "dev"
	default:
		return "dev"
	}
}

// Effective reports the purpose a recorded execution carries. Records that
// predate the purpose field are delivery executions: the dev/ prefix and the
// finish-first lifecycle say so.
func (p TaskExecutionPurpose) Effective() TaskExecutionPurpose {
	if p == "" {
		return TaskExecutionPurposeDelivery
	}
	return p
}

// executionPurpose reports the effective purpose of a recorded execution.
func executionPurpose(execution TaskExecution) TaskExecutionPurpose {
	return TaskExecutionPurpose(execution.Purpose).Effective()
}

// expectedExecutionBranch is the branch name an execution's purpose, owner,
// type, and task record together imply.
func expectedExecutionBranch(execution TaskExecution) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", executionPurpose(execution).BranchPrefix(), execution.Owner.Actor, execution.Owner.Host, execution.Type, execution.Task)
}

// purposeMatchesReceipt reports whether a receipt claims the execution's own
// purpose.
func purposeMatchesReceipt(execution TaskExecution, receipt TaskReceipt) bool {
	return executionPurpose(execution) == TaskExecutionPurpose(receipt.Purpose).Effective()
}

// validateExecutionPurpose refuses an execution whose recorded purpose value
// is invalid, or whose branch does not carry the namespace its purpose owns.
func validateExecutionPurpose(execution TaskExecution) error {
	if execution.Purpose != "" && !TaskExecutionPurpose(execution.Purpose).Valid() {
		return fmt.Errorf("execution purpose %q is invalid", execution.Purpose)
	}
	if execution.Branch != "" && !strings.HasPrefix(execution.Branch, executionPurpose(execution).BranchPrefix()+"/") {
		return fmt.Errorf("execution branch does not match its purpose")
	}
	// Older records predate type validation. Preserve their delivery
	// interpretation when that historical field is absent, while enforcing the
	// full shape for complete modern identities.
	if execution.Type.Valid() && execution.Branch != expectedExecutionBranch(execution) {
		return fmt.Errorf("execution branch does not match its purpose")
	}
	return nil
}

// receiptPurposeValid reports whether a receipt carries a purpose value this
// runtime writes.
func receiptPurposeValid(receipt TaskReceipt) bool {
	return receipt.Purpose == "" || TaskExecutionPurpose(receipt.Purpose).Valid()
}

// receiptPurposeMatchesOrAbsent accepts a receipt only when it names no task,
// or carries a valid purpose that matches the execution it is bound to.
func receiptPurposeMatchesOrAbsent(execution TaskExecution, receipt TaskReceipt) bool {
	return receipt.Task == "" || (receiptPurposeValid(receipt) && purposeMatchesReceipt(execution, receipt))
}

// IsTerminalExecutionStatus reports whether a record has reached an end
// state -- one that no further verb can move. DONE and ABORTED are terminal;
// ACTIVE is running, and BLOCKED and UNKNOWN are awaiting a decision, which
// is why both still advertise `run-abort`. READY is not an execution status:
// it is the list-level answer when the runtime responded and nothing is
// blocked, so this predicate does not treat it as terminal or live.
func IsTerminalExecutionStatus(status string) bool {
	return status == TaskExecutionStatusDone || status == TaskExecutionStatusAborted
}

// TaskResponse is the single response document every verb prints with
// --json; task-manager parses only this shape.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract; parity goldens pin them.
type TaskResponse struct {
	SchemaVersion  int                             `json:"schemaVersion"`
	Status         string                          `json:"status"`
	ActiveCount    int                             `json:"activeCount"`
	Task           string                          `json:"task"`
	AllowedActions []string                        `json:"allowedActions"`
	Reason         string                          `json:"reason"`
	ReasonCode     string                          `json:"reasonCode,omitempty"`
	NextAction     string                          `json:"nextAction"`
	FinishReady    bool                            `json:"finishReady"`
	Warnings       []string                        `json:"warnings,omitempty"`
	Lock           *TaskLockInfo                   `json:"lock,omitempty"`
	Execution      *TaskExecution                  `json:"execution,omitempty"`
	Executions     []TaskExecution                 `json:"executions,omitempty"`
	States         []DerivedTaskState              `json:"states,omitempty"`
	Receipt        *TaskReceipt                    `json:"receipt,omitempty"`
	Diagnostics    []CommandDiagnostics            `json:"diagnostics,omitempty"`
	Provider       *IntegrationProviderDiagnostics `json:"provider,omitempty"`
}

// DerivedTaskState pairs one recorded execution with the status derived from
// live repository and receipt evidence.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract; parity goldens pin them.
type DerivedTaskState struct {
	Execution      TaskExecution        `json:"execution"`
	Status         string               `json:"status"`
	AllowedActions []string             `json:"allowedActions"`
	Reason         string               `json:"reason"`
	NextAction     string               `json:"nextAction"`
	FinishReady    bool                 `json:"finishReady"`
	Diagnostics    []CommandDiagnostics `json:"diagnostics,omitempty"`
}

// TaskLockInfo describes the mutation lock file for diagnostics: where it
// lives, who holds it, and whether that holder is still running.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract; parity goldens pin them.
type TaskLockInfo struct {
	Path       string    `json:"path"`
	PID        int       `json:"pid,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	AgeSeconds int64     `json:"ageSeconds,omitempty"`
	PIDRunning *bool     `json:"pidRunning,omitempty"`
}

// TaskRuntimeConfig declares repository-owned runtime configuration.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract; parity goldens pin them.
type TaskRuntimeConfig struct {
	SchemaVersion            int                      `yaml:"schema-version,omitempty" json:"schemaVersion"`
	RepositoryID             string                   `yaml:"repository-id" json:"repositoryId"`
	WorktreeRoots            map[string]string        `yaml:"worktree-roots" json:"worktreeRoots"`
	IntegrationProvider      string                   `yaml:"integration-provider" json:"integrationProvider"`
	IntegrationNetworkPolicy IntegrationNetworkPolicy `yaml:"integration-network-policy,omitempty" json:"integrationNetworkPolicy"`
}

// IntegrationNetworkPolicy limits network use by the declared integration
// provider. An omitted value preserves the existing provider-controlled mode.
type IntegrationNetworkPolicy string

// Integration network policy values accepted by the runtime declaration.
const (
	IntegrationNetworkPolicyAllow   IntegrationNetworkPolicy = "allow"
	IntegrationNetworkPolicyNoFetch IntegrationNetworkPolicy = "no-fetch"
)
