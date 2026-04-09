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

### Safe flags whitelist
Only whitelisted flags in `safeGitFlags` map are allowed. Add new flags there explicitly.

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
