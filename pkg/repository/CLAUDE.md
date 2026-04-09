# pkg/repository — CLAUDE.md

Core git repository abstraction and bulk operations.

---

## Purpose

Provides `Client` interface for all git operations (clone, fetch, pull, push, status).
**Bulk-first**: all operations scan a directory and process multiple repos in parallel.

---

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `Client` | `client.go` | Main interface — use `NewClient()` |
| `Repository` | (embedded) | Single repo state |
| `BulkXxxOptions` | `bulk.go` | Per-operation bulk config |
| `BulkXxxResult` | `bulk.go` | Aggregated results |
| `RepositoryXxxResult` | `bulk.go` | Per-repo result (implements `GetStatus()`) |

---

## Defaults

```go
// pkg/repository/defaults.go
DefaultLocalScanDepth = 1    // current dir + 1 level
DefaultLocalParallel  = 10   // 10 concurrent workers
```

---

## Usage Pattern

```go
c := repository.NewClient(repository.WithClientLogger(logger))

// Bulk operation
result, err := c.BulkFetch(ctx, repository.BulkFetchOptions{
    Directory: "/workspace",
    Parallel:  10,
    MaxDepth:  1,
})
for _, r := range result.Results {
    fmt.Println(r.Path, r.GetStatus())
}
```

---

## DO / DON'T

- **DO** use `NewClient()` — never instantiate `client{}` directly
- **DO** use `BulkFlagOptions` for shared flag registration in CLI
- **DON'T** call `internal/gitcmd` directly from callers — go through `Client`
- **DON'T** add `sync.Mutex` inside bulk loops (already handled by `errgroup`)

---

## Auth Error Handling

Bulk ops detect auth failures via `isAuthenticationError(stderr)` and return `ErrAuthRequired` instead of blocking on credential prompts (`GIT_TERMINAL_PROMPT=0` is set).

---

## Adding a New Bulk Operation

1. Add `BulkXxxOptions` + `BulkXxxResult` + `RepositoryXxxResult` structs to `bulk.go`
2. Add `BulkXxx(ctx, BulkXxxOptions)` to `Client` interface in `client.go`
3. Implement in a new `bulk_xxx.go` file using the `errgroup` pattern
4. Register flags with `addBulkFlagsWithOpts()` in the CLI command

---

## Testing

```go
// Use mock executor for unit tests
exec := gitcmd.NewExecutor(gitcmd.WithGitBinary("/usr/bin/git"))
c := repository.NewClient(repository.WithExecutor(exec))

// Use testutil for real git repos
repo := testutil.TempGitRepoWithCommit(t)
```
