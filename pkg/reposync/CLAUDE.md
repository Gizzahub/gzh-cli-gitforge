# pkg/reposync — CLAUDE.md

Sync planner and executor for repository orchestration.

---

## Purpose

Converts a desired repository list into an action plan, then executes it.
Used by both `workspacecli` (local config) and `reposynccli` (forge API).

---

## Core Interfaces

```go
// plan.go
type Planner interface {
    Plan(ctx context.Context, req PlanRequest) (Plan, error)
}

// executor.go
type Executor interface {
    Execute(ctx context.Context, plan Plan, opts ExecOptions) error
}
```

---

## Planners

| File | Type | Input |
|------|------|-------|
| `planner_static.go` | `StaticPlanner` | Local config (`[]RepoSpec`) |
| `planner_forge.go` | `ForgePlanner` | Forge API (GitHub/GitLab/Gitea) |

Auto-detects action per repo:
- **clone** — target path does not exist
- **pull/fetch** — repo exists, up-to-date
- **update** — repo exists, behind remote

---

## Key Types

| Type | Purpose |
|------|---------|
| `PlanRequest` | `PlanInput` + `PlanOptions` |
| `RepoSpec` | URL, target path, branch, auth, strategy |
| `Plan` | `[]Action` — ordered list |
| `Action` | type (clone/pull/fetch/skip), spec, strategy |
| `Strategy` | clone strategy: `mirror`, `standard`, `shallow` |
| `AuthConfig` | SSH key path or token |

---

## Execution Flow

```
Plan → executor.go
  for each action (parallel):
    executor_git.go   → real git clone/pull/fetch
    executor_noop.go  → dry-run (logs only)
  progress.go         → progress reporting callback
  state_file.go       → persist last-sync state
```

---

## Diagnostics (`diagnostic.go`)

```go
diag := reposync.NewDiagnosticExecutor(baseExecutor)
report := diag.Report()  // per-repo health: healthy/warning/error/unreachable
```

Health symbols: `✓` healthy · `⚠` warning · `✗` error · `⊘` unreachable

---

## DO / DON'T

- **DO** implement new sync strategies by adding to `strategy.go`
- **DO** use `executor_noop.go` for `--dry-run` — don't add dry-run logic elsewhere
- **DON'T** call `pkg/repository` from executor — use `executor_git.go` (gitcmd direct)
- **DON'T** store credentials in `state_file.go`

---

## Testing

```go
// Use noop executor for unit tests
exec := reposync.NewNoopExecutor()
err := exec.Execute(ctx, plan, reposync.ExecOptions{DryRun: true})

// Use integration tests for real git ops
// see diagnostic_integration_test.go
```
