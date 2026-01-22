---
title: Standardize --format flag values across commands
priority: P2
effort: M
created: 2026-01-22
decision: Option A - Core + extended formats
decision-at: 2026-01-23T00:00:00Z
started-at: 2026-01-23T03:00:00Z
completed-at: 2026-01-23T03:30:00Z
type: refactor
area: cli
tags: [consistency, api-design, ux]
context: Different commands support different --format values (default, compact, json, llm, table, csv, markdown)
options:
  - label: 'Option A: Define core + extended formats'
    pros: Flexible; all commands support essentials; specialized formats where needed
    cons: Need to document which formats apply where
  - label: 'Option B: Universal format set'
    pros: All commands support all formats; simple mental model
    cons: Not all formats make sense for all commands; more implementation
recommendation: Option A (core + extended formats)
recommendation-reason: Pragmatic; ensures json/llm work everywhere while allowing specialized formats
---

# Standardize --format flag values

## Problem

Different command groups support different `--format` values:

| Command Group                        | Valid Values                      |
| ------------------------------------ | --------------------------------- |
| Bulk ops (status, fetch, pull, push) | `default, compact, json, llm`     |
| History commands                     | `table, json, csv, markdown, llm` |
| Workspace status                     | `default, json, compact`          |

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
1. Doesn't force meaningless implementations
1. Clear categorization helps users understand capabilities

______________________________________________________________________

**Awaiting decision to proceed with implementation.**

______________________________________________________________________

## Implementation Summary

**Completed:** 2026-01-23

**Changes Made (Option A - Core + Extended Formats):**

**Core formats** (all commands):

- `default` - Human-readable (default)
- `json` - Machine-parseable
- `llm` - LLM-optimized

**Extended formats** (command-specific):

- `compact` - Bulk/status commands only
- `table`, `csv`, `markdown` - History/report commands only

**Files Modified:**

- cmd/gz-git/cmd/bulk_common.go:
  - Added CoreFormats constant
  - Updated ValidHistoryFormats to include "default"
  - Added documentation comments
- pkg/reposynccli/status_command.go: Updated help text to include llm
- pkg/workspacecli/status_command.go: Updated help text to include llm

**Verification:**

- ✅ Build successful (`make fmt && make build`)
- ✅ History commands now support "default" format
- ✅ All commands support core formats (default, json, llm)
- ✅ Extended formats remain command-specific

**Benefits:**

- Consistent core format support across all commands
- Clear documentation of format categories
- Command-specific formats where appropriate
