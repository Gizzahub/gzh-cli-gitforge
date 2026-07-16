# 8. Testing Architecture

> gzh-cli-gitforge 아키텍처 문서 · [인덱스](README.md) · [ARCHITECTURE.md](../../ARCHITECTURE.md)

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
