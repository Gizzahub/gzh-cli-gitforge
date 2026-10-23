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

After [ISSUE-89](../_archive/issue/089-release-0-12-blocked-by-removed-keys.md)
and [ISSUE-90](../_archive/issue/090-removed-key-guard-skips-other-loaders.md), a removed key
refuses that load with exit 2. [ISSUE-100](100-removed-key-scan-missed-singlepageweb.md)
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
  source commit that carries a removed key.
- Actual: the release applies the default branch's pre-release working-tree
  declaration and refuses on a key the source already removed. The default
  branch has no other path to drop the key without bypassing the
  release.

## Fix

In `--release` mode, every release-path loader reads the declaration
committed at the release source commit through `loadPlanDeclaration`
(`pkg/integrate/release_declaration.go`):

- Check's first read: the `--expect-source` commit, or the source ref when
  the flag is omitted.
- The post-fetch revalidation in `validateOwnedCheckPlan`: the fetched source
  SHA.
- Run's `rejectUnconfirmedHeuristicTarget` and `finishRunReclaim`.
- The evidence provenance.

`config.LoadCommittedTaskPattern` applies the removed-key guard, the
literal-protected-name check and the other root declaration rules to the
committed bytes. Its errors name the file and the short commit. A root
declaration committed as a symbolic link is refused. The check report prints
`INFO  declaration — <path> at release source commit <short>`. Non-release
integration still reads the working tree.

There is no flag, no environment bypass and no warning-only downgrade.

## Scope

- Out of scope: the singlepageweb-\* releases themselves. They need a gz-git
  release with this fix.
