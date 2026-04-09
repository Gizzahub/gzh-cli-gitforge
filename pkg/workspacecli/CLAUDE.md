# pkg/workspacecli — CLAUDE.md

CLI commands for local workspace management (config-based repo sync).

---

## Purpose

Implements all `gz-git workspace` subcommands using local `.gz-git.yaml` config.
**Does NOT call Forge APIs** — use `reposynccli` for that.

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
  → config_loader.go   (load .gz-git.yaml)
  → reposync.Planner   (build action plan)
  → sync_preview       (show diff preview to user)
  → reposync.Executor  (execute: clone / pull / skip)
  → sync_progress_tui  (TUI progress display)
```

---

## Factory Pattern (`factory.go`)

Commands share dependencies via `Factory`:
```go
type Factory struct {
    ConfigLoader ConfigLoader
    PlannerFn    func(cfg *config.Config) reposync.Planner
    ExecutorFn   func() reposync.Executor
}
```
Inject mocks in tests via `factory_test.go`.

---

## Sync Preview (`sync_tree_output.go`)

`workspace sync` shows a detailed preview before executing:
- Repository summary (clone/update/skip counts)
- File-level changes (added/modified/deleted)
- Conflict warnings (dirty worktree, diverged branches)
- Interactive confirmation

---

## DO / DON'T

- **DO** use `Factory` for dependency injection — not direct instantiation
- **DO** show preview + confirm before destructive sync operations
- **DON'T** call Forge APIs here — delegate to `reposynccli`
- **DON'T** run hook commands via shell — use `exec.Command(args[0], args[1:]...)`

---

## Testing

```go
// factory_test.go pattern
f := workspacecli.NewFactory(
    workspacecli.WithConfigLoader(mockLoader),
    workspacecli.WithPlannerFn(func(cfg) reposync.Planner { return mockPlanner }),
)
cmd := workspacecli.NewSyncCommand(f)
```
