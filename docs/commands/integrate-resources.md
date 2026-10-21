# Integration resource scope

`gz-git integrate check` and `gz-git integrate run` coordinate only git
mutations. gz-git runs no repository code, so there is no machine-capacity
slot: the host measurement slot, its FIFO admission, `HOST_SLOTS_HELD`, and
`branch.resourceHighLoad` / `branch.prepareHighLoad` are removed (see
[removed gate declarations](integrate-prepare-profile.md)).

## No waiting

gz-git never makes one integration wait for another. It detects a conflict
or a stale target and reports it; deciding whether and when to retry belongs
to the caller.

Two runs on one target are not ordered. The exact-lease push lets one land;
every other run refuses at once and names the target ref, the commit it
checked, the current commit, and the action:

```text
target branch changed before the push: origin/develop is 4f1c…, readiness checked 9a0e…; rebase onto origin/develop and re-run check (git push: …)
```

A run that starts after the other landed reports it earlier, as a failed
`freshness` row (`1 commits behind origin/develop — git rebase origin/develop`)
or a failed revalidation:
`target branch changed during readiness: origin/develop is 4f1c…, readiness checked 9a0e…; rebase onto origin/develop and re-run check`.
With `--expect-target` both refusals are a target mismatch (exit 4). The
revalidation says `target origin/develop is 4f1c…, expected 9a0e…`; only the
lease refusal adds `; rebase onto origin/develop and re-run check`.

## Local fences

Only gz-git's own on-disk state is fenced, and a held fence is refused
immediately, never awaited:

- **Task fence.** `integrate run` reclaim, `integrate cleanup`, and legacy
  cleanup of the same task branch delete the same worktree and branches, so
  only one runs at a time, even across target refs and execution IDs.
- **Execution run lock.** Only one invocation per execution ID runs at a
  time.

A refused command exits 2 with `integration resource is busy`, the fenced
resource, and the holder, for example
`task resource fence for dev/actor/feat/x is held by another process (pid=4242 since=2026-10-21T03:00:00Z); gz-git does not wait for it, retry after that process finishes`.

Receipt files are still written under a short internal lock with a revision
CAS; that lock covers one file write and is not a command-level wait.

`--lock-wait` and `branch.lockWait` are removed (see
[removed gate declarations](integrate-prepare-profile.md)). Run JSON keeps
`resource_wait_duration_ms`, `validation_duration_ms`, `plan_duration_ms` and
`preparation_duration_ms` for schema stability; they are always `0`.
