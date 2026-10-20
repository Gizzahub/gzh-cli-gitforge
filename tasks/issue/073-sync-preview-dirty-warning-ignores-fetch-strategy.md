---
id: ISSUE-73
title: "Sync preview warns that a fetch-only sync may overwrite uncommitted changes"
type: bug
priority: P3
severity: low
discovered-in: "foundation-devbox gz-git workspace adoption"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

With `defaults.sync.strategy: fetch`, the sync preview still warns that a dirty
checkout's uncommitted changes may be overwritten. The fetch strategy only runs
`git fetch origin` (`pkg/repository/update.go:251-253`, :272-275) and cannot
touch the working tree, so the warning overstates the risk and teaches
operators to ignore a warning that matters for pull and reset strategies.

Both preview paths ignore the strategy:

- Detailed preview (`--dry-run` or verbose, `sync_command.go:269-279`):
  `analyzeRepoChanges` (`pkg/workspacecli/sync_command.go:1985-1997`) adds the
  `ConflictTypeDirtyWorktree` conflict and `Local uncommitted changes may be overwritten` for every `ActionUpdate` on a dirty working tree.
  `action.Strategy` is available but not read.
- Compact preview (plain `workspace sync`): `displayCompactSyncPreview` prints
  `has local changes (may be overwritten)` (`sync_command.go:2221-2228`).

## Reproduction

On gz-git 0.10.5 (source ed395f5d; unchanged at master a312815d), from
foundation-devbox with a dirty `airflow-dags` checkout, excerpt of
`gz-git workspace sync --dry-run`:

```
  • airflow-dags: Local uncommitted changes may be overwritten
  ...
  ↓ airflow-dags (update)
```

## Expected vs Actual

- Expected: under the fetch strategy the preview does not warn that local
  changes may be overwritten, because fetch never touches the working tree.
- Actual: the overwrite warning is printed for every dirty checkout that is
  planned for update. This was reproduced in the detailed preview; for the
  compact preview it comes from reading the code.

## Follow-up scope

Derive the dirty-worktree conflict and both warnings from the effective
strategy: keep them for strategies that touch the working tree; for fetch,
omit them or report the dirty state as information. Extend
`pkg/workspacecli/sync_preview_test.go` (dirty-warning assertions at
:193-198, :267, :287) with a fetch-strategy case for both preview paths.

Registration only; no remediation is included here.

## Resolution

Fixed in `a16d4659`. `updateTouchesWorktree` derives the warning from the
planned `Action.Strategy`, which every planner fills with the effective value
and which the executor runs as reset when unset. Fetch drops the dirty-worktree conflict
and warning from both preview paths; every other strategy, including an unset
one, keeps them. `TestSyncPreviewDirtyWarningFollowsStrategy` covers fetch,
pull, and reset on a real dirty repository for both paths, and fails on the
previous code.
