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
`../devbox`, the devbox itself.

## Expected vs Actual

- Expected: no action of the root config targets the devbox or a
  directory containing it.
- Actual: the devbox can be planned for reset or delete.
