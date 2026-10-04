# Release promotion from the remote integration ref

`gz-git integrate check|run <remote>/<integration> --target <remote>/<default> --release`
promotes the integration branch onto the default branch. A caller such as
CE `ce task run-release` runs it from the repository's primary checkout,
which stays on the default branch, and names the remote-tracking ref
(`origin/develop`) as the source.

That source is the remote's state by definition. It is never checked out and
has no upstream, so the task-branch rows are judged differently:

| Row            | Task branch or local `develop` source  | Remote-tracking release source               |
| -------------- | -------------------------------------- | -------------------------------------------- |
| `working-tree` | HEAD must be the branch, tree clean    | HEAD is not compared; the checkout stays clean |
| `push`         | upstream must equal the branch         | PASS: the source is the remote ref           |
| legacy `make`  | measured where HEAD is                 | measured in a detached worktree at the source SHA |

The checkout must still be clean because `run` fast-forwards the local
default-branch worktree after the push. A local `develop` source keeps the
task-branch rules: HEAD must be that branch.

Without a preparation profile, the source worktree is pristine — it carries
none of the live checkout's `node_modules/`, `.venv`, or build output. A
repository whose gate needs installed dependencies declares a profile such as
`pnpm-frozen-lockfile-v1` ([preparation profiles](integrate-prepare-profile.md)),
which then prepares both the target and the source commit alike.

Release never reclaims: the integration ref matches neither the reclaim
guard's task patterns nor the integration/default names it protects.
