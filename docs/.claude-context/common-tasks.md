# Common Tasks - gzh-cli-gitforge

## Core Design: Bulk-First Defaults

gz-git은 **기본적으로 bulk 모드**로 동작합니다. 모든 주요 명령어는 디렉토리를 스캔하여 여러 repository를 병렬 처리합니다.

### 기본값 (pkg/repository/types.go)

```go
const (
    DefaultBulkMaxDepth = 1    // 현재 디렉토리 + 1레벨
    DefaultBulkParallel = 10    // 10개 병렬 처리
)
```

### 스캔 깊이 설명

```
depth=0: 현재 디렉토리만 (단일 repo 동작)
depth=1: 현재 + 1레벨 (기본값)
depth=2: 현재 + 2레벨
```

### 단일 repo 작업

경로를 직접 지정하면 해당 repo만 처리:

```bash
gz-git status /path/to/repo
gz-git fetch /path/to/repo
```

### bulk 명령어 플래그 등록 (cmd/gz-git/cmd/bulk_common.go)

```go
func addBulkFlags(cmd *cobra.Command, flags *BulkFlags) {
    cmd.Flags().IntVarP(&flags.Depth, "scan-depth", "d",
        repository.DefaultBulkMaxDepth, "directory depth to scan")
    cmd.Flags().IntVarP(&flags.Parallel, "parallel", "j",
        repository.DefaultBulkParallel, "parallel workers")
    cmd.Flags().BoolVarP(&flags.DryRun, "dry-run", "n",
        false, "preview without executing")
    cmd.Flags().StringVar(&flags.Include, "include", "", "include pattern")
    cmd.Flags().StringVar(&flags.Exclude, "exclude", "", "exclude pattern")
}
```

______________________________________________________________________

## Adding New Git Commands

### Where to add

`cmd/gzh-git/` - create new command file

### Example

```go
// cmd/gzh-git/status.go
var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show working tree status",
    RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
    // Implementation
    return nil
}

func init() {
    rootCmd.AddCommand(statusCmd)
}
```

## Adding Git Execution Logic

### Where to add

`internal/gitcmd/` - safe command execution

### Example

```go
// internal/gitcmd/executor.go
func (e *Executor) Run(ctx context.Context, args ...string) error {
    // Sanitize inputs
    sanitized := sanitize(args)

    cmd := exec.CommandContext(ctx, "git", sanitized...)
    return cmd.Run()
}
```

## Adding Output Parsing

### Where to add

`internal/parser/` - git output parsing

### Example

```go
// internal/parser/status.go
func ParseStatus(output string) (*Status, error) {
    lines := strings.Split(output, "\n")
    // Parse git status output
    return &Status{}, nil
}
```

## Adding Public APIs

### Where to add

`pkg/{feature}/` directory

### Example

```go
// pkg/branch/branch.go
package branch

type Manager struct {
    executor gitcmd.Executor
}

func (m *Manager) List(ctx context.Context) ([]string, error) {
    // Implementation
    return nil, nil
}
```

## Input Sanitization (Critical for Security)

### Always sanitize user inputs

```go
func sanitize(args []string) []string {
    var sanitized []string
    for _, arg := range args {
        // Remove dangerous characters
        if !containsDangerousChars(arg) {
            sanitized = append(sanitized, arg)
        }
    }
    return sanitized
}

func containsDangerousChars(s string) bool {
    dangerous := []string{";", "|", "&", "$", "`"}
    for _, d := range dangerous {
        if strings.Contains(s, d) {
            return true
        }
    }
    return false
}
```

## Testing Git Operations

### Use git-specific test helpers

```go
import "github.com/gizzahub/gzh-cli-gitforge/internal/testutil"

func TestGitOperation(t *testing.T) {
    // Create temp git repo
    repo := testutil.TempGitRepo(t)

    // Create repo with commit
    repoWithCommit := testutil.TempGitRepoWithCommit(t)

    // Test operations
}
```

## Working with Branches

```go
import "github.com/gizzahub/gzh-cli-gitforge/pkg/branch"

manager := branch.NewManager(executor)

// List branches
branches, err := manager.List(ctx)

// Create branch
err = manager.Create(ctx, "feature-branch")

