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

import "time"

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
	Task                string               `json:"task"`
	Operation           string               `json:"operation"`
	Status              string               `json:"status"`
	Owner               TaskOwner            `json:"owner"`
	PerformedBy         *TaskOwner           `json:"performedBy,omitempty"`
	Branch              string               `json:"branch"`
	Worktree            string               `json:"worktree"`
	Reason              string               `json:"reason,omitempty"`
	CreatedAt           time.Time            `json:"createdAt"`
	SourceHead          string               `json:"sourceHead,omitempty"`
	SourceHeadBefore    string               `json:"sourceHeadBefore,omitempty"`
	BaseHead            string               `json:"baseHead,omitempty"`
	TaskHead            string               `json:"taskHead,omitempty"`
	SourcePushed        bool                 `json:"sourcePushed"`
	WorktreeRemoved     bool                 `json:"worktreeRemoved"`
	LocalBranchRemoved  bool                 `json:"localBranchRemoved"`
	RemoteBranchRemoved bool                 `json:"remoteBranchRemoved"`
	ToolVersion         string               `json:"tool_version,omitempty"`
	ToolRevision        string               `json:"tool_revision,omitempty"`
	Diagnostics         []CommandDiagnostics `json:"diagnostics,omitempty"`
}

// Task execution status tokens as they appear in response documents and
// receipt records.
const (
	TaskExecutionStatusActive  = "ACTIVE"
	TaskExecutionStatusReady   = "READY"
	TaskExecutionStatusAborted = "ABORTED"
	TaskExecutionStatusBlocked = "BLOCKED"
	TaskExecutionStatusDone    = "DONE"
	TaskExecutionStatusUnknown = "UNKNOWN"
)

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
