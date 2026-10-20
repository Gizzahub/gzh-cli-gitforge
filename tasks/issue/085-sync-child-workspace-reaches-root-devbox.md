---
id: ISSUE-85
title: "workspace sync plans the root devbox through a child config's `path: ..`"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-80 independent review"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`workspace sync` refuses to plan the config directory itself
([ISSUE-80](080-sync-self-guard-compares-path-strings.md)). A recursive
config workspace, however, is planned by passing the child's directory as
the config directory: `planConfigWorkspacesWithDryRun` calls
`createActionsFromFlatConfig(childPlan, wsPath)` and
`planGitWorkspaces(ctx, childCfg, wsPath, ...)`. The guards inside compare
the child's entries only with the child directory. A child entry that
reaches the root devbox, such as a git workspace with `path: ..` or a
symlink to the root, is planned as an update target with strategy `reset`.

## Reproduction

A root devbox config with `workspaces: {child: {path: child, sync: {recursive: true}}}`, and `child/.gz-git.yaml` with
`workspaces: {up: {type: git, path: .., url: <any url>}}`. On 35566495,
`planConfigWorkspacesWithDryRun` returns one action, `up`, whose target is
the root devbox directory, with strategy `reset`. A test in a scratch copy reproduced it;
the CLI path (`gz-git workspace sync --dry-run`) was not run.

## Expected vs Actual

- Expected: an entry of a child config that reaches the root devbox is
  skipped with the self-sync warning, as a root entry is.
- Actual: the root devbox is planned for `reset`, which can overwrite its
  uncommitted changes.

## Resolution

Resolved on 2026-10-20.

- `planConfigWorkspacesWithDryRun` passes the child's flat-repository and
  git-workspace actions through `keepActionsOffConfigDir`
  (`pkg/workspacecli/self_sync_guard.go`). It drops any action whose target
  reaches the root config directory, by the same `pointsAtConfigDir` check,
  and prints
  `Skipping '<repo>' in workspace '<child>': path '<target>' points to config directory`.
  A target is resolved the way the executor resolves it.
- Forge actions are not filtered: a forge workspace plans its repositories
  under its own path, so no forge action's target is the workspace
  directory itself, and the child's forge guard already compares the
  workspace path.
- Test: `TestConfigPlannerSkipsChildEntriesReachingConfigDir` covers a child
  git workspace at `..`, a child git workspace at a symlink to the root, and
  a child `repositories:` entry at that symlink; each keeps only the sibling
  `member` and prints the warning.
- Mutation evidence, from a scratch copy: removing the flat-repository
  filter fails the `repositories` case, and removing the git-workspace
  filter fails both git cases.

## Observation, not investigated

A child config's `repositories:` entry with a relative `path` keeps it
unresolved (`TargetPath: ".."`), and `filepath.Abs` resolves it against the
process working directory, not the child directory. Whether the executor
places such repositories relative to the child is not checked here; it is a
candidate for a separate card.
