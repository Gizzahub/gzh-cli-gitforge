# Release promotion from the remote integration ref

`gz-git integrate run <remote>/<integration> --target <remote>/<default> --release`
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
  replaces. Apart from a [hotfix](#hotfix-landing), the default branch changes
  only by release, so applying the guard there would let a key the integration
  branch already deleted block the path that delivers the deletion
  (ISSUE-109). YAML and JSON syntax
  errors, several YAML documents and a duplicated `branch` key still fail
  the release.
- **Source commit: everything else.** Task patterns, reclaim and provenance
  come from the declaration committed at the release source commit. That is
  the commit the source ref resolves to after the fetch, held to
  `--source` when that flag is given. The release fast-forwards the
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

The symbolic-link refusal binds the target too. Since ISSUE-109 the target
declaration is read from the target commit, where a root `.gz-git.*`
committed as a symbolic link is refused; the earlier working-tree read
followed the link, so a target that used to release now refuses every
release. To recover, commit the declaration as a regular file on the default
branch.

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

## Integration that is not a release

An integration without `--release` reads its declaration from the working
tree it runs in, with one exception: `branch.integrationBranch`. The value
committed on the default branch decides where a task branch may land. The
default branch is the remote `HEAD` branch, read from its remote-tracking ref
after the fetch; `--no-fetch` reads that ref as it stands. This is the commit
a release reads too, since a release target is the default branch.

The task branch's own value never gates (ISSUE-409). A task branch is the
change being judged and may itself edit `integrationBranch`:

- A branch that declares the default branch would otherwise land there
  without a release.
- A branch that first names a separate integration branch would otherwise
  have no route. Its own value makes the default branch a release target,
  and the release refuses because the default branch does not name that
  branch yet.

So the change that names a new integration branch lands on the default branch
as an ordinary integration, while the default branch still declares itself.
From then on only a release or a [hotfix](#hotfix-landing) changes the default
branch.

Until that change lands, the default branch's value refuses every other
target, the new integration branch included, even where that branch already
declares itself:

```text
target must be the integration branch (origin/main) or the default branch (origin/main) (integrationBranch main from .gz-git.yaml at origin/main commit 9aab0def1234 gates this integration)
```

Land the declaration change with an explicit `--target <default branch>`.
Without `--target` the working tree's value picks the new integration branch,
which the default branch does not accept yet.

The authority is the default branch, not the target. An integration branch
whose commit still carries an older value takes the merge that updates it.

The working tree's `integrationBranch` still picks the target when `--target`
is omitted, and it gates when the default branch offers nothing to gate on:

- The default branch is unknown: `--no-fetch` finds no local
  `<remote>/HEAD`.
- The default branch has no root declaration, or declares no integration
  branch. This is a repository adopting the declaration.
- The default branch's root declaration is a symbolic link or does not
  parse. A release refuses that state, so an ordinary integration is the only
  repair; the check passes with a warning that names the problem.

A `<remote>/HEAD` that names a branch with no commit is not a fallback. The
remote deleted or renamed its default branch and the local ref is stale, so
the integration is refused until the ref is repaired:

```text
default branch declaration: refs/remotes/origin/HEAD names origin/main, which resolves to no commit; run 'git remote set-head origin --auto'
```

`--controller-config` and a caller-supplied integration branch are not
repository declarations, and the default branch's value does not replace
them. When the default branch's value gates, the check output and any target
refusal name it:

```text
  INFO  declaration — working tree declares integrationBranch develop; integrationBranch main from .gz-git.yaml at origin/main commit 9aab0def1234 gates the integration
```

```text
promoting onto the default branch requires --release (integrationBranch develop from .gz-git.yaml at origin/main commit 9aab0def1234 gates this integration)
```

Release never reclaims: the integration ref matches neither the reclaim
guard's task patterns nor the integration/default names it protects.

## Hotfix landing

`gz-git integrate run <task-branch> --hotfix` lands a task branch on the
default branch without a release, in a repository whose default branch names a
separate integration branch:

```text
gz-git integrate run dev/claude/mbp/fix/x --hotfix --source <full-sha> \
  --target origin/master@<full-sha> --record run-42@g7 \
  --owner claude/mbp/agent --json
```

A bare target such as `master@<full-sha>` reads the local branch, which must
then be at the pinned commit.

It is opt-in. A call without `--hotfix` is judged as described above, and a
task branch cannot reach the default branch that way. A hotfix lands only when
every condition holds; any violation pushes nothing and exits 1:

1. The call does not combine `--hotfix` with `--release`,
   `--direct-to-default`, `--controller-config` or `--no-fetch`.
1. `--source`, `--record`, `--owner` and a pinned `--target <ref>@<sha>` are
   all present.
1. The gate is the declaration committed on the default branch, read at the
   same commit the task lands on. There is no working-tree fallback and no
   caller-supplied integration branch: a missing or unreadable declaration
   refuses, and so does an integration branch that is the default branch or
   does not exist.
1. The target is the default branch, and the source is a local branch that is
   neither the default branch nor a declared integration branch.
1. No commit between the target and the source is reachable from the remote
   integration branch. Only the exact ref `refs/remotes/<remote>/<integration>`
   is read, and a missing ref refuses even where a local branch has the same
   name. No patch ids are compared, so a cherry-picked fix is accepted.
1. The source commits the same `branch.integrationBranch` as the target commit.

The exit code stays 4 when `--source` or the pinned target no longer matches.
With `--json` the run document carries `"landing": "hotfix"`; other landings
omit the field, and the receipt schema is the same. Reclaim follows the task
branch rules.

`gz-git` does not bring the hotfix back into the integration branch. The caller
does that as a separate integration.
