---
id: ISSUE-101
title: "`workspace status` lists a nested config's repositories that `workspace sync` skips without `sync.recursive`"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-96 fix"
discovered-at: 2026-10-22
created: 2026-10-22
---

## Summary

`workspace sync` plans a `type: config` workspace only when it sets
`sync.recursive` (`GetConfigWorkspaces`, `pkg/config/recursive.go`). The status
loader, `reposynccli.FileSpecLoader.loadConfigWorkspace`
(`pkg/reposynccli/config_loader.go`), loads every `type: config`
workspace regardless. `workspace status` and `quicksync --check`
therefore list a nested config's repositories that sync never clones,
and report them missing.

ISSUE-96 fixed where status looks for those repositories, under the
nested config's directory. It did not change which repositories status
considers part of the devbox. This mismatch was recorded there and is
carried over here.

## Reproduction

Derived from the code; not run end to end. A devbox with
`workspaces: {child: {type: config, path: child}}`, no `sync.recursive`,
and `child/.gz-git.yaml` listing `repositories: [{name: lib, url: <url>}]`.
`gz-git workspace sync` skips `child`; `gz-git workspace status` reports
`child/lib` missing.

## Expected vs Actual

- Expected: status and sync agree on which nested config workspaces
  belong to the devbox, or status marks the skipped ones as not synced.
- Actual: status lists repositories that sync skips.

## Fix note

`loadConfigWorkspace` is shared with `forge status`, which has no
`sync.recursive` notion. Decide whether the filter belongs in the
workspacecli wrapper or in the shared loader before changing it.
