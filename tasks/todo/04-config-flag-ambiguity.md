---
title: Clarify --config flag usage (input vs output)
priority: P2
effort: S
created: 2026-01-22
status: todo
type: refactor
area: cli
tags: [consistency, api-design, ux]
---

# Clarify --config flag ambiguity

## Problem

The `--config` flag is used for both **input** (read config) and **output** (write config) purposes across different commands:

| Command | --config Purpose | Direction |
|---------|------------------|-----------|
| `clone --config` | Read clone specs | Input |
| `workspace init --config` | Write new config | **Output** |
| `workspace sync --config` | Read workspace config | Input |
| `workspace add --config` | Read/modify config | Input |

This is confusing because users expect `--config` to specify an input file.

## Current State

### Input usage (correct intuition)

```go
// cmd/gz-git/cmd/clone.go:77
cloneCmd.Flags().StringVarP(&cloneConfig, "config", "c", "", "YAML config file for clone specifications")

// pkg/workspacecli/sync_command.go:185
cmd.Flags().StringVarP(&syncConfigPath, "config", "c", "", "config file path")
```

### Output usage (counterintuitive)

```go
// pkg/workspacecli/init_command.go:73
cmd.Flags().StringVarP(&initConfigPath, "config", "c", ".gz-git.yaml", "output config file path")
```

## Proposed Solution

### Option A: Rename output flags (RECOMMENDED)

**Changes**:
- Keep `--config, -c` for input files
- Use `--output, -o` for output files
- Backward compatibility: accept `--config` in init with deprecation warning

**Benefits**:
- Clear distinction between input/output
- Follows common CLI conventions (many tools use `-o` for output)

### Option B: Context-specific naming

**Changes**:
- `--input-config` for reading
- `--output-config` for writing

**Drawbacks**:
- Longer flag names
- Breaks existing usage

## Files Affected

```
pkg/workspacecli/init_command.go:73  # --config used for output
pkg/workspacecli/scan_command.go     # Check if uses --config for output
```

## Acceptance Criteria

- [ ] **Decision**: Choose naming convention
- [ ] **Implementation**:
  - [ ] Rename output flags to `--output, -o`
  - [ ] Keep `--config` as deprecated alias where needed
  - [ ] Update help text to clarify direction
- [ ] **Documentation**:
  - [ ] Update CLAUDE.md
  - [ ] Update command examples
- [ ] **Testing**:
  - [ ] Test new flag name
  - [ ] Test backward compatibility
- [ ] **Quality**:
  - [ ] Run `make quality`

## Priority Justification

**P2 (Medium)**: Affects usability but users can infer from context. Less critical than P1 issues.
