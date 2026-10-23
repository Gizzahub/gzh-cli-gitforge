---
id: ISSUE-109
title: "Release reads the target checkout's working-tree declaration, so a removed key there blocks the release that deletes it"
type: bug
priority: P1
severity: high
discovered-in: "ce task run-release in seven singlepageweb-* repositories after the ISSUE-100 cleanup, 2026-10-23"
discovered-at: 2026-10-23
created: 2026-10-23
---

## Summary

`ce task run-release` runs `gz-git integrate check|run` with `--release`
from the primary checkout, which stays on the default branch. `integrate.Check`
loaded the repository-root declaration with
`config.LoadRepoRootTaskPattern(root)` (`pkg/integrate/check.go:99`), which
reads the working tree, which is the default branch's `.gz-git.yaml`.

After [ISSUE-89](089-release-0-12-blocked-by-removed-keys.md)
and [ISSUE-90](090-removed-key-guard-skips-other-loaders.md), a removed key
refuses that load with exit 2. [ISSUE-100](../../issue/100-removed-key-scan-missed-singlepageweb.md)
then found `branch.prepareProfile` still declared in singlepageweb-devbox's
child repositories. The repositories deleted it on `develop`, for example
singlepageweb-calculator `0ee4886d16bb1076194d0ed188eef375609cc70b`.
`master` still carries it, and `master` changes only by release. So the
release that delivers the deletion is refused by the key it deletes. Seven
singlepageweb-\* repositories are stuck.

## Reproduction

From the primary checkout on `master` of an affected repository, with
`origin/develop` at a commit whose `.gz-git.yaml` no longer declares
`branch.prepareProfile`:

```sh
gz-git integrate check origin/develop --target origin/master --release \
  --expect-source 0ee4886d16bb1076194d0ed188eef375609cc70b
```

Exit 2, stderr:

```text
branch.prepareProfile was removed: gz-git no longer builds, runs, or lints repository code; verification belongs to the task runtime (CE `ce task gate` and the repository's own checks), which must pass before gz-git is called; delete it from <primary-checkout>/.gz-git.yaml
```

`integrate run` with the same arguments fails the same way. The message was
reproduced in-process on a fixture: `master` commits a declaration with the
key, and `develop` deletes it
(`pkg/integrate/release_declaration_test.go` `releaseDeclarationFixture`).

## Expected vs Actual

- Expected: a release applies the declaration it installs, which is the one
  committed at the release source commit. The default branch carries that
  declaration after the fast-forward. The removed-key guard still refuses a
  source commit that carries a removed key. The integration branch that
  gates the release still comes from the target, so a source cannot
  authorize its own promotion.
- Actual: the release applies the default branch's pre-release working-tree
  declaration and refuses on a key the source already removed. The default
  branch has no other path to drop the key without bypassing the
  release.

## Fix

In `--release` mode, the declaration is read from commits after the fetch,
and split by role.

- **Source commit.** `loadPlanDeclaration`
  (`pkg/integrate/release_declaration.go`), called from `resolveTarget`,
  reads the source commit the plan resolved. It is strictly validated.
  `validateOwnedCheckPlan` returns that read. `finishRunReclaim` and the
  evidence provenance reuse it through `checkedPlanDeclaration` for task
  patterns, reclaim and provenance.
- **Target commit.** `loadTargetIntegrationBranch` reads only
  `branch.integrationBranch` committed at the target commit, through
  `config.CommittedIntegrationBranch`. It skips the removed-key guard, and
  syntax and document-shape errors still fail. When no controller supplies
  an integration config, that value feeds `ResolveIntegrationBranch`, and so
  drives `validateTarget` and the remote-release-ref rows.

An earlier revision of this fix took the gating integration branch from the
source commit. That let a source branch declare itself the integration
branch and authorize its own promotion. The source's value now never gates
and is never a fallback. A target that declares none keeps the heuristic,
which refuses a release.

Nothing reads a release declaration before the fetch. An earlier version
read the local source ref pre-fetch whenever `--expect-source` was not
local, so a stale clone was refused at a commit nobody approved. A release
without `--target` would need the declaration before the fetch to derive
its target. `Check` and Run's `rejectUnconfirmedHeuristicTarget` refuse it
with `ErrReleaseNeedsTarget`. It never passed `validateTarget` anyway.

The committed declaration files are listed with `git diff-tree` against the
empty tree, restricted by `:(glob)**/.gz-git.{yaml,yml,json}` pathspecs. The
listing therefore does not grow with the rest of the tree. `ls-tree` does
not support glob pathspecs.

`config.LoadCommittedTaskPattern` applies the removed-key guard, the
literal-protected-name check and the other root declaration rules to the
source's committed bytes. Its errors name the file and the short commit. A
root declaration committed as a symbolic link is refused on either side. The
check report prints
`INFO  declaration — <path> at release source commit <short>; integrationBranch source=<list>, target=<list> (from <file> at target commit <short>, gates the release)`. Non-release
integration still reads the working tree. Its pre-fetch read and post-fetch
change detection are unchanged.

When the provenance cannot read the declaration, it records a
`declaration unreadable at intent time: <error>` fact. This stays non-fatal,
as before.

There is no flag, no environment bypass and no warning-only downgrade.

## Resolution

Resolved on 2026-10-23 by `c8b44f0a` (read the release declaration from the
source commit), `2cd836b4` (read it only after the fetch) and `f02ae7a4`
(gate a release on the target's `integrationBranch`), released in v0.13.1
(`9dc8bcce`). The follow-ups are ISSUE-110.

## Scope

- Out of scope: the singlepageweb-\* releases themselves. They need a gz-git
  release with this fix.
