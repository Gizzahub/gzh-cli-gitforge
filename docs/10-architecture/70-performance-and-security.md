# 9-10. Performance and Security

> gzh-cli-gitforge 아키텍처 문서 · [인덱스](README.md) · [ARCHITECTURE.md](../../ARCHITECTURE.md)

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

        sanitized = append(sanitized, strings.TrimSpace(arg))
    }

    return sanitized, nil
}
```

> **No flag allowlist by design.** Git is executed via `exec.CommandContext`
> without a shell, so a flag allowlist provides no shell-injection defense and only
> breaks legitimate flags. The residual threat is *option injection* (a user value
> parsed by git as a flag), handled by the `--` end-of-options separator and the
> per-value validators (`SanitizeBranchName`, `SanitizePath`, `SanitizeURL`,
> `SanitizeCommitMessage`) at the call sites.

### 10.3 Path Validation

- Bulk directory arguments are validated via `os.Stat` before scanning (`cmd/gz-git/cmd/bulk_common.go`).
- Repository operations verify `.git` presence via `internal/gitcmd.Executor.IsGitRepository` (`internal/gitcmd/executor.go`).
