# Integration resources and target ownership

> **Status (2026-10-21).** The readiness contract (`contract-v1`) is removed,
> and with it everything this document once specified about host capacity:
> the host measurement slot, FIFO admission tickets, `HOST_SLOTS_HELD`,
> `branch.resourceHighLoad`, `branch.prepareHighLoad`, `resourcePlan: plan-v1`,
> and the target-ref refresh during a slot wait. gz-git runs no repository
> code, so it has no capacity to meter. Those keys now fail to load; see
> [removed gate declarations](../commands/integrate-prepare-profile.md). What
> remains is git-mutation ownership, below.

## Implemented scope

A target lock keys the Git common directory and fully qualified destination
ref. It is acquired before fetch and is retained by `integrate run` through its
exact-lease push and reclaim. Source and destination are revalidated before the
push; the command does not rebase or resolve conflicts automatically.

A task fence serializes native integration mutations of one task branch across
target refs and execution IDs, and is always acquired before target ownership.
An execution run lock serializes the receipt of one execution ID.

## Waiting

Each acquisition of the task fence, the target lock, and the execution run
lock polls until the lock is free or the command context ends. An interrupt
before ownership mutates nothing. `--lock-wait` and `branch.lockWait`, which
bounded only the removed host slot, are removed keys. Run JSON keeps
`resource_wait_duration_ms` at `0` for schema stability.

## Current boundaries

Git target ownership is per repository and ref; nested checks of another
repository have a distinct key, and a child targeting the same repository and
branch waits for that owner. Windows supports lock release and
process-liveness checks, but directory fsync is unavailable and native crash
durability has not been verified.
