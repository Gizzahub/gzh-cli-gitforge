# Refresh a stale task branch under the target lock

`gz-git integrate run --refresh-under-target-lock` lets a run that already
holds the target lock rebase a stale task branch onto the target tip,
instead of refusing it as `stale-base` and sending the task session back to
rebase and queue again. The decision is ce-devenv ADR-0003.

Both sides must opt in:

- the caller passes `--refresh-under-target-lock`, and
- the **target commit** declares it in its committed `.gz-git.yaml`
  (or `.gz-git.yml` / `.gz-git.json`):

```yaml
branch:
  refreshUnderTargetLock: true
```

The declaration is read from the target commit only. A task branch that
adds it to its own `.gz-git.yaml` is ignored, so a branch cannot grant
itself permission to be rewritten. The value must be a literal boolean in
one document, with no duplicate key, YAML merge, or alias; anything else
fails the run. Parent and profile configuration never carry it.

## What a refresh does

When the check fails only because the task branch is behind the target, the
run rebases the task branch in its linked worktree, lease-pushes it, and
re-runs readiness against the rebased commit. The run output starts with
one `REFRESHED <branch> <before> -> <after> onto <target> (<reason>)` line
per refresh, and the JSON protocol and receipt carry `refreshes` and
`task_sha_before_refresh`.

When the target moves again while gates run, the run refreshes again, at
most twice. A third move refuses as before.

## Refusals

The refresh never resolves anything on the task owner's behalf. Each of
these refuses with `not ready: refresh under target lock refused` and
leaves the local and remote task branch at the checked commit:

- the branch is checked out in the primary checkout or in no worktree;
- the worktree HEAD is not the checked source, or the worktree is dirty or
  has a rebase, merge, cherry-pick, revert, or bisect in progress;
- the branch does not track `<remote>/<branch>` or is not pushed;
- the branch contains a merge commit;
- the rebase conflicts or empties a commit;
- the lease push loses a race.

A failure other than a stale base or a target that moved after every gate
passed is not refreshed.

## Incompatible flags

`--expect-target` pins the target the caller reviewed, `--expect-source`
pins the task revision the caller approved, and `--release` promotes an
integration ref rather than a task branch. Combining any of them with
`--refresh-under-target-lock` exits 2 before the engine runs.

A run bound to a native execution context also lands only its approved
source revision, so it never refreshes: a stale base is refused exactly as
without the opt-in.
