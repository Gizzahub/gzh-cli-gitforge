# pkg/workspacecli — CLAUDE.md

CLI commands for local workspace management (config-based repo sync).

---

## Purpose

Implements all `gz-git workspace` subcommands using local `.gz-git.yaml` config.
**Does NOT call Forge APIs** — use `reposynccli` for that.

This is the **declarative workspace engine** (config-driven convergence).
Contrast with scan-based bulk commands (`fetch`/`pull`/`status`, …) which are
**ad-hoc scan operations** over a directory tree.

---

## Commands

| File | Command | Description |
|------|---------|-------------|
| `init_command.go` | `workspace init` | Scan dir → generate config |
| `sync_command.go` | `workspace sync` | Clone/update from config |
| `quicksync_command.go` | `sync` (alias) | Auto-init + sync (one shot) |
| `status_command.go` | `workspace status` | Health check |
| `add_command.go` | `workspace add` | Add repo to config |
| `validate_command.go` | `workspace validate` | Validate config file |
| `generate_command.go` | `workspace generate-config` | Generate config from forge |

---

## Key Flow: `workspace sync`

```
sync_command.go
  → config load (.gz-git.yaml)
  → reposync.Planner   (build action plan)
  → sync_preview       (show diff preview to user)
  → reposync.Executor  (execute: clone / pull / skip)
  → sync_progress_tui  (TUI progress display)
```

Workspace recursion (child configs): `--recurse-workspaces`
(deprecated alias: `--recursive` / `-R`).

---

## Factory Pattern (`factory.go`)

Commands are methods on `CommandFactory` (empty struct; no DI options today):

```go
type CommandFactory struct{}

f := workspacecli.CommandFactory{}
root := f.NewRootCmd() // workspace root + subcommands
```

Inject mocks in tests by constructing `CommandFactory{}` and exercising
command constructors (`newSyncCmd`, `newInitCmd`, …) / `NewRootCmd`.

---

## Sync Preview (`sync_tree_output.go`)

`workspace sync` shows a detailed preview before executing:
- Repository summary (clone/update/skip counts)
- File-level changes (added/modified/deleted)
- Conflict warnings (dirty worktree, diverged branches)
- Interactive confirmation (`--interactive`)

---

## DO / DON'T

- **DO** build commands via `CommandFactory` — not ad-hoc package-level constructors
- **DO** show preview + confirm before destructive sync when interactive
- **DON'T** call Forge APIs here — delegate to `reposynccli`
- **DON'T** run hook commands via shell — use `exec.Command(args[0], args[1:]...)`

---

## Testing

```go
f := workspacecli.CommandFactory{}
cmd := f.NewRootCmd()
// or: f.newSyncCmd() for focused tests
```
