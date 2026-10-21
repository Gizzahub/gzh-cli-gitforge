# Integration resources: detect and report, never wait

> **Status (2026-10-21).** The readiness contract (`contract-v1`) is removed,
> and with it everything this document once specified about host capacity:
> the host measurement slot, FIFO admission tickets, `HOST_SLOTS_HELD`,
> `branch.resourceHighLoad`, `branch.prepareHighLoad`, `resourcePlan: plan-v1`,
> and the target-ref refresh during a slot wait. gz-git runs no repository
> code, so it has no capacity to meter. Those keys now fail to load; see
> [removed gate declarations](../commands/integrate-prepare-profile.md).
>
> **Status (2026-10-21, ISSUE-93).** The target lock is removed. gz-git no
> longer makes one integration wait for another; ordering integrations of one
> target belongs to the caller.

## Contract

gz-git detects and reports conflicts and stale targets. It does not order
integrations, and no command waits for another command.

- **Concurrent runs on one target** are not serialized. Each run checks
  readiness against the target tip it fetched, revalidates source and target
  after the checks, and pushes with an exact lease
  (`--force-with-lease=refs/heads/<target>:<checked commit>`). Exactly one
  run can land on a given tip. Every other run refuses at once, at whichever
  step first sees the moved target:
  - freshness: `N commits behind <target> — git rebase <target>`;
  - revalidation: `target branch changed during readiness: <ref> is <current>, readiness checked <checked>; rebase onto <ref> and re-run check`;
  - the lease push: `target branch changed before the push: <ref> is <current>, readiness checked <checked>; rebase onto <ref> and re-run check (git push: …)`.
    When the push endpoint cannot be read back, the current commit is
    reported as unknown instead of guessed.
- **With `--expect-target`** the revalidation and lease refusals are
  `ErrTargetMismatch` (exit 4):
  `target <ref> is <current>, expected <checked>`; the lease refusal adds
  `; rebase onto <ref> and re-run check`. That classification is granted only
  from a sole porcelain stale-lease record proven against the exact push
  endpoint; a target already equal to the source stays a generic push
  failure, because the push may have landed.
- The command never rebases and never resolves a conflict.

## Local state gz-git still protects

Only gz-git's own on-disk state keeps mutual exclusion, and every exclusion
that can be held across git commands is a non-blocking try-lock: the second
command refuses with `ErrResourceBusy` (exit 2), names the holder's recorded
`pid=… since=…`, and exits.

- **Task fence** (`<git-common-dir>/gz-git-integrate-targets/<sha256("task-resource:"+branch)>.lock`).
  `integrate run` reclaim, `integrate cleanup`, and legacy cleanup apply of
  one task branch delete the same worktree and branches, so two of them must
  not run at once, even with different target refs or execution IDs. The
  directory is the one the removed target lock used, so an older gz-git
  contends on the same file.
- **Execution run lock** (`<receipt>.run`). Two invocations of one execution
  ID must not run at once, so one cannot record a pre-push refusal while the
  other reaches intent or pushes.
- **Receipt write lock.** It covers only the read-compare-write of a receipt
  or outcome file, takes no other lock while held, and therefore stays a
  short blocking acquisition; the receipt revision is the CAS.

Cleanup re-reads its receipt after taking the task fence and refuses with
`ErrEvidenceReplay` when the target ref or any immutable field changed since
its first read.

## Retired

- The `target_lock_wait` pre-mutation phase is read-only. Outcome documents
  that carry it still validate; no run writes it. A complete execution
  context now records a pre-mutation outcome only for `readiness_rejected`.
- `CheckOptions.LockNotice`, `RunOptions.LockNotice` (embedded) and
  `CleanupOptions.LockNotice` are removed: nothing waits, so nothing prints a
  wait notice.
- `--lock-wait` and `branch.lockWait`, which bounded only the removed host
  slot, are removed keys. Run JSON keeps `resource_wait_duration_ms` at `0`
  for schema stability.

## Current boundaries

The task fence is per repository and task branch; other repositories and
unrelated task branches have independent keys. On Windows the holder record
may be unreadable while the lock is held, and the refusal then names "an
unidentified process". Windows supports lock release and process-liveness
checks, but directory fsync is unavailable and native crash durability has
not been verified.
