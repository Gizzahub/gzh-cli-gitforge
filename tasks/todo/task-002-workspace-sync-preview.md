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
---

## Purpose

Add interactive preview mode to `workspace sync` that shows detailed diff of changes before execution, with user confirmation prompt. Enhances sync safety by providing visibility into what will change.

## Background

Migrated from gizzahub-devbox TASK-005 (FR-A03.3). The codebase already has:
- `diff` command with detailed output
- `conflict detect` command
- `workspace sync --dry-run` for preview

This task integrates these into a cohesive interactive workflow.

## Phase 0: Command Structure Fit Analysis (REQUIRED FIRST)

Before implementation, verify the feature fits the existing command structure.

### Questions to Answer

1. **Flag Design**: `--preview` vs `--confirm` vs `--interactive`?
2. **Integration**: Reuse existing `BulkDiff` and `ConflictDetect` or inline?
3. **Output Format**: How does preview interact with `--format json|llm`?
4. **Default Behavior**: Should preview be opt-in or default?

### Analysis Checklist

- [ ] Review `pkg/workspacecli/sync_command.go` structure
- [ ] Review `pkg/repository/bulk_diff.go` API
- [ ] Review `cmd/gz-git/cmd/conflict.go` API
- [ ] Check TUI capabilities in `pkg/tui/`
- [ ] Understand current `--dry-run` behavior

### Expected Output

```markdown
## Fit Analysis Result

### Recommended Approach
- [ ] Add `--preview` flag to `workspace sync`
- [ ] Add `--confirm` flag (prompt before execution)
- [ ] Make preview default, add `--no-preview` to skip
- [ ] Other: ___

### Existing Code Reuse
| Component | Package | Reusable? |
|-----------|---------|-----------|
| Diff generation | pkg/repository/bulk_diff.go | ? |
| Conflict detection | pkg/merge/detector.go | ? |
| TUI prompts | pkg/tui/ | ? |

### Proposed UX Flow
1. `gz-git workspace sync -c config.yaml`
2. Shows: "Analyzing 12 repositories..."
3. Displays summary: 3 clone, 5 update, 2 skip, 2 conflict
4. Prompts: "Proceed? [y/N/d(details)]"
5. If 'd': shows full diff per repo
6. If 'y': executes sync

### Blockers or Concerns
- ...
```

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

- [ ] Phase 0 fit analysis completed and documented
- [ ] Preview summary displays before sync
- [ ] Confirmation prompt implemented
- [ ] Detail view shows per-repo diff
- [ ] `--yes` flag skips confirmation
- [ ] Unit tests for preview logic
- [ ] `make quality` passes

## Implementation Checklist

### Phase 0: Analysis
- [ ] Complete fit analysis above
- [ ] Get approval on approach

### Phase 1: Preview Logic
- [ ] Add preview types to `pkg/reposync/types.go`
- [ ] Implement `PreviewSync()` in `pkg/reposync/preview.go`
- [ ] Integrate `BulkDiff` for change details
- [ ] Integrate conflict detection

### Phase 2: CLI Integration
- [ ] Add `--preview` / `--confirm` / `--yes` flags
- [ ] Implement summary display in `pkg/workspacecli/sync_command.go`
- [ ] Add detail view prompt handler
- [ ] Wire confirmation logic

### Phase 3: TUI Enhancement
- [ ] Color-coded summary output
- [ ] Interactive prompt using `pkg/tui/` or `huh` library

### Phase 4: Documentation
- [ ] Update `workspace sync --help`
- [ ] Add examples to `docs/usage/workspace-command.md`

## Verification

```bash
# Phase 0: Analysis
cat pkg/workspacecli/sync_command.go | head -100
cat pkg/repository/bulk_diff.go | head -50

# Phase 1-2: Manual testing
gz-git workspace sync -c test.yaml --preview
gz-git workspace sync -c test.yaml --confirm
gz-git workspace sync -c test.yaml --yes  # Skip confirmation

# Compare with existing
gz-git workspace sync -c test.yaml --dry-run

# Phase 3: Quality
make quality
```

## Technical Notes

- Reuse `BulkDiffOptions` pattern for preview options
- Use `huh` library (already in deps) for interactive prompts
- Consider `--format json` output for CI integration
- Existing TUI patterns in `pkg/tui/formatter.go`

## Related Code

```
pkg/workspacecli/sync_command.go  # Main sync logic
pkg/reposync/run.go               # Orchestrator
pkg/repository/bulk_diff.go       # Diff implementation
pkg/merge/detector.go             # Conflict detection
cmd/gz-git/cmd/conflict.go        # CLI conflict command
pkg/tui/                          # TUI utilities
```
