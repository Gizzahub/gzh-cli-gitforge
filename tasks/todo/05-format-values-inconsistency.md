---
title: Standardize --format flag values across commands
priority: P2
effort: M
created: 2026-01-22
status: todo
type: refactor
area: cli
tags: [consistency, api-design, ux]
---

# Standardize --format flag values

## Problem

Different command groups support different `--format` values, making it hard for users to remember which formats are available:

| Command Group | Valid Values |
|---------------|--------------|
| Bulk ops (status, fetch, pull, push) | `default, compact, json, llm` |
| History commands | `table, json, csv, markdown, llm` |
| Workspace status | `default, json, compact` |

## Current State

### Bulk commands (cmd/gz-git/cmd/bulk_common.go:93-96)

```go
func validateBulkFormat(format string) error {
    validFormats := []string{"default", "compact", "json", "llm"}
    // history commands also accept: table, csv, markdown
}
```

### Workspace status (pkg/workspacecli/status_command.go:58)

```go
cmd.Flags().StringVarP(&statusFormat, "format", "f", "default", "output format (default, json, compact)")
```

## Format Value Analysis

| Format | Bulk | History | Workspace | Description |
|--------|------|---------|-----------|-------------|
| `default` | ✅ | ❌ | ✅ | Human-readable with colors |
| `compact` | ✅ | ❌ | ✅ | Minimal output |
| `json` | ✅ | ✅ | ✅ | Machine-parseable JSON |
| `llm` | ✅ | ✅ | ❌ | LLM-optimized (token efficient) |
| `table` | ❌ | ✅ | ❌ | ASCII table |
| `csv` | ❌ | ✅ | ❌ | CSV export |
| `markdown` | ❌ | ✅ | ❌ | Markdown table |

## Proposed Solution

### Option A: Define core + extended formats (RECOMMENDED)

**Core formats** (all commands must support):
- `default` - Human-readable
- `json` - Machine-parseable
- `llm` - LLM-optimized

**Extended formats** (command-specific):
- `compact` - For bulk/status commands
- `table`, `csv`, `markdown` - For history/report commands

**Implementation**:
1. Create shared format validation function
2. Commands declare which format set they use
3. Error messages list valid formats for that command

### Option B: Universal format set

**All commands support all formats**:
- `default, compact, json, llm, table, csv, markdown`

**Drawbacks**:
- Not all formats make sense for all commands
- More implementation work

## Files Affected

```
cmd/gz-git/cmd/bulk_common.go:93-96    # Bulk format validation
pkg/workspacecli/status_command.go:58  # Workspace format
pkg/workspacecli/sync_command.go       # Check format support
cmd/gz-git/cmd/history.go              # History format validation
```

## Acceptance Criteria

- [ ] **Decision**: Choose format standardization approach
- [ ] **Implementation**:
  - [ ] Create shared format types/constants
  - [ ] Add `llm` format to workspace status
  - [ ] Update validation to use shared set
  - [ ] Improve error messages to list valid formats
- [ ] **Documentation**:
  - [ ] Document format options in CLAUDE.md
  - [ ] Add format examples to command help
- [ ] **Testing**:
  - [ ] Test all formats work correctly
  - [ ] Test error messages for invalid formats
- [ ] **Quality**:
  - [ ] Run `make quality`

## Priority Justification

**P2 (Medium)**: Affects discoverability and scripting, but commands still work with their supported formats.
