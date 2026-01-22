---
title: Standardize --format flag values across commands
priority: P2
effort: M
created: 2026-01-22
moved-at: 2026-01-22T00:00:00Z
type: refactor
area: cli
tags: [consistency, api-design, ux]
context: "Different commands support different --format values (default, compact, json, llm, table, csv, markdown)"
options:
  - label: "Option A: Define core + extended formats"
    pros: "Flexible; all commands support essentials; specialized formats where needed"
    cons: "Need to document which formats apply where"
  - label: "Option B: Universal format set"
    pros: "All commands support all formats; simple mental model"
    cons: "Not all formats make sense for all commands; more implementation"
recommendation: "Option A (core + extended formats)"
recommendation-reason: "Pragmatic; ensures json/llm work everywhere while allowing specialized formats"
---

# Standardize --format flag values

## Problem

Different command groups support different `--format` values:

| Command Group | Valid Values |
|---------------|--------------|
| Bulk ops (status, fetch, pull, push) | `default, compact, json, llm` |
| History commands | `table, json, csv, markdown, llm` |
| Workspace status | `default, json, compact` |

## Decision Required

### Option A: Define core + extended formats (RECOMMENDED)

**Core formats** (all commands must support):
- `default` - Human-readable
- `json` - Machine-parseable
- `llm` - LLM-optimized

**Extended formats** (command-specific):
- `compact` - For bulk/status commands
- `table`, `csv`, `markdown` - For history/report commands

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

## AI Recommendation

**Option A (core + extended)** because:
1. Ensures critical formats (json, llm) work everywhere
2. Doesn't force meaningless implementations
3. Clear categorization helps users understand capabilities

---
**Awaiting decision to proceed with implementation.**
