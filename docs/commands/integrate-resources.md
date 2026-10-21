# Integration resource scope

`gz-git integrate check` and `gz-git integrate run` coordinate only git
mutations. gz-git runs no repository code, so there is no machine-capacity
slot: the host measurement slot, its FIFO admission, `HOST_SLOTS_HELD`, and
`branch.resourceHighLoad` / `branch.prepareHighLoad` are removed (see
[removed gate declarations](integrate-prepare-profile.md)).

## Locks

- **Task fence.** `integrate run` serializes native integration mutations of
  the same task branch, even when callers select different target refs or
  execution IDs. It is taken before target ownership, so lock order is
  consistent.
- **Target ownership.** Every check and run takes a target lock before it
  fetches. The key is the repository's Git common directory plus the
  destination `refs/heads/<branch>`. A run holds it through the exact lease
  push and reclaim, so two worktrees of one repository cannot both mutate one
  destination. The operation still rechecks source and destination before the
  push lease; it never rebases or resolves a conflict automatically.
  Unrelated task branches and other repositories have independent keys.
- **Execution run lock.** A run bound to an execution ID serializes the
  receipt of that execution.

A wait prints its reason (for example `waiting for target ownership of refs/heads/develop`) to the configured command error writer.

## Waiting

Each lock acquisition waits until the lock is free or the command is
interrupted; there is no wait budget. An interrupt before ownership exits 2
with nothing mutated. `--lock-wait` and `branch.lockWait` are removed (see
[removed gate declarations](integrate-prepare-profile.md)).

Run JSON keeps `resource_wait_duration_ms`, `validation_duration_ms`,
`plan_duration_ms` and `preparation_duration_ms` for schema stability; they
are always `0`.
