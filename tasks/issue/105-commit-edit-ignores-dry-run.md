---
id: ISSUE-105
title: "gz-git commit -e --dry-run commits the edited messages"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-103 fix"
discovered-at: 2026-10-23
created: 2026-10-23
---

## Summary

`--dry-run` is documented as "show what would be done without doing it", but
with `-e` it does not hold. `commitEditorResult` (`cmd/gz-git/cmd/commit.go`)
sets `opts.DryRun = false` and `opts.Yes = true` unconditionally before its
`BulkCommit`, so once the editor closes with content every listed repository
is committed with the edited message. The ISSUE-103 fix made the first pass a
dry run for every `-e` run; it did not change this second pass.

## Reproduction

1. Make a repository `r1` dirty under a scan root.
1. Use an editor that writes `r1: feat: edited` to its file.
1. Run `gz-git commit -e --dry-run <root>`.
1. Observed on the ISSUE-103 branch: HEAD of `r1` moved from
   `Initial commit` to `feat: edited`.

## Expected vs Actual

- Expected: `--dry-run -e` lets the messages be edited and shows what would
  be committed, without committing; or the flag combination is rejected.
- Actual: the edited messages are committed.

## Fix note

Decide between keeping `commitFlags.DryRun` in `commitEditorResult` (edit,
then preview) and rejecting `-e` with `--dry-run`. Add a regression test that
asserts HEAD is unchanged.
