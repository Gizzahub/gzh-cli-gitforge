# Release promotion from the remote integration ref

`gz-git integrate check|run <remote>/<integration> --target <remote>/<default> --release`
promotes the integration branch onto the default branch. A caller such as
CE `ce task run-release` runs it from the repository's primary checkout,
which stays on the default branch, and names the remote-tracking ref
(`origin/develop`) as the source.

That source is the remote's state by definition. It is never checked out and
has no upstream, so the task-branch rows are judged differently:

| Row                  | Task branch or local `develop` source    | Remote-tracking release source                 |
| -------------------- | ---------------------------------------- | ---------------------------------------------- |
| `working-tree`       | HEAD must be the branch, tree clean      | HEAD is not compared; the checkout stays clean |
| `push`               | upstream must equal the branch           | PASS: the source is the remote ref             |
| `readiness contract` | `none declared`: no repository code runs | `none declared`: no repository code runs       |

The checkout must still be clean because `run` fast-forwards the local
default-branch worktree after the push. A local `develop` source keeps the
task-branch rules: HEAD must be that branch.

gz-git runs no repository code, so the git rows alone decide readiness
([removed gate declarations](integrate-prepare-profile.md)).

## Declaration source

A release reads the repository-root `.gz-git.{yaml,yml,json}` from commits,
not from the primary checkout's working tree, and splits it by role.

- **Target commit: the gate.** The integration branch a release may promote
  is the `branch.integrationBranch` committed at the target commit (the
  `--target` ref after the fetch). Only that key is read, and the removed-key
  guard is skipped. That declaration is the pre-release state the release
  replaces. The default branch changes only by release, so applying the guard
  there would let a key the integration branch already deleted block the
  only path that delivers the deletion (ISSUE-109). YAML and JSON syntax
  errors, several YAML documents and a duplicated `branch` key still fail
  the release.
- **Source commit: everything else.** Task patterns, reclaim and provenance
  come from the declaration committed at the release source commit. That is
  the commit the source ref resolves to after the fetch, held to
  `--expect-source` when that flag is given. The release fast-forwards the
  default branch to that commit, so the default branch carries this
  declaration afterwards. It passes every declaration check: a removed key
  or a literal protected task pattern at the source commit still refuses the
  release, and the error names the file and the short commit.

The source's `integrationBranch` never gates the release. Otherwise a branch
could declare itself the integration branch and so authorize its own
promotion. The two values may differ; the release follows the target's. When
the target declares no integration branch, the remote-HEAD heuristic applies
and refuses the release. The source's value is never a fallback. A root
declaration committed as a symbolic link is refused on either side. The check
output names both values:

```text
  INFO  declaration — /repo/.gz-git.yaml at release source commit 0ee4886d16bb; integrationBranch source=develop, target=develop (from .gz-git.yaml at target commit 9aab0def1234, gates the release)
```

Nothing reads a release declaration before the fetch. A local ref can be
stale and still carry a key the remote already deleted; reading it would
judge a commit nobody approved. Each declaration is read once, after the
fetch, and reclaim and the evidence provenance reuse the source read. The
gating integration branch is declared in the target commit, so a release
needs an explicit `--target <default branch>`. Without one, `check` and
`run` refuse before the fetch:

```text
--release requires an explicit --target <default branch>
```

Integration into the integration branch keeps reading the working tree.

Release never reclaims: the integration ref matches neither the reclaim
guard's task patterns nor the integration/default names it protects.
