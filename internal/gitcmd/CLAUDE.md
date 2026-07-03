# internal/gitcmd — CLAUDE.md

Safe git command execution. **Security-critical package.**

---

## Purpose

Wraps `os/exec` for git commands with:
- Input sanitization (prevent command injection)
- Timeout support
- Structured `Result` output

Every git operation in this project goes through this package.

---

## Key Types

| Type | Purpose |
|------|---------|
| `Executor` | Main entry point — create with `NewExecutor()` |
| `Result` | Stdout, Stderr, ExitCode, Duration, Error |

---

## Security Rules (CRITICAL)

```go
// SAFE — args passed separately, no shell
cmd := exec.Command("git", "clone", url)

// DANGEROUS — never do this
cmd := exec.Command("sh", "-c", "git clone " + url)
```

**Sanitize before use**:
```go
if err := gitcmd.SanitizeInput(userInput); err != nil {
    return err // reject dangerous chars: ; | & > < $ ` ../ \x00
}
```

### Blocked patterns (`sanitize.go`)
- Command separators: `; | & > < $`
- Command substitution: `$(...)` `` ` ``
- Path traversal: `../`
- System dirs: `/etc/ /usr/ /bin/ /sbin/`
- Null bytes, newlines

### No flag allowlist (by design)
Git runs via `exec.CommandContext` **without a shell**, so a flag allowlist gives no
shell-injection defense — it only breaks legitimate flags. The residual threat is
**option injection** (a user value parsed by git as a flag, e.g. a branch named
`--upload-pack=…`). Defend it at the value layer, not with a flag list:
- `--` end-of-options separator before user-supplied positional args
- per-value validators: `SanitizeBranchName` (rejects leading `-`), `SanitizePath`, `SanitizeURL`, `SanitizeCommitMessage`

---

## Usage

```go
exec := gitcmd.NewExecutor(
    gitcmd.WithTimeout(30 * time.Second),
    gitcmd.WithEnv([]string{"GIT_TERMINAL_PROMPT=0"}),
)

result, err := exec.Run(ctx, "/repo/path", "fetch", "--all", "--prune")
if err != nil || result.ExitCode != 0 {
    return fmt.Errorf("fetch failed: %s", result.Stderr)
}
```

---

## DO / DON'T

- **DO** always pass args as separate strings to `Run()`
- **DO** call `SanitizeInput()` on any user-supplied URLs, branch names, paths
- **DON'T** use `sh -c` or string concatenation
- **DON'T** log `result.Stderr` if it might contain auth tokens (strip first)
- **DON'T** expose this package's `Executor` to callers above `pkg/` layer

---

## Testing

```go
// executor_test.go pattern: use real git binary, temp dirs
repo := testutil.TempGitRepoWithCommit(t)
e := gitcmd.NewExecutor()
result, err := e.Run(ctx, repo, "status", "--porcelain")
```
