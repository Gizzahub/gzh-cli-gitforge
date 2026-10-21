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
The same holds for a root config synced from another directory, and for
the `--recursive` sync after execution (`runChildSync`,
`pkg/workspacecli/sync_recursive.go`), which builds the child's flat
actions the same way.

## Fix note

The per-repository guard (`keepActionsOffEnclosingConfigDirs`,
`pkg/workspacecli/self_sync_guard.go`) resolves a relative target against
the working directory on purpose, as the executor does. Rebase the paths
once in the loader, not in the executor alone, or the guard and the
executor will disagree about where a path points.

## Expected vs Actual

- Expected: a relative `repositories:` path is resolved against the
  directory of the config that lists it, as workspace paths are.
- Actual: it is resolved against the process working directory.

## Resolution

Resolved on 2026-10-21.

- `createActionsFromFlatConfig` (`pkg/workspacecli/sync_command.go`) now
  resolves each repository's `TargetPath` with `resolveWorkspacePath`
  against the directory of the config that lists it, as workspace paths
  are resolved. Absolute paths are kept. It is called
  for the root config, for a child config while planning, and for the
  `--recursive` child sync (`runChildSync`), so all three are covered.
- Place of the fix: the fix note asks for the loader. The rebase is done
  where the flat plan becomes actions instead, because every consumer that
  plans or executes reads those actions afterwards: the devbox filter in
  the same function, `keepActionsOffEnclosingConfigDirs`, the executor and
  `recordReadOnlyWorkspaceAccess`. They therefore agree on one absolute
  target. `recordReadOnlyWorkspaceAccess` and
  `reconcileIntegrationParticipationExcept` already joined a relative
  path with the config directory, so before this change they used a
  different directory than the executor. The loader itself is
  unchanged: `config_loader_test.go` and `flat_loader_parity_test.go` pin
  its relative `TargetPath`, and `validate` reads it without placing
  anything.
- Behaviour changes:
  - Planned flat repositories now show an absolute path in plan and
    dry-run output.
  - A `repositories:` path starting with `~/` is now expanded to the home
    directory; before, the executor treated it as a directory named `~`
    under the working directory.
  - A flat entry and a forge or git workspace entry for the same directory
    are now deduplicated, since both keys are absolute. The flat entry is
    planned first, so it wins, and the dropped entry's workspace settings
    (read-only access, integration branch) do not apply to that
    directory. Before, both actions ran on the same directory.
- `workspace status` and the `quicksync --check` health check load through
  `reposynccli.FileSpecLoader`, which keeps a flat `repositories:` path
  relative. Sync now places it under the config directory, so they would
  have checked the working directory instead. Both now load through
  `configDirSpecLoader` (`pkg/workspacecli/status_loader.go`). It passes
  the base loader an absolute config path, so workspace entries, which
  that loader joins with the config's directory, come back absolute, and
  it resolves any remaining relative path against the config directory.
  `workspace status --format json` therefore reports an absolute `path`
  for flat repositories. `reposynccli` itself and the `sync` command
  family that uses it are unchanged.
- Found out of scope and registered as ISSUE-96: a nested `type: config`
  workspace's relative repositories come back from the base loader
  relative to the nested config, so status resolves them against the
  top-level config directory, not where sync places them. Before this
  change they resolved against the working directory, so it is not a
  regression.
- Tests (`pkg/workspacecli/flat_repository_path_test.go`):
  `TestFlatConfigResolvesRelativePathsAgainstConfigDir` checks that `lib`
  and `deps/nested` are placed under the config directory, an absolute
  path is kept, `~/src/home` is expanded, and `.` is dropped as the
  devbox. `TestConfigDirSpecLoaderPlacesPathsLikeSync` loads through the
  status wrapper from another working directory and expects the same
  paths as sync. `TestConfigDirSpecLoaderAcceptsRelativeConfigPath` loads a
  workspaces config through a relative `-c` that names a directory, from
  above and from below the devbox, and expects the workspace path once. `TestConfigPlannerResolvesChildRepositoriesAgainstChildDir`
  plans a child whose `lib` has no path and whose `up` is at `..`: `lib`
  is placed in the child, and `up`, the devbox seen from the child, is
  dropped by the enclosing-config guard.
- Mutation: with the resolution line removed, both planner tests fail;
  before the fix `up` resolved against the test's working directory and
  was planned. With the wrapper's resolution removed, the status loader
  test fails; with the base loader given the relative path again, the
  relative `-c` test fails.
