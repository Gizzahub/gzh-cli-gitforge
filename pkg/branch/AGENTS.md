# pkg/branch — CLAUDE.md

Git branch management: listing, switching, cleanup, worktree ops.

---

## Purpose

Provides `Service` for branch operations on a single repository.
Handles parallel bulk cleanup via `Manager` across multiple repos.

---

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `Service` | `manager.go` | Single-repo branch operations |
| `Manager` | `manager.go` | Bulk operations across repos |
| `BranchInfo` | `types.go` | Name, remote, merged, gone |
| `CleanupOptions` | `cleanup.go` | Filters for cleanup |
| `CleanupResult` | `cleanup.go` | Per-branch cleanup outcome |
| `Worktree` | `worktree.go` | Worktree add/list/remove |

---

## Service Usage

```go
svc := branch.NewService(repoPath)

// List branches
branches, err := svc.List(branch.ListOptions{Remote: true, Local: true})

// Cleanup merged/stale/gone branches
result, err := svc.Cleanup(branch.CleanupOptions{
    DryRun:          true,
    RemoveMerged:    true,
    RemoveGone:      true,
    ProtectedBranches: []string{"main", "master", "release/*"},
})
```

---

## Cleanup Categories

| Category | Meaning | Flag |
|----------|---------|------|
| **merged** | Merged into current branch | `RemoveMerged` |
| **stale** | No commits for N days | `RemoveStale` + `StaleDays` |
| **gone** | Remote tracking branch deleted | `RemoveGone` |

---

## Parallel Operations (`parallel.go`)

```go
mgr := branch.NewManager(repoPaths, branch.ManagerOptions{Parallel: 10})
results, err := mgr.CleanupAll(ctx, cleanupOpts)
```

---

## Worktree (`worktree.go`)

```go
svc := branch.NewService(repoPath)
err := svc.AddWorktree(ctx, branch.WorktreeOptions{
    Path:   "/tmp/feature-worktree",
    Branch: "feature/new-ui",
})
```

---

## DO / DON'T

- **DO** always include protected branches in cleanup config
- **DO** default `DryRun: true` in interactive commands
- **DON'T** force-delete branches without explicit `--force` flag from user
- **DON'T** remove worktrees without checking for uncommitted changes

---

## Testing

```go
repo := testutil.TempGitRepoWithCommit(t)
svc := branch.NewService(repo)
// create test branches, then test cleanup
```
