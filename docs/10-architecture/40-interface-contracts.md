# 5. Interface Contracts

> gzh-cli-gitforge 아키텍처 문서 · [인덱스](README.md) · [ARCHITECTURE.md](../../ARCHITECTURE.md)

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
