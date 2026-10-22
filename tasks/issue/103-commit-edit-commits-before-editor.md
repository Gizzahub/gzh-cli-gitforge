---
id: ISSUE-103
title: "gz-git commit -e without --yes commits before the editor opens"
type: bug
priority: P2
severity: medium
discovered-in: "independent review of the ISSUE-099 fix"
discovered-at: 2026-10-22
created: 2026-10-22
---

## Summary

`gz-git commit -e` is documented as "edit messages in $EDITOR before
committing". Without `--yes`, `prepareCommitRun`
(`cmd/gz-git/cmd/commit.go`) leaves `DryRun` false because `commitEdit`
excludes the run from preview mode, and `BulkCommit`'s second phase does not
consult `Yes`. The first `BulkCommit` call in `runCommit` therefore commits
every dirty repository with its pre-editor message (the message map entry,
or the generated message when there is none). Only then does
`commitEditorResult` open the editor and run a second `BulkCommit`.

## Reproduction

1. Make a repository dirty under a scan root.
1. Run `EDITOR=true gz-git commit -e --json '{"r1":"feat: x"}' <root>`
   (no `--yes`).
1. Observed in the review: one new commit carrying `feat: x` was created
   before the editor ran, then `Canceled (empty file).` was printed and the
   exit code was 0.

## Expected vs Actual

- Expected: nothing is committed until the editor closes with content; an
  empty file cancels with no commit.
- Actual: the pre-editor messages are committed first, and canceling the editor
  does not undo that.

## Scope

- Fix in `cmd/gz-git/cmd/commit.go`: run the first `BulkCommit` as a dry run
  whenever `-e` is set, so only `commitEditorResult` commits.
- Add a regression test that cancels the editor and asserts HEAD is unchanged.
