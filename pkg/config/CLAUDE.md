# pkg/config — CLAUDE.md

Configuration management with 5-layer precedence and profile support.

---

## Purpose

Loads, validates, and merges gz-git configuration from multiple sources.
Supports two config kinds: **repositories** (flat list) and **workspace** (hierarchical).

---

## 5-Layer Precedence (Highest → Lowest)

```
1. Command flags       --provider gitlab
2. Project config      .gz-git.yaml (current dir or parent)
3. Active profile      ~/.config/gz-git/profiles/{active}.yaml
4. Global config       ~/.config/gz-git/config.yaml
5. Built-in defaults
```

---

## Key Types (`types.go`)

| Type | Purpose |
|------|---------|
| `ConfigKind` | `"repositories"` or `"workspace"` |
| `ConfigMeta` | `version`, `kind`, `metadata` header |
| `Profile` | Named profile with forge defaults |
| `Hooks` | `before`/`after` command lists (no shell expansion) |
| `RepositorySpec` | Per-repo config (url, name, path, branch, hooks) |
| `WorkspaceConfig` | Hierarchical workspace with groups |

---

## Config File Paths (`paths.go`)

```
~/.config/gz-git/
├── config.yaml          # Global config
├── profiles/
│   ├── default.yaml
│   └── work.yaml
└── state/               # Internal state (do not edit)
```

---

## Kind Auto-Detection (`loader.go`)

| File Content | Detected Kind |
|-------------|---------------|
| Has `workspaces:` or `profiles:` key | `workspace` |
| Has `repositories:` key | `repositories` |
| Empty / template | `repositories` (default) |

---

## Usage Pattern

```go
// Load config with precedence
cfg, err := config.Load(config.LoadOptions{
    ProjectFile: ".gz-git.yaml",
    ProfileName: "work",
})

// Validate
if err := config.Validate(cfg); err != nil {
    return err
}
```

---

## Profile Management (`manager.go`)

```bash
gz-git config profile create work
gz-git config profile use work
gz-git config show --effective   # merged result
gz-git config hierarchy          # tree view
```

---

## DO / DON'T

- **DO** use `config.Load()` — never parse YAML directly in commands
- **DO** run `config.Validate()` before using a loaded config
- **DON'T** expand env vars in hook commands (hooks run without shell)
- **DON'T** hardcode `~/.config/gz-git/` — use `paths.ConfigDir()`

---

## Symlinks (`symlink.go`)

Config files support symlink resolution for shared team configs. `symlink.go` resolves these safely without following circular links.

---

## Testing

```go
// Use temp dir with config file
dir := t.TempDir()
os.WriteFile(filepath.Join(dir, ".gz-git.yaml"), testYAML, 0644)
cfg, err := config.LoadFrom(dir)
```
