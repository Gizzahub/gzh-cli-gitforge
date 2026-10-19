# ISSUE: Sync preview warns that a fetch-only sync may overwrite uncommitted changes

- Status: open; reproduced on gz-git 0.10.5 (source ed395f5d); code unchanged at master a312815d
- Owner: `pkg/workspacecli`

## Finding

With `defaults.sync.strategy: fetch`, `gz-git workspace sync --dry-run` on a
workspace whose checkout has uncommitted changes prints:

```
  • airflow-dags: Local uncommitted changes may be overwritten
  ↓ airflow-dags (update)
```

`analyzeRepoChanges` (`pkg/workspacecli/sync_command.go:1985-1997`) adds the
`ConflictTypeDirtyWorktree` conflict and this warning for every
`ActionUpdate` on a dirty working tree without consulting the repository's sync
strategy. A fetch only updates remote-tracking refs and cannot overwrite the
working tree, so the preview overstates the risk and teaches operators to
ignore a warning that matters for pull/reset strategies.

## Follow-up scope

Derive the dirty-worktree conflict and warning from the effective strategy:
keep them for strategies that touch the working tree, and for fetch either omit
them or report the dirty state as information. Extend `sync_preview_test.go`
with a fetch-strategy case.

Registration only; no remediation is included here.
