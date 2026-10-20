---
id: ISSUE-76
title: "Bulk pull, fetch and update exit 0 when a repository stops on a conflict"
type: bug
priority: P3
severity: low
discovered-in: "pull --push follow-up review"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`gz-git pull`, `fetch` and `update` count only `StatusError` toward the
partial-failure exit code (`errPartialFailure`, exit 2). A repository that
stops on a conflict, an in-progress rebase or merge, or missing auth is
shown in the issue list but the command still exits 0, so a script cannot
tell a clean run from one that left repositories unsynced.

`pull --push` already counts these outcomes as failures
(`pullPushFailedPulls` in `cmd/gz-git/cmd/pull_push.go`), because there the
caller asked for a push that did not happen. Plain `pull` was left unchanged
on purpose: changing it alters the exit contract of every bulk command.

## Reproduction

1. Make a clone with a local commit that conflicts with a new upstream
   commit on the same line.
1. Run `gz-git pull --merge-strategy rebase <workspace>`.
1. The result shows `CONFLICT`; the exit code is 0.

## Expected vs Actual

- Expected: a run that leaves a repository unsynced because of a conflict,
  an in-progress operation or missing auth is distinguishable by exit code.
- Actual: only `StatusError` raises exit 2; these outcomes exit 0.

## Decision needed

Whether conflict, rebase/merge-in-progress and auth-required count as
failures for `pull`, `fetch` and `update` as a family. This is an exit-code
contract change across bulk commands and belongs in a decision record
before code. Not-syncable outcomes (no remote, no upstream, skipped) should
stay non-failures either way, as in `pull --push`.

## Resolution (2026-10-20)

The user approved option A and the change is recorded as D8 in
`docs/10-architecture/90-design-decisions.md`. A consumer check found no caller
that relies on exit 0. `handoff start` and gzh-cli use library results, and no
scripts call these commands.

- `syncFailures` in `cmd/gz-git/cmd/bulk_common.go` counts `error`, `conflict`,
  `rebase-in-progress`, `merge-in-progress` and `auth-required` for `pull`,
  `fetch` and `update`. It replaces `pullPushFailedPulls`, so `pull --push`
  counts pulls the same way.
- Base-sync statuses stay non-failures, as already documented in the
  troubleshooting guide.
- `TestPullConflictExitsTwo` covers the change: it fails on the old counting
  (exit 0) and passes now. Released as 0.11.0 (minor; incompatible exit-code
  change).
