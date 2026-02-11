---
id: TASK-002
title: "Workspace Sync Preview Mode (diff + confirmation)"
type: feature

priority: P2
effort: S

parent: null
depends-on: []
blocks: []

created-at: 2026-02-02T00:00:00Z
origin: gizzahub-devbox/TASK-005
started-at: 2026-02-02T15:50:00Z
decision-made: 2026-02-02T16:30:00Z
decision: "Option A: Preview as default, --yes to skip"
decision-by: "user"
completed-at: 2026-02-02T17:00:00Z
completion-summary: "Implemented preview summary, --yes flag, huh confirmation prompt, TTY detection, unit tests, docs updated"

archived-at: 2026-02-11T00:00:00Z
verified-at: 2026-02-11T00:00:00Z
verification-summary: |
  - Verified: buildSyncSummary(), displaySyncSummary(), confirmSyncPrompt(), isTerminal() in sync_command.go
  - Verified: --yes/-y flag and --dry-run interaction (needsConfirmation logic at line 159)
  - Verified: TTY detection using go-isatty for CI auto-approve
  - Verified: Unit tests pass (TestBuildSyncSummary 3 cases, TestDisplaySyncSummary 2 cases)
  - Verified: docs/usage/workspace-command.md updated with preview example and flag behavior matrix
  - Evidence: go test ./pkg/workspacecli/... -run "TestBuildSyncSummary|TestDisplaySyncSummary" PASS
---

## Purpose

Add interactive preview mode to `workspace sync` that shows detailed diff of changes before execution, with user confirmation prompt. Enhances sync safety by providing visibility into what will change.

## Background

Migrated from gizzahub-devbox TASK-005 (FR-A03.3). The codebase already has:
- `diff` command with detailed output
- `conflict detect` command
- `workspace sync --dry-run` for preview

This task integrates these into a cohesive interactive workflow.

## Definition of Done

- [x] Phase 0 fit analysis completed and documented
- [x] Preview summary displays before sync
- [x] Confirmation prompt implemented
- [x] `--yes` flag skips confirmation
- [x] TTY detection for CI mode (auto-yes)
- [x] Unit tests for preview logic
- [x] `make quality` passes
- [x] Documentation updated

## Implementation Summary

### Files Changed
- `pkg/workspacecli/sync_command.go` - Added preview summary, confirmation prompt, --yes flag
- `pkg/workspacecli/sync_preview_test.go` - Unit tests for preview functions
- `docs/usage/workspace-command.md` - Updated sync command documentation

### Key Functions Added
- `buildSyncSummary()` - Counts action types
- `displaySyncSummary()` - Displays preview summary
- `confirmSyncPrompt()` - Interactive confirmation using huh library
- `isTerminal()` - TTY detection for CI mode

### Flag Behavior Matrix

| Flags | Preview | Prompt | Execute |
|-------|---------|--------|---------|
| (default) | ✓ | ✓ | If 'y' |
| `--yes` | ✓ | ✗ | ✓ |
| `--dry-run` | ✓ | ✗ | ✗ |
| Non-TTY | ✓ | ✗ | ✓ |

## Verification

```bash
# Tests pass
go test ./pkg/workspacecli/... -run "TestBuildSyncSummary|TestDisplaySyncSummary"

# Quality passes
make quality
```
