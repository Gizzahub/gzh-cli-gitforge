---
title: Unify target/path directory naming across CLI and config
priority: P1
effort: M
created: 2026-01-22
status: todo
type: refactor
area: cli, config
tags: [consistency, api-design, ux]
---

# Unify target/path directory naming

## Problem

The same concept (destination directory) uses different names across CLI flags, YAML fields, and struct fields:

| Layer | Name Used | Location |
|-------|-----------|----------|
| CLI flag | `--target` | reposynccli, workspacecli |
| YAML field | `path` | config/types.go |
| Struct field | `TargetPath` | FromForgeOptions |
| Struct field | `Path` | Workspace struct |

This inconsistency confuses users when switching between CLI and config files.

## Current State

### CLI Flags (--target)

```go
// pkg/reposynccli/from_forge_command.go:85
cmd.Flags().StringVarP(&opts.TargetPath, "target", "t", ".", "target directory")

// pkg/workspacecli/add_command.go:54
cmd.Flags().StringVarP(&addTarget, "target", "t", "", "target path")

// pkg/workspacecli/status_command.go:50
cmd.Flags().StringVarP(&statusTarget, "target", "t", ".", "directory to scan")
```

### YAML Config (path)

```yaml
# .gz-git.yaml
workspaces:
  myproject:
    path: ~/mydevbox/myproject  # Uses 'path', not 'target'
```

### Struct Fields (mixed)

```go
// pkg/reposynccli/from_forge_command.go
type FromForgeOptions struct {
    TargetPath string  // Uses 'TargetPath'
}

// pkg/config/types.go:301
type Workspace struct {
    Path string `yaml:"path"`  // Uses 'Path'
}
```

## Proposed Solution

### Option A: Standardize on "path" (RECOMMENDED)

**Changes**:
- Rename CLI flags from `--target` to `--path`
- Keep `--target` as deprecated alias
- All struct fields use `Path`
- YAML uses `path`

**Benefits**:
- Shorter, cleaner
- Matches YAML field name
- Common convention in many tools

### Option B: Standardize on "target"

**Changes**:
- Rename YAML field from `path` to `target`
- Keep `path` as deprecated alias in YAML
- All struct fields use `Target` or `TargetPath`

**Drawbacks**:
- Longer names
- Breaking change for existing config files

## Files Affected

```
pkg/reposynccli/from_forge_command.go:85   # --target flag
pkg/reposynccli/config_generate_command.go # --target flag
pkg/workspacecli/add_command.go:54         # --target flag
pkg/workspacecli/status_command.go:50      # --target flag
pkg/workspacecli/generate_command.go:47    # --target flag
pkg/config/types.go:301                    # Workspace.Path
```

## Acceptance Criteria

- [ ] **Decision**: Choose "path" or "target"
- [ ] **Implementation**:
  - [ ] Rename CLI flags to chosen name
  - [ ] Add backward compatibility alias
  - [ ] Align struct field names
  - [ ] Ensure YAML field matches
- [ ] **Documentation**:
  - [ ] Update CLAUDE.md
  - [ ] Update example configs
- [ ] **Testing**:
  - [ ] Test new flag name
  - [ ] Test backward compatibility
- [ ] **Quality**:
  - [ ] Run `make quality`

## Priority Justification

**P1 (High)**: Core UX issue affecting daily usage of CLI vs config files.
