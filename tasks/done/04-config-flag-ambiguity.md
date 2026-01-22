---
title: Clarify --config flag usage (input vs output)
priority: P2
effort: S
created: 2026-01-22
decision: Option A - Use --output, -o for output files
decision-at: 2026-01-23T00:00:00Z
started-at: 2026-01-23T02:30:00Z
completed-at: 2026-01-23T03:00:00Z
type: refactor
area: cli
tags: [consistency, api-design, ux]
context: --config is used for both input (read) and output (write) across commands
options:
  - label: 'Option A: Rename output flags to --output, -o'
    pros: Clear distinction; follows common CLI conventions (many tools use -o for output)
    cons: Requires changing init command flag
  - label: 'Option B: Context-specific naming (--input-config, --output-config)'
    pros: Very explicit about direction
    cons: Longer flag names; breaks existing usage
recommendation: Option A (--output, -o for output files)
recommendation-reason: Standard convention; -o for output is widely recognized; minimal breaking change
---

# Clarify --config flag ambiguity

## Problem

The `--config` flag is used for both **input** (read config) and **output** (write config) purposes across different commands:

| Command                   | --config Purpose      | Direction  |
| ------------------------- | --------------------- | ---------- |
| `clone --config`          | Read clone specs      | Input      |
| `workspace init --config` | Write new config      | **Output** |
| `workspace sync --config` | Read workspace config | Input      |
| `workspace add --config`  | Read/modify config    | Input      |

This is confusing because users expect `--config` to specify an input file.

## Decision Required

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

## AI Recommendation

**Option A (--output, -o for output files)** because:

1. `-o` for output is a widely recognized convention
1. Minimal change - only affects init/scan commands
1. Maintains backward compatibility with deprecation warning

______________________________________________________________________

**Awaiting decision to proceed with implementation.**

---

## Implementation Summary

**Completed:** 2026-01-23

**Changes Made (Option A):**
1. Renamed `workspace init --config` → `--output, -o`
2. Renamed `workspace scan --config` → `--output, -o`
3. Added deprecated `--config, -c` alias for backward compatibility
4. Updated usage examples to use `-o`

**Files Modified:**
- pkg/workspacecli/init_command.go
- pkg/workspacecli/scan_command.go

**Verification:**
- ✅ Build successful (`make fmt && make build`)
- ✅ Deprecation warning works: `Flag --config has been deprecated, use --output instead`
- ✅ New `--output, -o` flag works correctly

**Impact:**
- Clear distinction: `--config` for input, `--output` for output
- Follows common CLI convention (`-o` for output)
- Backward compatible with deprecation warnings
