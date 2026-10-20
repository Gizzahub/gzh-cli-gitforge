---
id: ISSUE-86
title: "a relative `repositories:` path is placed relative to the working directory"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-85 independent review"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

A flat config's `repositories:` entry keeps its `path` as written, and the
entry's name when `path` is omitted (`pkg/workspacecli/config_loader.go`,
`TargetPath: path`). Nothing rebases it on the directory of the config that
lists it, and the executor clones to and updates `Repo.TargetPath` as given
(`pkg/reposync/executor_git.go`). A relative path is therefore resolved
against the process working directory. Workspace paths, by contrast, are
resolved against the config directory (`resolveWorkspacePath`).

## Reproduction

Derived from the code; not run end to end. A root devbox with
`workspaces: {child: {path: child, sync: {recursive: true}}}`, and
`child/.gz-git.yaml` with `repositories: [{name: lib, url: <url>}]`. Run
`gz-git workspace sync` from the root: the child's planned action has
`TargetPath: "lib"`, so `lib` is cloned into the root, not into `child/`.
The same holds for a root config synced from another directory.

## Expected vs Actual

- Expected: a relative `repositories:` path is resolved against the
  directory of the config that lists it, as workspace paths are.
- Actual: it is resolved against the process working directory.
