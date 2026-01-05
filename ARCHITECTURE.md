# Architecture Design Document

**Project**: gzh-cli-gitforge
**Version**: 1.0
**Last Updated**: 2025-11-27
**Status**: Draft

______________________________________________________________________

## Table of Contents

1. [Executive Summary](#1-executive-summary)
1. [Architectural Overview](#2-architectural-overview)
1. [Design Principles](#3-design-principles)
1. [Component Design](#4-component-design)
1. [Interface Contracts](#5-interface-contracts)
1. [Data Flow](#6-data-flow)
1. [Error Handling Strategy](#7-error-handling-strategy)
1. [Testing Architecture](#8-testing-architecture)
1. [Performance Considerations](#9-performance-considerations)
1. [Security Architecture](#10-security-architecture)
1. [Deployment Architecture](#11-deployment-architecture)
1. [Design Decisions](#12-design-decisions)

______________________________________________________________________

## 1. Executive Summary

### 1.1 Architectural Goals

gzh-cli-gitforge adopts a **Library-First Architecture** with the following goals:

1. **Dual-Purpose Design**: Function as both standalone CLI and reusable Go library
1. **Clean Separation**: Zero coupling between library code (`pkg/`) and CLI code (`cmd/`)
1. **Maximum Reusability**: Enable easy integration into gzh-cli and other projects
1. **Interface-Driven**: All core functionality via well-defined interfaces
1. **Testability**: 100% mockable components for comprehensive testing

### 1.2 Key Architectural Decisions

| Decision                       | Rationale                                     | Trade-offs                            |
| ------------------------------ | --------------------------------------------- | ------------------------------------- |
| Library-First over CLI-First   | Enables reuse in gzh-cli; better API design   | More upfront design effort            |
| Git CLI over go-git library    | Maximum compatibility; simpler                | External dependency on Git            |
| Interfaces over concrete types | Testability; extensibility                    | More files, indirection               |
| Functional options pattern     | API extensibility without breaking changes    | More boilerplate                      |
| Context propagation            | Cancellation, timeouts, request-scoped values | Every function signature includes ctx |

______________________________________________________________________

## 2. Architectural Overview

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    gzh-cli-gitforge System                        │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌────────────────────────────────────────────────────┐     │
│  │              CLI Layer (cmd/)                       │     │
│  │  ┌──────────┐  ┌─────────┐  ┌──────────┐          │     │
│  │  │ Commands │  │ Output  │  │    UI    │          │     │
│  │  │  (Cobra) │  │ Format  │  │ Progress │          │     │
│  │  └────┬─────┘  └────┬────┘  └─────┬────┘          │     │
│  └───────┼─────────────┼──────────────┼───────────────┘     │
│          │             │              │                      │
│  ┌───────┼─────────────┼──────────────┼───────────────┐     │
│  │       ▼             ▼              ▼               │     │
│  │          Public Library API (pkg/)                 │     │
│  │  ┌──────────┐  ┌────────┐  ┌────────┐  ┌────────┐ │     │
│  │  │Repository│  │ Branch │  │ History│  │ Merge  │ │     │
│  │  │  Client  │  │Manager │  │Analyzer│  │Manager │ │     │
│  │  └────┬─────┘  └───┬────┘  └───┬────┘  └───┬────┘ │     │
│  └───────┼────────────┼───────────┼───────────┼──────┘     │
│          │            │           │           │             │
│  ┌───────┼────────────┼───────────┼───────────┼──────┐     │
│  │       ▼            ▼           ▼           ▼      │     │
│  │      Internal Implementation (internal/)          │     │
│  │  ┌─────────┐  ┌─────────┐  ┌────────────┐        │     │
│  │  │ Git CMD │  │ Parsers │  │ Validation │        │     │
│  │  │Executor │  │ (status,│  │  (input)   │        │     │
│  │  │         │  │log,diff)│  │            │        │     │
│  │  └────┬────┘  └────┬────┘  └─────┬──────┘        │     │
│  └───────┼────────────┼──────────────┼───────────────┘     │
│          │            │              │                      │
│  ┌───────┼────────────┼──────────────┼───────────────┐     │
│  │       ▼            ▼              ▼               │     │
│  │              External Dependencies                │     │
│  │  ┌─────────┐  ┌────────────┐  ┌──────────┐      │     │
│  │  │ Git CLI │  │ Filesystem │  │   OS     │      │     │
│  │  │ (2.30+) │  │   (I/O)    │  │(exec,env)│      │     │
│  │  └─────────┘  └────────────┘  └──────────┘      │     │
│  └───────────────────────────────────────────────────┘     │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Layer Responsibilities

**CLI Layer (`cmd/`):**

- User interaction (prompts, confirmations)
- Command parsing (Cobra)
- Output formatting (table, JSON)
- Progress reporting
- Configuration file management

**Library Layer (`pkg/`):**

- Core Git operations
- Business logic (commit automation, conflict resolution)
- Public APIs for external consumers
- NO CLI dependencies (no Cobra, no fmt.Println)

**Internal Layer (`internal/`):**

- Git command execution
- Output parsing (status, log, diff)
- Input validation and sanitization
- Shared utilities (not exposed)

**External Layer:**

- Git CLI binary (system installation)
- Filesystem I/O
- Operating system (exec, environment)

______________________________________________________________________

## 3. Design Principles

### 3.1 SOLID Principles

**Single Responsibility:**

- Each package has one clear purpose
- `pkg/branch/` only handles branch operations
- `internal/gitcmd/` only executes Git commands

**Open/Closed:**

- Interfaces open for extension
- Functional options allow new parameters without breaking API

**Liskov Substitution:**

- All interface implementations are substitutable
- Mocks can replace real implementations

**Interface Segregation:**

- Small, focused interfaces (not god interfaces)
- `CommitManager`, `BranchManager` separate, not combined

**Dependency Inversion:**

- Depend on interfaces, not concretions
- Accept `Logger` interface, not `*zap.Logger`

### 3.2 Library-First Principles

**P1: Zero CLI Dependencies in pkg/**

```go
// ❌ WRONG: pkg/ code importing CLI framework
import "github.com/spf13/cobra"

// ✅ CORRECT: pkg/ only uses stdlib and interfaces
import (
    "context"
    "io"
)
```

**P2: Dependency Injection via Interfaces**

```go
// ❌ WRONG: Hard-coded logger
func Process() {
    log.Println("processing...")
}

// ✅ CORRECT: Logger injected via interface
func Process(ctx context.Context, logger Logger) {
    logger.Info("processing...")
}
```

**P3: Context Propagation**

```go
// ❌ WRONG: No context
func Clone(url, path string) error

// ✅ CORRECT: Context as first parameter
func Clone(ctx context.Context, url, path string) error
```

### 3.3 Go Idioms

**Functional Options Pattern:**

```go
type CloneOption func(*CloneConfig)

func WithBranch(b string) CloneOption {
    return func(c *CloneConfig) { c.Branch = b }
}

// Usage allows evolution without breaking changes
Clone(ctx, url, path, WithBranch("main"), WithDepth(1))
```

**Error Wrapping:**

```go
if err != nil {
    return fmt.Errorf("failed to clone repository: %w", err)
}
```

**Interface Satisfaction:**

```go
var _ CommitManager = (*commitManager)(nil) // Compile-time check
```

______________________________________________________________________

## 4. Component Design

### 4.1 Directory Structure

```
gzh-cli-gitforge/
├── pkg/                          # PUBLIC API
│   ├── repository/               # Core repo operations
│   │   ├── interfaces.go         # Repository, Client interfaces
│   │   ├── client.go             # Client implementation
│   │   ├── types.go              # Repository, Info, Status types
│   │   └── options.go            # Functional options
│   ├── branch/                   # Branch management
│   │   ├── interfaces.go         # BranchManager interface
│   │   ├── manager.go            # Manager implementation
│   │   ├── worktree.go           # Worktree operations
│   │   ├── workflow.go           # Parallel workflows
│   │   └── types.go              # Branch, Worktree, etc.
│   ├── history/                  # Git history analysis
│   │   ├── interfaces.go         # Analyzer interface
│   │   ├── analyzer.go           # Analyzer implementation
│   │   ├── stats.go              # Statistics calculation
│   │   ├── contributors.go       # Contributor analysis
│   │   └── types.go              # Commit, Statistics, etc.
│   ├── merge/                    # Advanced merge/rebase
│   │   ├── interfaces.go         # MergeManager interface
│   │   ├── manager.go            # Manager implementation
│   │   ├── conflict.go           # Conflict detection
│   │   ├── strategies.go         # Resolution strategies
│   │   └── types.go              # ConflictReport, etc.
│   └── config/                   # Configuration
│       ├── config.go             # Config struct
│       └── validation.go         # Config validation
│
├── internal/                     # INTERNAL (not exposed)
│   ├── gitcmd/                   # Git command execution
│   │   ├── executor.go           # Command executor
│   │   ├── sanitize.go           # Input sanitization
│   │   └── errors.go             # Error types
│   ├── parser/                   # Git output parsing
│   │   ├── status.go             # Parse git status
│   │   ├── log.go                # Parse git log
│   │   ├── diff.go               # Parse git diff
│   │   └── common.go             # Shared parsing utilities
│   └── validation/               # Input validation
│       ├── validator.go          # Validation logic
│       └── patterns.go           # Regex patterns
│
├── cmd/                          # CLI APPLICATION
│   └── gz-git/                  # Binary: gz-git
│       ├── main.go               # Entry point
│       ├── root.go               # Root command
│       └── internal/             # CLI-specific (not reusable)
│           ├── cli/              # Cobra commands
│           │   ├── branch/       # Branch commands
│           │   ├── history/      # History commands
│           │   └── merge/        # Merge commands
│           ├── output/           # Output formatting
│           │   ├── table.go      # Table renderer
│           │   ├── json.go       # JSON formatter
│           │   └── formatter.go  # Common interface
│           └── ui/               # User interface
│               ├── progress.go   # Progress bars
│               └── prompt.go     # User prompts
│
├── examples/                     # Library usage examples
│   ├── basic/                    # Basic usage
│   ├── branch/                   # Branch features
│   └── gzh_cli_integration/      # gzh-cli integration
│
├── test/                         # Integration & E2E tests
│   ├── integration/              # Integration tests
│   └── e2e/                      # End-to-end tests
│
└── configs/                      # Default configurations
    └── templates/                # Configuration templates
```

### 4.2 Component Diagram

```
┌─────────────────────────────────────────────────────────┐
│                     pkg/repository                       │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Client (interface)                               │   │
│  │  - Open(ctx, path) (*Repository, error)         │   │
│  │  - Clone(ctx, opts) (*Repository, error)        │   │
│  │  - GetStatus(ctx, repo) (*Status, error)        │   │
│  └──────────────────┬───────────────────────────────┘   │
│                     │ implements                         │
│  ┌──────────────────▼───────────────────────────────┐   │
│  │ client (struct)                                   │   │
│  │  - executor: *gitcmd.Executor                    │   │
│  │  - logger: Logger                                │   │
│  └───────────────────────────────────────────────────┘   │
└──────────────────────┬────────────────────────────────┘
                       │ uses
        ┌──────────────▼────────────────┐
        │   internal/gitcmd/Executor    │
        │  - Run(ctx, dir, args)        │
        └───────────────┬────────────────┘
                        │ executes
            ┌───────────▼───────────┐
            │   Git CLI (external)   │
            └────────────────────────┘
```

______________________________________________________________________

## 5. Interface Contracts

### 5.1 Core Repository Interface

```go
// pkg/repository/interfaces.go
package repository

import (
    "context"
    "io"
)

// Client defines core repository operations
type Client interface {
    // Repository lifecycle
    Open(ctx context.Context, path string) (*Repository, error)
    Clone(ctx context.Context, opts CloneOptions) (*Repository, error)
    IsRepository(ctx context.Context, path string) bool

    // Repository inspection
    GetInfo(ctx context.Context, repo *Repository) (*Info, error)
    GetStatus(ctx context.Context, repo *Repository) (*Status, error)

    // Basic operations (delegated to specific managers)
    // Use CommitManager, BranchManager, etc. for advanced features
}

// Repository represents a Git repository handle
type Repository struct {
    Path      string // Absolute path to repository
    GitDir    string // Path to .git directory
    WorkTree  string // Working tree path (may differ for worktrees)
    IsBare    bool   // True if bare repository
    IsShallow bool   // True if shallow clone
}

// Logger interface for dependency injection
// Library consumers provide their own logger implementation
type Logger interface {
    Debug(msg string, args ...interface{})
    Info(msg string, args ...interface{})
    Warn(msg string, args ...interface{})
    Error(msg string, args ...interface{})
}

// ProgressReporter for long-running operations
type ProgressReporter interface {
    Start(total int64)
    Update(current int64)
    Done()
}
```

### 5.2 Branch Manager Interface

```go
// pkg/branch/interfaces.go
package branch

import (
    "context"
    "github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// Manager handles branch and worktree operations
type Manager interface {
    // Branch operations
    List(ctx context.Context, repo *repository.Repository, opts ListOptions) ([]Branch, error)
    Create(ctx context.Context, repo *repository.Repository, name string, opts CreateOptions) (*Branch, error)
    Delete(ctx context.Context, repo *repository.Repository, name string, force bool) error
    Checkout(ctx context.Context, repo *repository.Repository, name string, opts CheckoutOptions) error

    // Worktree operations
    ListWorktrees(ctx context.Context, repo *repository.Repository) ([]Worktree, error)
    AddWorktree(ctx context.Context, repo *repository.Repository, path string, opts WorktreeOptions) (*Worktree, error)
    RemoveWorktree(ctx context.Context, repo *repository.Repository, path string) error

    // Parallel workflow support
    CreateParallelWorkflow(ctx context.Context, repo *repository.Repository, config WorkflowConfig) (*Workflow, error)
}

// Branch represents a Git branch
type Branch struct {
    Name       string
    Hash       string
    IsHead     bool   // Current branch
    IsRemote   bool
    Upstream   string // Upstream branch (if any)
    AheadBy    int    // Commits ahead of upstream
    BehindBy   int    // Commits behind upstream
}

// Worktree represents a Git worktree
type Worktree struct {
    Path       string
    Branch     string
    Hash       string
    IsLocked   bool
    Reason     string // Lock reason
}
```

### 5.3 History Analyzer Interface

```go
// pkg/history/interfaces.go
package history

import (
    "context"
    "time"
    "github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// Analyzer handles Git history analysis
type Analyzer interface {
    // Commit analysis
    GetCommits(ctx context.Context, repo *repository.Repository, opts QueryOptions) ([]Commit, error)
    GetCommitStats(ctx context.Context, repo *repository.Repository, opts StatsOptions) (*Statistics, error)

    // Contributor analysis
    GetContributors(ctx context.Context, repo *repository.Repository, opts ContributorOptions) ([]Contributor, error)
    GetContributorStats(ctx context.Context, repo *repository.Repository, email string) (*ContributorStats, error)

    // File history
    GetFileHistory(ctx context.Context, repo *repository.Repository, path string) ([]FileChange, error)
    GetDiff(ctx context.Context, repo *repository.Repository, opts DiffOptions) (*Diff, error)
}

// Commit represents a Git commit
type Commit struct {
    Hash      string
    Author    string
    Email     string
    Timestamp time.Time
    Message   string
    Files     []string
    Stats     CommitStats
}

// Statistics aggregates commit data
type Statistics struct {
    TotalCommits   int
    TotalAuthors   int
    CommitsByDay   map[string]int // Date -> count
    CommitsByAuthor map[string]int
    LinesAdded     int
    LinesRemoved   int
    TopFiles       []FileStats
}

// Contributor represents a repository contributor
type Contributor struct {
    Name        string
    Email       string
    Commits     int
    LinesAdded  int
    LinesRemoved int
    FirstCommit time.Time
    LastCommit  time.Time
}
```

### 5.4 Merge Manager Interface

```go
// pkg/merge/interfaces.go
package merge

import (
    "context"
    "github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// Manager handles merge and rebase operations
type Manager interface {
    // Merge/rebase operations
    Merge(ctx context.Context, repo *repository.Repository, branch string, opts MergeOptions) (*Result, error)
    Rebase(ctx context.Context, repo *repository.Repository, upstream string, opts RebaseOptions) (*Result, error)

    // Conflict handling
    DetectConflicts(ctx context.Context, repo *repository.Repository, source, target string) (*ConflictReport, error)
    ResolveConflict(ctx context.Context, repo *repository.Repository, file string, strategy ResolutionStrategy) error
    AutoResolve(ctx context.Context, repo *repository.Repository, policy AutoResolvePolicy) (*ResolutionResult, error)
}

// ResolutionStrategy defines conflict resolution method
type ResolutionStrategy string

const (
    StrategyOurs     ResolutionStrategy = "ours"     // Keep our version
    StrategyTheirs   ResolutionStrategy = "theirs"   // Take their version
    StrategyUnion    ResolutionStrategy = "union"    // Combine both
    StrategyPatience ResolutionStrategy = "patience" // Patience algorithm
)

// ConflictReport contains conflict analysis
type ConflictReport struct {
    HasConflicts   bool
    ConflictCount  int
    ConflictingFiles []ConflictFile
    Resolvable     bool // True if auto-resolvable
}

// ConflictFile represents a file with conflicts
type ConflictFile struct {
    Path           string
    ConflictType   ConflictType
    OurVersion     string
    TheirVersion   string
    BaseVersion    string
    Difficulty     DifficultyLevel
}
```

______________________________________________________________________

## 6. Data Flow

### 6.1 Repository Open Flow

```
Library Consumer (gzh-cli)
         │
         ▼
┌──────────────────────┐
│ pkg/repository       │
│ Client.Open(ctx,path)│
└───────────┬──────────┘
            │
            ├──▶ Validate path exists
            │    (stdlib + pkg/repository checks)
            │
            ├──▶ Check .git directory
            │    (filesystem check)
            │
            ├──▶ Execute: git rev-parse --git-dir
            │    (internal/gitcmd/Executor)
            │
            ├──▶ Parse Git output
            │    (internal/parser)
            │
            ▼
  ┌───────────────────┐
  │ Return Repository │
  │  - Path           │
  │  - GitDir         │
  │  - WorkTree       │
  │  - IsBare         │
  └───────────────────┘
```

### 6.2 Error Flow

```
Error Occurs in Git CLI
         │
         ▼
┌──────────────────────┐
│ internal/gitcmd      │  Capture stderr, exit code
│ Executor.Run()       │
└───────────┬──────────┘
            │
            ▼
┌──────────────────────┐
│ pkg/repository       │  Wrap error with context
│ Client method        │  return GitError{Op, Path, Err}
└───────────┬──────────┘
            │
            ▼
┌──────────────────────┐
│ cmd/gz-git/cmd       │  Cobra command layer
│ Command handler      │  Format user-friendly message
└───────────┬──────────┘
            │
            ▼
      Display to User
   "Failed to clone repository at /path:
    remote: repository not found

    Suggestions:
    - Check repository URL
    - Verify access permissions"
```

______________________________________________________________________

## 7. Error Handling Strategy

### 7.1 Error Types

```go
// internal/gitcmd/executor.go
type GitError struct {
    Command  string
    ExitCode int
    Stderr   string
    Cause    error
}

// pkg/repository/types.go
type ValidationError struct {
    Field  string
    Value  string
    Reason string
}

// Domain-specific "error-like" results are modeled as types, e.g.:
//   pkg/merge.ConflictReport
//   pkg/branch.CleanupReport
```

### 7.2 Error Handling Pattern

```go
// pkg/repository/client.go (simplified)
if opts.URL == "" {
    return nil, &ValidationError{
        Field:  "URL",
        Value:  opts.URL,
        Reason: "URL is required",
    }
}

// RunOutput/RunLines return *gitcmd.GitError on non-zero exit codes.
branch, err := c.executor.RunOutput(ctx, repo.Path, "rev-parse", "--abbrev-ref", "HEAD")
if err != nil {
    return nil, fmt.Errorf("failed to resolve current branch: %w", err)
}

_ = branch
```

### 7.3 Error Inspection Helpers

```go
// Example helper when you want to branch on git exit codes/output.
func IsNotRepository(err error) bool {
    var gitErr *gitcmd.GitError
    if errors.As(err, &gitErr) {
        return gitErr.ExitCode == 128 &&
            strings.Contains(gitErr.Stderr, "not a git repository")
    }
    return false
}
```

______________________________________________________________________

## 8. Testing Architecture

### 8.1 Test Pyramid

```
         ┌─────────┐
         │   E2E   │  10% - Full workflows, real Git repos
         │  Tests  │
         └─────────┘
       ┌───────────────┐
       │  Integration  │  30% - Real Git, test repos
       │     Tests     │
       └───────────────┘
     ┌───────────────────┐
     │    Unit Tests     │  60% - Mocked dependencies
     │                   │
     └───────────────────┘
```

### 8.2 Unit Testing Strategy

**Packages**: `pkg/*`, `internal/*`

```go
// Example: pkg/repository/bulk_commit_test.go
func TestBulkCommit(t *testing.T) {
    ctx := context.Background()
    client := repository.NewClient()

    result, err := client.BulkCommit(ctx, repository.BulkCommitOptions{
        Directory: t.TempDir(),
        DryRun:    true,
        Logger:    repository.NewNoopLogger(),
    })
    if err != nil {
        t.Fatalf("BulkCommit failed: %v", err)
    }

    _ = result
}
```

### 8.3 Integration Testing Strategy

**Package**: `tests/integration/`

```go
// tests/integration/helper_test.go (snippet)
repo := NewTestRepo(t)
repo.SetupWithCommits()

output := repo.RunGzhGitSuccess("status")
AssertContains(t, output, "Bulk Status Results")
```

### 8.4 E2E Testing Strategy

**Package**: `tests/e2e/`

```go
// tests/e2e/basic_workflow_test.go
// +build e2e

package e2e

import (
    "os/exec"
    "testing"
)

func TestWorkflow_CommitToPush(t *testing.T) {
    // Setup: Create repo, make changes
    repoPath := setupTestRepo(t)
    defer cleanup(t, repoPath)

    // Execute CLI commands
    steps := []struct {
        cmd  string
        args []string
    }{
        {"gz-git", []string{"commit", "--yes"}},
        {"gz-git", []string{"push", "--dry-run"}},
    }

    for _, step := range steps {
        cmd := exec.Command(step.cmd, step.args...)
        cmd.Dir = repoPath

        output, err := cmd.CombinedOutput()
        if err != nil {
            t.Fatalf("command failed: %s\n%s", step.cmd, output)
        }

        t.Logf("%s output:\n%s", step.cmd, output)
    }

    // Verify: Check Git log
    verifyCommitExists(t, repoPath)
}
```

______________________________________________________________________

## 9. Performance Considerations

### 9.1 Performance Requirements

| Operation                      | Target (p95) | Strategy                        |
| ------------------------------ | ------------ | ------------------------------- |
| `status`                       | \<50ms       | Cached repository state         |
| `commit`                       | \<100ms      | Minimal validation              |
| `switch`                       | \<100ms      | Skip dirty/in-progress repos    |
| Bulk update (100 repos)        | \<30s        | Parallel execution (goroutines) |
| History analysis (10K commits) | \<5s         | Streaming, pagination           |

### 9.2 Optimization Strategies

**Parallel Execution:**

```go
// pkg/repository/bulk_commit.go (simplified)
semaphore := make(chan struct{}, common.Parallel)
for _, repoPath := range filteredRepos {
    wg.Add(1)
    go func(path string) {
        defer wg.Done()
        semaphore <- struct{}{}
        defer func() { <-semaphore }()

        repoResult := c.analyzeRepositoryForCommit(ctx, common.Directory, path, opts)
        _ = repoResult
    }(repoPath)
}
```

**Avoiding unnecessary work:**

- Bulk operations short-circuit early (e.g., skip repos without remotes/upstreams, skip dirty repos when unsafe).
- Watch mode re-runs the same bulk operation at a configurable interval (`--watch`, `--interval`).

**Streaming for Large Results:**

```go
// pkg/history/analyzer.go
func (a *analyzer) GetCommits(ctx context.Context, repo *Repository, opts QueryOptions) (<-chan Commit, error) {
    commitChan := make(chan Commit, 100)

    go func() {
        defer close(commitChan)

        // Stream commits incrementally
        cmd := exec.CommandContext(ctx, "git", "log", "--format=%H|%an|%ae|%at|%s")
        stdout, _ := cmd.StdoutPipe()
        cmd.Start()

        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            commit := parseCommitLine(scanner.Text())
            select {
            case commitChan <- commit:
            case <-ctx.Done():
                return
            }
        }
    }()

    return commitChan, nil
}
```

______________________________________________________________________

## 10. Security Architecture

### 10.1 Security Principles

1. **Input Validation**: Sanitize all user inputs before passing to Git CLI
1. **Path Validation**: Ensure paths stay within repository boundaries
1. **Command Injection Prevention**: No direct string interpolation in commands
1. **Credential Safety**: Never log or expose credentials
1. **Least Privilege**: Run with minimal necessary permissions

### 10.2 Input Sanitization

```go
// internal/gitcmd/sanitize.go
package gitcmd

import (
    "regexp"
    "strings"
)

var (
    // Dangerous patterns that could enable command injection
    dangerousPatterns = []*regexp.Regexp{
        regexp.MustCompile(`[;&|]`),     // Command separators
        regexp.MustCompile(`\$\(`),      // Command substitution
        regexp.MustCompile("`"),         // Backticks
        regexp.MustCompile(`\.\./`),     // Path traversal
    }
)

// SanitizeArgs validates and sanitizes Git command arguments
func SanitizeArgs(args []string) ([]string, error) {
    sanitized := make([]string, 0, len(args))

    for _, arg := range args {
        // Check for dangerous patterns
        for _, pattern := range dangerousPatterns {
            if pattern.MatchString(arg) {
                return nil, fmt.Errorf("potentially dangerous argument: %s", arg)
            }
        }

        // Additional validation for specific arg types
        if strings.HasPrefix(arg, "-") {
            // Flag validation
            if !isValidGitFlag(arg) {
                return nil, fmt.Errorf("invalid Git flag: %s", arg)
            }
        }

        sanitized = append(sanitized, arg)
    }

    return sanitized, nil
}

func isValidGitFlag(flag string) bool {
    // Whitelist of known safe Git flags
    safeFlags := []string{
        "--all", "--amend", "--no-verify", "--dry-run",
        "--force", "--quiet", "--verbose",
        // ... more safe flags
    }

    for _, safe := range safeFlags {
        if flag == safe || strings.HasPrefix(flag, safe+"=") {
            return true
        }
    }

    return false
}
```

### 10.3 Path Validation

- Bulk directory arguments are validated via `os.Stat` before scanning (`cmd/gz-git/cmd/bulk_common.go`).
- Repository operations verify `.git` presence via `internal/gitcmd.Executor.IsGitRepository` (`internal/gitcmd/executor.go`).

______________________________________________________________________

## 11. Deployment Architecture

### 11.1 Build Pipeline

```
┌───────────────┐
│ Source Code   │
│ (main branch) │
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ GitHub Actions│
│ CI/CD         │
└───────┬───────┘
        │
        ├───▶ golangci-lint
        ├───▶ go test (unit, integration)
        ├───▶ go test -race (race detection)
        ├───▶ gosec (security scan)
        │
        ▼
┌───────────────┐
│ Build Binary  │
│ (multi-platform)
└───────┬───────┘
        │
        ├───▶ linux/amd64
        ├───▶ linux/arm64
        ├───▶ darwin/amd64
        ├───▶ darwin/arm64
        ├───▶ windows/amd64
        │
        ▼
┌───────────────┐
│ Create Release│
│ (GitHub)      │
└───────┬───────┘
        │
        ├───▶ Upload binaries
        ├───▶ Generate checksums
        ├───▶ Sign with GPG
        │
        ▼
┌───────────────┐
│ Publish       │
│ - Go module   │
│ - Homebrew    │
│ - APT/YUM     │
└───────────────┘
```

### 11.2 Deployment Targets

**Go Module (Primary):**

```bash
go get github.com/gizzahub/gzh-cli-gitforge@latest
```

**Homebrew (macOS/Linux):**

```bash
brew install gz-git
```

**Direct Download:**

```bash
curl -sL https://github.com/gizzahub/gzh-cli-gitforge/releases/latest/download/gz-git-linux-amd64 -o gz-git
chmod +x gz-git
```

______________________________________________________________________

## 12. Design Decisions

### 12.1 Decision Log

#### D1: Git CLI vs. go-git Library

**Decision**: Use Git CLI
**Rationale**:

- Maximum compatibility with all Git features
- Simpler implementation (no need to reimplement Git logic)
- Users already have Git installed
- Easier to debug (same commands users run manually)

**Trade-offs**:

- External dependency on Git binary
- Slower than pure Go (process spawning overhead)
- Parsing text output vs. structured API

**Alternatives Considered**:

- go-git/v5: Pure Go, no external deps, but incomplete feature set
- Hybrid: Use go-git for simple ops, Git CLI for complex (too complex)

#### D2: Library-First Architecture

**Decision**: Design library (`pkg/`) with zero CLI dependencies
**Rationale**:

- Enables reuse in gzh-cli and other projects
- Better API design (forced to think about interfaces)
- Easier testing (no CLI framework mocks)
- Clear separation of concerns

**Trade-offs**:

- More upfront design effort
- Indirection layer between CLI and logic
- Some code duplication (CLI and library versions)

**Alternatives Considered**:

- CLI-first, extract library later (risky, usually doesn't happen)
- Monolithic design (violates single responsibility)

#### D3: Functional Options Pattern

**Decision**: Use functional options for all complex operations
**Rationale**:

- API extensibility without breaking changes
- Sensible defaults
- Self-documenting (option names are clear)
- Idiomatic Go pattern

**Trade-offs**:

- More verbose (but clearer)
- Slightly more allocations (usually negligible)

**Example**:

```go
// Instead of:
Clone(ctx, url, path, branch, depth, progress, recursive)

// Use:
Clone(ctx, url, path,
    WithBranch("main"),
    WithDepth(1),
    WithProgress(os.Stdout),
)
```

#### D4: Context Propagation

**Decision**: All operations accept `context.Context` as first parameter
**Rationale**:

- Cancellation support (user can Ctrl+C)
- Timeout support (prevent infinite hangs)
- Request-scoped values (trace IDs, etc.)
- Idiomatic Go concurrency pattern

**Trade-offs**:

- Every function signature includes ctx
- Must remember to pass context through

#### D5: Interface-Driven Design

**Decision**: Define interfaces for all major components
**Rationale**:

- Testability (easy to mock)
- Extensibility (consumers can provide implementations)
- Decoupling (depend on interfaces, not concretions)

**Trade-offs**:

- More files (interface + implementation)
- Indirection (but worth it for benefits)

______________________________________________________________________

## 13. Future Considerations

### 13.1 Potential Enhancements (v2.0+)

**Plugin Architecture:**

- Allow custom commit templates from plugins
- Custom conflict resolution strategies
- Extensible history analyzers

**Performance:**

- libgit2 integration for performance-critical paths
- Persistent cache (disk-based)
- Incremental updates

**Features:**

- Git hooks automation
- Submodule management
- Advanced visualizations (TUI)
- Team collaboration features (code review integration)

### 13.2 Scalability

**Large Repositories (100K+ commits):**

- Streaming APIs (don't load all commits into memory)
- Pagination for queries
- Parallel processing for bulk operations

**High Concurrency:**

- Connection pooling for Git operations
- Rate limiting for external APIs (GitHub, GitLab)
- Circuit breakers for error handling

______________________________________________________________________

## Appendix

### A.1 Key Files Summary

| File                           | Purpose             | Criticality |
| ------------------------------ | ------------------- | ----------- |
| `pkg/repository/interfaces.go` | Core repository API | CRITICAL    |
| `pkg/branch/manager.go`        | Branch management   | HIGH        |
| `internal/gitcmd/executor.go`  | Git command wrapper | CRITICAL    |
| `cmd/gz-git/main.go`           | CLI entry point     | MEDIUM      |

### A.2 Dependencies

```go
// go.mod
module github.com/gizzahub/gzh-cli-gitforge

go 1.25.1

require (
    github.com/fsnotify/fsnotify v1.9.0
    github.com/gizzahub/gzh-cli-core v0.0.0-20251230045225-725b628c716a
    github.com/google/go-github/v66 v66.0.0
    github.com/spf13/cobra v1.10.2
    github.com/xanzy/go-gitlab v0.115.0
    golang.org/x/oauth2 v0.34.0
    golang.org/x/sync v0.19.0
    gopkg.in/yaml.v3 v3.0.1
)
```

### A.3 References

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [gzh-cli Architecture](https://github.com/gizzahub/gzh-cli/blob/main/ARCHITECTURE.md)

______________________________________________________________________

## Revision History

| Version | Date       | Author      | Changes                     |
| ------- | ---------- | ----------- | --------------------------- |
| 1.0     | 2025-11-27 | Claude (AI) | Initial architecture design |

______________________________________________________________________

**End of Document**
