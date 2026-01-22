---
title: Unify target/path directory naming across CLI and config
priority: P1
effort: M
created: 2026-01-22
decision: Option A - Standardize on --path
decision-at: 2026-01-23T00:00:00Z
started-at: 2026-01-23T00:00:00Z
completed-at: 2026-01-23T01:00:00Z
type: refactor
area: cli, config
tags: [consistency, api-design, ux]
context: CLI uses --target, YAML uses path. Need to choose one naming convention.
options:
  - label: "Option A: Standardize on 'path'"
    pros: Shorter, cleaner; matches YAML field; common convention
    cons: Requires CLI flag rename
  - label: "Option B: Standardize on 'target'"
    pros: Already used in CLI; no CLI changes needed
    cons: Longer; breaking change for existing config files
recommendation: Option A (path)
recommendation-reason: Matches existing YAML convention; shorter; more idiomatic
---

# Unify target/path directory naming

## Problem

The same concept (destination directory) uses different names across CLI flags, YAML fields, and struct fields:

| Layer        | Name Used    | Location                  |
| ------------ | ------------ | ------------------------- |
| CLI flag     | `--target`   | reposynccli, workspacecli |
| YAML field   | `path`       | config/types.go           |
| Struct field | `TargetPath` | FromForgeOptions          |
| Struct field | `Path`       | Workspace struct          |

This inconsistency confuses users when switching between CLI and config files.

## Decision Required

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

## AI Recommendation

**Option A (path)** because:

1. YAML already uses `path` - changing CLI is less disruptive than changing config format
1. Existing config files would break with Option B
1. "path" is shorter and more idiomatic in CLI tools

______________________________________________________________________

**Awaiting decision to proceed with implementation.**

______________________________________________________________________

## Implementation Summary

**Completed:** 2026-01-23

**Changes Made:**

1. Renamed all `--target` flags to `--path` across 6 command files
1. Added deprecated `--target` alias with deprecation warning
1. Updated all usage examples and help text
1. Updated MarkFlagRequired calls from "target" to "path"

**Files Modified:**

- pkg/reposynccli/from_forge_command.go
- pkg/reposynccli/config_generate_command.go
- pkg/reposynccli/status_command.go
- pkg/reposynccli/setup_command.go
- pkg/workspacecli/add_command.go
- pkg/workspacecli/status_command.go
- pkg/workspacecli/generate_command.go

**Verification:**

- ✅ Build successful (`make fmt && make build`)
- ✅ Deprecation warning works (`Flag --target has been deprecated, use --path instead`)
- ✅ All usage examples updated to use `--path`

**Backward Compatibility:**

- Old `--target` flag still works with deprecation warning
- Users can migrate gradually