// Switch branch
err = manager.Switch(ctx, "main")
```

## Working with Commits

```go
import "github.com/gizzahub/gzh-cli-gitforge/pkg/commit"

handler := commit.NewHandler(executor)

// Create commit
err = handler.Commit(ctx, "commit message")

// Amend commit
err = handler.Amend(ctx, "new message")
```

## Error Handling Best Practices

```go
func runGitCommand(ctx context.Context, args ...string) error {
    if err := executor.Run(ctx, args...); err != nil {
        // Wrap with context
        return fmt.Errorf("git %s failed: %w",
            strings.Join(args, " "), err)
    }
    return nil
}
```

______________________________________________________________________

## Repository Health Diagnostics (sync status)

### Overview

`gz-git sync status` provides comprehensive health checks for multiple repositories:

- Fetches from all remotes (with timeout)
- Detects network connectivity issues
- Compares local vs remote branches
- Identifies potential conflicts
- Provides actionable recommendations

### Architecture

```
DiagnosticExecutor (pkg/reposync/diagnostic_executor.go)
    ├── CheckHealth() - Main entry point
    ├── checkOne() - Per-repository health check
    ├── fetchWithTimeout() - Remote fetch with timeout
    ├── classifyDivergence() - Analyze local vs remote
    ├── classifyHealth() - Determine overall health
    └── generateRecommendation() - Create actionable guidance
```

### Type Hierarchy

```go
// Health classification
type HealthStatus string
const (
    HealthHealthy      // up-to-date, clean
    HealthWarning      // diverged, can be resolved
    HealthError        // conflicts, dirty + behind
    HealthUnreachable  // network timeout, auth failed
)

// Divergence analysis
type DivergenceType string
const (
    DivergenceNone         // local == remote
    DivergenceFastForward  // can fast-forward pull
    DivergenceDiverged     // merge/rebase needed
    DivergenceAhead        // can push
    DivergenceConflict     // merge conflicts exist
    DivergenceNoUpstream   // no upstream configured
)

// Network status
type NetworkStatus string
const (
    NetworkOK           // fetch succeeded
    NetworkTimeout      // fetch timed out
    NetworkUnreachable  // DNS/connection failed
    NetworkAuthFailed   // authentication failed
)
```

### Adding New Health Checks

```go
// pkg/reposync/diagnostic_executor.go
func (e DiagnosticExecutor) checkOne(ctx context.Context, ...) RepoHealth {
    // ... existing checks ...

    // Add custom health check
    if opts.CheckCustom {
        customStatus := e.checkCustomCondition(ctx, r)
        health.CustomStatus = customStatus
    }

    return health
}
```

### Customizing Recommendations

```go
// pkg/reposync/diagnostic_executor.go
func generateRecommendation(health RepoHealth) string {
    switch health.HealthStatus {
    case HealthWarning:
        if health.CustomCondition {
            return "Custom recommendation for your case"
        }
    }
    // ... existing logic ...
}
```

### Testing Health Checks

```go
// Integration test example
func TestCustomHealthCheck(t *testing.T) {
    repo := testutil.TempGitRepoWithCommit(t)
    defer os.RemoveAll(filepath.Dir(repo))

    executor := DiagnosticExecutor{
        Client: repo.NewClient(),
    }

    opts := DiagnosticOptions{
        SkipFetch: true, // Fast test without network
        CheckWorkTree: true,
    }

    report, err := executor.CheckHealth(ctx, repos, opts)
    // Assert on report.Results
}
```

### CLI Integration

```go
// pkg/reposynccli/status_command.go
func (f CommandFactory) runStatus(cmd *cobra.Command, opts *StatusOptions) error {
    // Load repositories from config or scan directory
    repos := loadRepos(opts)

    // Execute health check
    executor := reposync.DiagnosticExecutor{}
    report, err := executor.CheckHealth(ctx, repos, diagOpts)

    // Display results
    f.printHealthReport(cmd, report, opts.Verbose)
}
```

### Performance Considerations

- **Parallel execution**: Default 4 workers, configurable with `--parallel`
- **Timeout handling**: Default 30s per fetch, configurable with `--timeout`
- **Skip fetch**: Use `--skip-fetch` for fast checks (may show stale data)
- **Network classification**: Errors are parsed from git stderr for specific guidance
