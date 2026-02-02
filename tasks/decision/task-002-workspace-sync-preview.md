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
moved-at: 2026-02-02T16:00:00Z
context: "Phase 0 analysis complete. User decision needed on UX approach."
options:
  - label: "Option A: Preview as default, --yes to skip"
    pros: "Safest UX, consistent with modern CLIs (gh pr merge)"
    cons: "May slow down scripted workflows if --yes not used"
  - label: "Option B: Preview opt-in via --preview flag"
    pros: "Non-breaking, explicit control"
    cons: "Users may not discover the feature"
  - label: "Option C: Separate --confirm flag"
    pros: "Orthogonal to --dry-run"
    cons: "More flags to remember"
recommendation: "Option A"
recommendation-reason: "Modern CLIs default to safe behavior. --yes is standard for automation."
---

## Purpose

Add interactive preview mode to `workspace sync` that shows detailed diff of changes before execution, with user confirmation prompt. Enhances sync safety by providing visibility into what will change.

## Background

Migrated from gizzahub-devbox TASK-005 (FR-A03.3). The codebase already has:
- `diff` command with detailed output
- `conflict detect` command
- `workspace sync --dry-run` for preview

This task integrates these into a cohesive interactive workflow.

## Phase 0: Command Structure Fit Analysis (COMPLETED)

### Questions Answered

1. **Flag Design**: Recommend `--yes` to skip confirmation (Option A)
2. **Integration**: Reuse `BulkDiff` and `ConflictDetector` (both available)
3. **Output Format**: JSON output should include preview data
4. **Default Behavior**: Preview default, `--yes` to skip

### Analysis Checklist (COMPLETED)

- [x] Review `pkg/workspacecli/sync_command.go` structure
- [x] Review `pkg/repository/bulk_diff.go` API
- [x] Review `pkg/merge/detector.go` API
- [x] Check TUI capabilities (`huh` library available in go.mod v0.8.0)
- [x] Understand current `--dry-run` behavior (preview only, no execution)

## Fit Analysis Result (2026-02-02)

### Recommended Approach

- [x] **Make preview default**, add `--yes` to skip confirmation
- [ ] Keep `--dry-run` as-is (preview without execution)
- [ ] `--dry-run --yes` → dry-run wins (no execution)

**Rationale**: Modern CLIs default to safe behavior (see `gh pr merge`, `terraform apply`).

### Existing Code Reuse

| Component | Package | Reusable? | Notes |
|-----------|---------|-----------|-------|
| Diff generation | `pkg/repository/bulk_diff.go` | ✅ | `BulkDiff()` → `BulkDiffResult` |
| Conflict detection | `pkg/merge/detector.go` | ✅ | `ConflictDetector.Preview()` |
| TUI prompts | `charmbracelet/huh` | ✅ | Already in go.mod v0.8.0 |
| Status formatters | `pkg/tui/formatter.go` | ✅ | `FormatHealthIcon()` |

### Proposed UX Flow

```
$ gz-git workspace sync -c config.yaml

Analyzing 12 repositories...

Summary:
  ✓ 5 will be updated (behind remote)
  + 3 will be cloned (new)
  ⊘ 2 skipped (up-to-date)
  ⚠ 2 have warnings (dirty + behind)

Proceed? [y/N/d(details)/q]
```

### Flag Behavior Matrix

| Flags | Preview | Prompt | Execute |
|-------|---------|--------|---------|
| (default) | ✓ | ✓ | If 'y' |
| `--yes` | ✓ | ✗ | ✓ |
| `--dry-run` | ✓ | ✗ | ✗ |
| `--dry-run --yes` | ✓ | ✗ | ✗ |

### Blockers or Concerns

1. **TTY detection**: Non-TTY should default to `--yes` (CI mode)
2. **Large orgs**: 100+ repos needs summary-only by default
3. **Conflict definition**: dirty AND behind = warning, not error

---

## Scope

### Must
- Preview summary before sync execution
- Show clone/update/skip/conflict counts
- Interactive confirmation prompt
- Detail view option (per-repo diff)
- Works with existing `--dry-run` (preview without prompt)

### Should
- Color-coded output (green=safe, yellow=warning, red=conflict)
- Statistics: files added/modified/deleted per repo
- `--yes` flag to skip confirmation (CI/automation)

### Must Not
- Automatic conflict resolution (user must decide)
- Complex merge strategies
- Breaking existing `--dry-run` behavior

## Definition of Done

- [x] Phase 0 fit analysis completed and documented
- [ ] Preview summary displays before sync
- [ ] Confirmation prompt implemented
- [ ] Detail view shows per-repo diff
- [ ] `--yes` flag skips confirmation
- [ ] Unit tests for preview logic
- [ ] `make quality` passes

## Implementation Checklist

### Phase 0: Analysis
- [x] Complete fit analysis above
- [ ] **Get approval on approach** ← DECISION NEEDED

### Phase 1: Preview Logic
- [ ] Add preview types to `pkg/reposync/types.go`
- [ ] Implement `PreviewSync()` in `pkg/reposync/preview.go`
- [ ] Integrate `BulkDiff` for change details
- [ ] Integrate conflict detection

### Phase 2: CLI Integration
- [ ] Add `--yes` flag (skip confirmation)
- [ ] Implement summary display in `pkg/workspacecli/sync_command.go`
- [ ] Add detail view prompt handler
- [ ] Wire confirmation logic

### Phase 3: TUI Enhancement
- [ ] Color-coded summary output
- [ ] Interactive prompt using `huh` library

### Phase 4: Documentation
- [ ] Update `workspace sync --help`
- [ ] Add examples to `docs/usage/workspace-command.md`

## Verification

```bash
# Manual testing
gz-git workspace sync -c test.yaml           # Preview + prompt
gz-git workspace sync -c test.yaml --yes     # Preview, no prompt, execute
gz-git workspace sync -c test.yaml --dry-run # Preview only

# Quality
make quality
```

## Technical Notes

- Reuse `BulkDiffOptions` pattern for preview options
- Use `huh` library for interactive prompts
- Consider `--format json` output for CI integration
- Existing TUI patterns in `pkg/tui/formatter.go`

## Related Code

```
pkg/workspacecli/sync_command.go  # Main sync logic
pkg/reposync/run.go               # Orchestrator
pkg/repository/bulk_diff.go       # Diff implementation
pkg/merge/detector.go             # Conflict detection
pkg/tui/                          # TUI utilities
```
