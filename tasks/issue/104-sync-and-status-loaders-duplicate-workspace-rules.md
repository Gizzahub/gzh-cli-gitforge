---
id: ISSUE-104
title: "sync and status resolve workspaces with duplicated rules that have drifted four times"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-101 decision"
discovered-at: 2026-10-23
created: 2026-10-23
---

## Summary

`gz-git` parses the same workspaces `.gz-git.yaml` with two independent
loaders:

- sync side: `pkg/workspacecli` planning (`sync_command.go`), built on
  `pkg/config` `LoadConfigRecursive`, `GetConfigWorkspaces` and
  `LoadWorkspaces`, plus `self_sync_guard.go`
- status side: `pkg/reposynccli` `FileSpecLoader` (`config_loader.go`:
  `loadWorkspacesConfig`, `loadConfigWorkspace`), used by `forge status`,
  `workspace status` (through `workspacecli.configDirSpecLoader`) and
  `quicksync --check`

Both sides must agree on four rules: the workspace path default, the
base directory a relative path resolves against, whether a
`type: config` workspace is included (the `sync.recursive` gate), and
cycle and self-reference handling. Each rule is implemented in both
packages, against two YAML schemas: `pkg/reposynccli/config_loader.go`
declares its own `workspacesConfig`, `workspaceEntry` and
`syncSettings` structs mirroring `pkg/config` types. The copies drifted
four times (ISSUE-95, ISSUE-96, ISSUE-102, ISSUE-101), and each drift
was fixed on one side only.

## Evidence

Duplicated rules, read from the code after ISSUE-101:

- Path default. Sync: `loadConfigRecursiveWithVisited`
  (`pkg/config/recursive.go`) and the planners in
  `pkg/workspacecli/sync_command.go` default a missing path to the
  workspace name. Status: `loadWorkspacesConfig` does the same since
  ISSUE-102; before, it used `"."`.
- Base directory. Three resolvers coexist: `pkg/config` `resolvePath`,
  workspacecli `resolveWorkspacePath`, and reposynccli `cleanPath` with
  an inline join onto `configDir`. `configDirSpecLoader`
  (`pkg/workspacecli/status_loader.go`) also rebases any repository
  target still relative.
- Config-workspace inclusion. Sync: `GetConfigWorkspaces` includes a
  workspace only when `ws.Sync.Recursive`. Status: ISSUE-101 added a
  `recursive` field to the local `syncSettings` and the same gate in
  `loadWorkspacesConfig`. The two still classify edge cases differently:
  `GetConfigWorkspaces` also treats an untyped workspace (or `type: git`
  without `url`) with `recursive: true` as a config workspace, which
  status loads as a git workspace; and it skips a `type: config`
  workspace that sets `source`, which status loads.
- Cycles. Sync: `loadConfigRecursiveWithVisited` caps depth at
  `MaxConfigDepth` (10) and tracks an enclosing-config chain;
  `self_sync_guard.go` adds `pointsAtConfigDir` (file identity) and
  `reachesEnclosingConfigDir` (walks up from each enclosing directory).
  Status: `loadWorkspacesConfig` carries a `configChain` through `ctx`
  and compares with `sameDir` (file identity), with no depth limit and
  no ancestor walk.

The four drifts:

- ISSUE-95: a nested config workspace cycle overflowed the `pkg/config`
  loader; fixed there only.
- ISSUE-96: status resolved a nested config's repository paths against
  the root directory instead of the nested config's directory.
- ISSUE-102: status defaulted a missing path to `"."` and had no cycle
  check, so a pathless config workspace overflowed the stack.
- ISSUE-101: status loaded `type: config` workspaces that sync skips
  without `sync.recursive`.

## Reproduction

Derived from the code; not run end to end. A devbox with
`workspaces: {child: {path: child, sync: {recursive: true}}}` (no
`type`, no `url`) and `child/.gz-git.yaml` listing
`repositories: [{name: lib, url: <url>}]`. `GetConfigWorkspaces`
returns `child`, so `gz-git workspace sync` plans `child/lib`; the
status loader treats `child` as a git workspace and scans it for
existing repositories, so `gz-git workspace status` does not list
`child/lib` until it is cloned.

## Expected vs Actual

- Expected: one resolution of path default, base directory,
  config-workspace inclusion and cycle handling, shared by the sync and
  status loaders, so a rule change lands once.
- Actual: each rule exists in two or three package-local copies over
  two schemas, and every fix so far landed on one side.

## Fix note

Remove the defect class: extract one workspace resolution both loaders
consume. Options:

1. `pkg/config` owns the resolver. It already owns the canonical
   schema, the recursive gate and the stronger cycle handling, and
   `pkg/reposynccli` already imports it. Risk: `pkg/config` gains a
   type shaped by reposync's needs.
1. A new shared package owns the resolver, keeping `pkg/config` free of
   reposync-facing types, at the cost of one more package.
1. Minimal: `pkg/reposynccli` decodes into `pkg/config` types instead of
   its local structs. Removes the schema copy but keeps the rule copies.

Open questions:

- Which cycle semantics become canonical: sync's guard with the
  ancestor walk and depth cap, or status's `sameDir` chain?
- `forge status` shares the status loader; should the recursive gate be
  a resolver option it can choose?
- `pkg/reposynccli/config_loader.go` is over 1000 lines and the
  file-size check flags it. Moving the workspace half
  (`workspacesConfig`, `loadWorkspacesConfig`, `loadForgeWorkspace`,
  `loadConfigWorkspace`) into a sibling file, after
  `config_loader_git_workspace.go`, fits this work.

