---
id: ISSUE-87
title: "a root forge workspace at an ancestor of the devbox can reset or delete it"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-85 independent review"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

The root config's forge planner skips a workspace only when its path is the
config directory itself. A forge workspace at an ancestor of the devbox,
such as `path: ..`, is planned, and its per-repository targets are
`<path>/<repo>` (`buildTargetPath`, `pkg/reposync/planner_forge.go`). Two
of them can be the devbox:

- a repository of the org whose name is the devbox directory's name is
  planned for update with the workspace strategy, which may be `reset`;
- with `sync.cleanupOrphans: true`, every subdirectory of the workspace path
  that is not an org repository is planned for delete
  (`planOrphanCleanup`), and the executor removes it with `os.RemoveAll`.

[ISSUE-85](085-sync-child-workspace-reaches-root-devbox.md) refuses this
for child configs only; the root config is planned with no enclosing
directories.

## Reproduction

Derived from the code; not run, since the forge planner has no provider
stub and a real provider needs network. A root devbox `devbox/` whose
`.gz-git.yaml` has
`workspaces: {org: {path: .., source: {provider: <p>, org: <o>}, sync: {cleanupOrphans: true}}}`
where `<o>` has no repository named `devbox`: the plan includes a delete of
`<parent>/devbox`, the absolute path of the devbox itself.

## Expected vs Actual

- Expected: no action of the root config targets the devbox or a
  directory containing it.
- Actual: the devbox can be planned for reset or delete.

## Resolution

Resolved on 2026-10-21.

- `planForgeWorkspaces` (`pkg/workspacecli/sync_command.go`) now skips a
  forge workspace whose path is an ancestor of the config directory being
  planned, with
  `Skipping workspace '<name>': path '<path>' contains the config directory`.
  It reuses `reachesEnclosingConfigDir` with that config directory, so a
  symlink to an ancestor is refused too. The check runs at every level,
  the root included, after the existing self-sync and enclosing checks, so
  the ISSUE-85 message for a child entry reaching the devbox is unchanged.
- Behaviour change for the root config: a forge workspace at `..`, `~` or
  any other ancestor of the devbox is no longer planned. For a child
  config, a forge workspace at a directory between the devbox and the
  child, which ISSUE-85 still planned, is now refused as well, since its
  orphan cleanup would delete the child's own directory. Siblings and
  descendants are planned as before.
- The root plan also filters the planned forge repositories with
  `keepActionsOffEnclosingConfigDirs` against the devbox, as children
  already did (ISSUE-85). This drops a repository whose target reaches the
  devbox through a symlink inside a sibling workspace
  (`../sibling/devbox -> ../devbox`), which the workspace-level check
  cannot see. The call site is in the command's `RunE` with no provider
  seam, so this filter has no direct test; the helper itself is covered
  by the ISSUE-85 tests.
- Scope: forge workspaces only, as planned. Root git and config
  workspaces at an ancestor still plan; a git workspace there targets the
  ancestor directory itself rather than its subdirectories.
- Test (`pkg/workspacecli/root_forge_ancestor_guard_test.go`):
  `TestPlanForgeWorkspacesRefusesAncestorOfRootConfigDir` loads a root
  config at `<parent>/devbox` with `cleanupOrphans: true` and calls
  `planForgeWorkspaces` with no enclosing directories, as the root plan
  does. Cases: `..`, the grandparent as an absolute path and a symlink to
  the parent are refused with no actions; `../sibling` is still planned.
  The base URL is unreachable and the context cancelled, so no network
  is used. The grandparent is a directory the test owns.
  `TestConfigPlannerRefusesChildForgeWorkspaceBetweenDevboxAndChild` plans
  a child at `devbox/a/child` whose forge workspace is `..`, and asserts it
  is refused with the new message and no actions.
- Mutation: with the check disabled (`if false && ...`), the three
  ancestor cases and the child case fail; the sibling case still passes.
