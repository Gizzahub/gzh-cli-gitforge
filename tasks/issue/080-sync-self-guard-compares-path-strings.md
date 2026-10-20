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