## Progress

2026-10-23 (option 1, partial): the classification rule and the path
default are shared; the card stays open for the rest.

- Decisions (user): sync's rule is canonical, so an untyped or
  `type: git` workspace without `url` and with `sync.recursive: true` is a
  config workspace in status too; `type: config` (or `type: git`) with a
  `source` is a config error in both commands instead of forge in sync and
  config in status.
- `pkg/config/workspace_kind.go`: `ClassifyWorkspace` (and
  `(*Workspace).Kind`), `SyncsConfigWorkspace`, `WorkspacePathOrName`, and
  the sentinel `ErrWorkspaceTypeConflict`. `GetForgeWorkspaces`,
  `GetGitWorkspaces`, `GetConfigWorkspaces`, `ValidateWorkspace` and
  `config hierarchy` use it; the status loader `loadWorkspacesConfig`
  replaces its own type inference and recursive gate with it.
  `loadConfigRecursiveWithVisited` checks the conflict on top and nested
  entries but keeps recursing only into explicit `type: config` entries:
  that recursion only validates, and an error there drops every workspace
  behind sync's load warning, while the config planner loads an inferred
  config workspace and fails on its errors (review finding).
- The conflict fails `LoadConfigRecursive`; `workspace sync` returns that
  error instead of its usual load warning. Status returns it from `Load`,
  including from a nested config. Accepted asymmetry: sync also validates
  the child of an explicit non-recursive `type: config` workspace (the
  ISSUE-95 cycle tests rely on it), so only sync refuses a conflict there,
  as it already does a cycle or parse error.
- Kept: the loader still accepts an unknown type
  (`TestLoadConfigRecursive_InvalidWorkspaceType`); the validator reports
  it. Status now skips an unknown type with a warning, as sync plans none,
  where it used to load it as a git workspace.
- Tests: `TestClassifyWorkspace`, `TestWorkspaceGettersShareClassification`,
  `TestLoadConfigRecursive_RefusesConfigWorkspaceWithSource`; in
  `pkg/reposynccli` `TestLoadInfersConfigWorkspaceLikeSync` and
  `TestLoadRefusesConfigWorkspaceWithSource`; in `pkg/workspacecli`
  `TestWorkspaceSyncRefusesConfigWorkspaceWithSource` and
  `TestWorkspaceSyncInferredConfigWorkspace` (plans the child's
  repositories; a broken child config fails sync). Mutation evidence,
  reverted: the old status inference fails both status tests; the old
  `GetForgeWorkspaces` selection fails the getter test; disabling the sync
  abort fails both conflict subtests; recursing into inferred config
  workspaces fails the broken-child subtest.
- Docs: `docs/.claude-context/config-guide.md` "Type inference".

2026-10-26 (cycle semantics): one shared cycle rule.

- Decisions (user): one detector in `pkg/config` with file identity and the
  depth cap; a cycle or over-deep nesting is a config error in both
  commands. Sync's ancestor walk (`reachesEnclosingConfigDir`) stays in the
  sync planner: it guards writes over a config directory, not recursion.
- `pkg/config/config_chain.go`: `ConfigChain.Enter` (config directories of
  the nesting path, compared with `SameDir`, cap `MaxConfigDepth`),
  `ErrConfigCycle`, `ErrConfigDepth`, `IsConfigTreeError`. `SameDir` moved
  from `pkg/reposynccli`. `loadConfigRecursiveWithVisited` replaces its
  string-keyed chain with it, so a symlink back to an ancestor reports a
  cycle instead of the depth error; `visited` still guards parent configs.
  The status loader carries the chain through `ctx` and returns the error
  instead of skipping the workspace, also from a deeper nested config;
  `workspace sync` returns it instead of its load warning.
- Kept: a parent-config cycle (`parent:` chain, guarded by `visited`) is not
  `ErrConfigCycle`; it stays a load warning in sync as in status. The other
  `LoadConfigRecursive` callers (the child-config planners,
  `config hierarchy`, `scan-depth`, `scan-exclude`, the manager) still
  warn on these errors; the top-level sync load and the status loader catch
  a cycle first.
- Accepted asymmetry (review finding), from the existing classification
  split: the sync loader enters only explicit `type: config` workspaces,
  `recursive` or not, while status enters every config workspace with
  `sync.recursive`, inferred ones included. So an untyped recursive
  workspace at `path: .` fails status but is skipped by the sync planner's
  self-sync guard, and an explicit non-recursive `type: config` cycle fails
  only sync.
- Tests: `TestConfigChainEnter`,
  `TestLoadConfigRecursive_ConfigWorkspaceSymlinkCycle`,
  `TestLoadConfigRecursive_ConfigWorkspaceDepth`, the parent-cycle check in
  `TestLoadConfigRecursive_SelfReference`; in
  `pkg/reposynccli` `TestLoadConfigWorkspaceCycles` (now errors) and
  `TestLoadConfigWorkspaceDepth`; in `pkg/workspacecli`
  `TestWorkspaceSyncRefusesConfigWorkspaceCycle`. Mutation evidence,
  reverted: the old sync abort condition fails the sync cycle test.
- Docs: `docs/.claude-context/config-guide.md` "Config workspace cycles".

Still open: one base-directory resolver (three remain), the duplicated YAML
schema in `pkg/reposynccli`, the `forge status` gate question, and the
`config_loader.go` split.
