# pkg/reposynccli — CLAUDE.md

CLI commands for Forge API-based repository sync (GitHub, GitLab, Gitea).

---

## Purpose

Implements `gz-git forge` subcommands that fetch repository lists from forge providers
and sync them locally. Bridges CLI → forge providers → `reposync` executor.

**Contrast with `workspacecli`**: this package calls Forge APIs; workspacecli uses local config.

---

## Commands

| File | Command | Description |
|------|---------|-------------|
| `from_forge_command.go` | `forge from` | Clone/sync from org/group |
| `config_generate_command.go` | `forge config generate` | Generate `.gz-git.yaml` from forge |
| `config_command.go` | `forge config` | Config subcommands root |
| `status_command.go` | `forge status` | Health diagnosis across repos |
| `setup_command.go` | `forge setup` | Interactive setup wizard |

---

## Filtering (`filter.go`)

`forge from` supports repo filtering before sync:

```go
filter := reposynccli.NewFilter(reposynccli.FilterOptions{
    Language:       "go",       // --language go
    MinStars:       100,        // --min-stars 100
    MaxStars:       1000,       // --max-stars 1000
    LastPushWithin: "30d",      // --last-push-within 30d
    Include:        "^myorg/",  // --include regex
    Exclude:        "^myorg/deprecated", // --exclude regex
})
repos := filter.Apply(allRepos)
```

---

## Factory Pattern (`factory.go`)

```go
type Factory struct {
    ProviderFn  func(cfg) provider.Provider
    ExecutorFn  func() reposync.Executor
    ProgressFn  func() Progress
}
```

---

## Progress Reporting

| File | Purpose |
|------|---------|
| `progress_console.go` | Terminal progress bar |
| `progress_status.go` | Status line updates |

---

## DO / DON'T

- **DO** use `filter.go` for all repo filtering — don't add filter logic in commands
- **DO** pass `--dry-run` through to `reposync.Executor` — don't duplicate dry-run logic
- **DON'T** hardcode provider URLs — use `pkg/provider` abstractions
- **DON'T** store auth tokens in generated config files

---

## Testing

```go
// factory_test.go pattern — mock provider
f := reposynccli.NewFactory(
    reposynccli.WithProviderFn(func(cfg) provider.Provider { return mockProvider }),
)
```
