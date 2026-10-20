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

Resolved on 2026-10-20 and revised on 2026-10-21 after the independent
review found that the first fix (d3eb76be) filtered after planning and
missed two paths.

- Each enclosing config directory is now refused where the child is
  planned, before any hook, configLink or config write runs. The forge and
  git planners take the config directories of the workspaces that recursed
  into the one being planned (`enclosing`) and skip a workspace whose path
  is one of them or an ancestor of one, with
  `Skipping workspace '<name>': path '<path>' reaches an enclosing config directory`
  (`reachesEnclosingConfigDir`, `pkg/workspacecli/self_sync_guard.go`).
  Ancestors are refused because a forge workspace there plans orphan
  deletes among its subdirectories, one of which holds the devbox.
- Behaviour change for child configs: an entry at any ancestor of an
  enclosing config directory, such as `~` or the folder holding the devbox,
  is now refused. When the devbox is named through a symlink, the directory
  holding that symlink is refused too. Siblings and descendants are still
  planned.
- Per-repository targets go through `keepActionsOffEnclosingConfigDirs`: a
  child's `repositories:` entries, and a child's forge repositories and
  orphan deletes, whose target is an enclosing config directory or an
  ancestor of one.
- Both recursion paths pass `enclosing`: `planConfigWorkspacesWithDryRun`
  (the planning-time recursion) passes the root config directory, and
  `runChildSync` (the `--recursive` sync after execution) passes the root
  plus every child above the one being synced (`recursiveSyncOpts.Enclosing`).
- The root config is planned with no enclosing directories, so root-level
  behaviour is unchanged.
- Tests (`pkg/workspacecli/self_sync_guard_test.go`):
  - `TestReachesEnclosingConfigDir`: the enclosing dir, a symlink to it, an
    enclosing dir named through a symlink, and an ancestor are refused; a
    descendant, a sibling, and no enclosing dirs are not.
  - `TestConfigPlannerSkipsChildEntriesReachingConfigDir`: a child git
    workspace at `..`, at a symlink to the root, and at `../..`; a child
    forge workspace at `..`; and a child `repositories:` entry at the
    symlink. Each keeps only the sibling `member` and prints the warning.
  - `TestConfigPlannerSkipsChildWorkspaceBeforeConfigLink`: in a real
    (non-dry) run, a child git workspace at `..` with a configLink leaves the
    root config untouched.
  - `TestRunChildSyncSkipsEntriesReachingEnclosingConfigDir`: the
    `--recursive` child sync skips a child git workspace at `..`, a child
    forge workspace at `..`, and a `repositories:` entry at the root.
  - `TestRunChildSyncPassesItsDirToGrandchildren`: a child that clones a
    grandchild from a local origin passes its own directory down, so the
    grandchild's workspace at `..` is skipped.
  - `TestKeepActionsOffEnclosingConfigDirs`: the per-repository filter
    drops a target that is the devbox or its parent and keeps a sibling.
- Mutation evidence, from the task worktree with each file restored after:
  ignoring `enclosing` in the workspace guards, dropping the ancestor walk,
  passing no `enclosing` from `runChildSync` to the git planner, and passing
  none to the child flat filter each fail the tests. After the second
  review, passing none from `runChildSync` to its flat filter or to the
  forge planner, and not appending the child to `Enclosing` for its
  grandchildren, also fail the tests.
- Not covered by a test: the calls that filter a child's forge actions
  after planning. No test seam stubs a forge provider, so removing them
  passes; the filter itself is covered directly.

## Follow-up

- [ISSUE-86](086-flat-repository-paths-resolve-against-cwd.md): a relative
  `repositories:` path is placed relative to the process working directory,
  not the config that lists it.
- [ISSUE-87](087-root-forge-workspace-reaches-devbox.md): a root forge
  workspace at an ancestor of the devbox can reset or delete the devbox.
- [ISSUE-88](088-root-config-link-replaces-own-config.md): a root workspace
  at the devbox with a configLink replaces the devbox's own config.
