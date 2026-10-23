---
id: ISSUE-80
title: "workspace sync's self-sync guard compares path strings, so a symlink plans the devbox itself"
type: bug
priority: P3
severity: medium
discovered-in: "ISSUE-72 re-review"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`workspace sync` refuses to plan a workspace that points at the config
directory, so a devbox is never reset by its own config. The guard is
`wsPath == configDir || effectivePath == "."`, a string comparison, in
three places in `pkg/workspacecli/sync_command.go` (:948, :1172, :1336 at
origin/master 963280cc). A path that reaches the same directory through
another spelling, such as a symlink to the devbox, passes the guard, and
sync plans the devbox checkout as an update target.

ISSUE-72 made the status loader compare the directories with
`os.SameFile` (`sameDir` in `pkg/reposynccli/config_loader_git_workspace.go`);
sync still accepts the bypass.

## Reproduction

In a devbox checkout, add to its `.gz-git.yaml` a workspace
`self: {type: git, path: <symlink to the devbox>, url: <any url>}` and run
`gz-git workspace sync --dry-run`. The plan reports `1 will be updated`
for `self`, with the warning that local uncommitted changes may be
overwritten.

## Expected vs Actual

- Expected: sync skips the workspace with the self-sync warning, as it does
  for `path: .`.
- Actual: sync plans the devbox itself as an update target.

## Scope

Compare the workspace path with the config directory by file identity in
all three guards, and cover the symlink and relative-config spellings in
the sync tests. Registration only; no remediation is included here.

## Resolution

- The three workspace guards (forge, git and config planners in
  `pkg/workspacecli/sync_command.go`) and a fourth guard the card did not
  count, in `createActionsFromFlatConfig`, which filters the root
  `repositories:` list (default strategy `reset`), call
  `pointsAtConfigDir` (`pkg/workspacecli/self_sync_guard.go`). It keeps
  the string match and adds `os.SameFile`, so a symlink to the config
  directory, or a spelling that differs only in case on a case-insensitive
  volume, is skipped. Relative spellings were already caught: the config
  directory is absolute and `filepath.Join` cleans the workspace path. A
  workspace path that does not exist yet cannot be the config directory
  and is still planned.
  The helper sits in its own file instead of growing `sync_command.go`;
  `sameDir` in `pkg/reposynccli` stays unexported.
- Tests: `TestPointsAtConfigDirMatchesFileIdentity` (same string, symlink,
  relative config directory, `.` segment, sibling, missing path) and
  `TestSyncPlannersSkipSymlinkToConfigDir`, which runs each planner on a
  symlink to a devbox whose config lists a member, and
  `TestFlatConfigSkipsSymlinkToConfigDir` for the root list.
- Mutation evidence, reverted before commit: restoring the string
  comparison at any one of the four guards fails the test for that guard.
  The first cut of the test missed the config planner mutant, because that
  planner also skips a path without a child config; the fixture now writes
  the devbox config and the assertion names the self-sync reason.
- Independent review (PASS WITH WARNINGS) follow-up: the fourth guard
  above, and the wording about relative spellings. The review also noted
  that the recursive planners compare a child workspace only with the
  child directory, so a child `path: ..` reaching the root devbox is not
  compared with the root; that predates this card and is not reproduced
  here.
