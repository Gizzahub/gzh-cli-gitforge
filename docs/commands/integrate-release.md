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

A release reads the repository-root `.gz-git.{yaml,yml,json}` committed at
the release source commit, not the primary checkout's working tree. The
source commit is the one the source ref resolves to after the fetch, held
to `--expect-source` when that flag is given. The release fast-forwards
the default branch to that commit, so its committed declaration is the one
the default branch carries afterwards; the working-tree copy is the state the
release replaces. The default branch changes only by release, so reading the
working tree would let a key the integration branch already deleted block the
only path that delivers the deletion (ISSUE-109).

The committed file passes every declaration check unchanged: a removed key
or a literal protected task pattern at the source commit still refuses the
release, and the error names the file and the short commit. A root
declaration committed as a symbolic link is refused. The check output names
the provenance:

```text
  INFO  declaration — /repo/.gz-git.yaml at release source commit 0ee4886d16bb
```

Nothing reads a release declaration before the fetch. A local source ref can
be stale and still carry a key the remote already deleted; reading it would
judge a commit nobody approved. The declaration is read once, after the
fetch, and reclaim and the evidence provenance reuse that read. Because the
integration branch the release promotes is declared in that commit, a
release needs an explicit `--target <default branch>`; without one, `check`
and `run` refuse before the fetch:

```text
--release requires an explicit --target <default branch>
```

Integration into the integration branch keeps reading the working tree.

Release never reclaims: the integration ref matches neither the reclaim
guard's task patterns nor the integration/default names it protects.
