# Common Tasks - gzh-cli-gitforge

## Core Design: Bulk-First Defaults

gz-git은 **기본적으로 bulk 모드**로 동작합니다. 모든 주요 명령어는 디렉토리를 스캔하여 여러 repository를 병렬 처리합니다.

### 기본값 (pkg/repository/types.go)

```go
const (
    DefaultBulkMaxDepth = 1    // 현재 디렉토리 + 1레벨
    DefaultBulkParallel = 5    // 5개 병렬 처리
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
