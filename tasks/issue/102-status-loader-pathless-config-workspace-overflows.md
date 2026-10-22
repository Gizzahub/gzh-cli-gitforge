---
id: ISSUE-102
title: "a `type: config` workspace without `path` crashes the status loader with a stack overflow"
type: bug
priority: P3
severity: medium
discovered-in: "ISSUE-96 independent review"
discovered-at: 2026-10-22
created: 2026-10-22
---

## Summary

`reposynccli.FileSpecLoader.loadWorkspacesConfig`
(`pkg/reposynccli/config_loader.go`) defaults a workspace's missing
`path` to `.`, the top-level config's own directory. For a
`type: config` workspace, `loadConfigWorkspace` then reads the same
`.gz-git.yaml` again and recurses until the process dies. This loader
has no cycle check; ISSUE-95 fixed the equivalent crash only in
`pkg/config`.

`workspace sync` defaults the same missing path to the workspace name
(`pkg/workspacecli/sync_command.go`), so status and sync also disagree
about where such a workspace lives.

## Reproduction

Reproduced on 2026-10-22, before and after
5d3afd35: a `.gz-git.yaml` of `workspaces: {child: {type: config}}`
loaded with `reposynccli.FileSpecLoader{}.Load` ends in
`fatal error: stack overflow`. `forge status`, `workspace status` and
`quicksync --check` go through this loader.

## Expected vs Actual

- Expected: the status loader resolves a missing `path` the way sync
  does, and reports a config workspace cycle as an error.
- Actual: the process crashes with a stack overflow.
