# Integration resource scope

`gz-git integrate check` and `gz-git integrate run` have two independent
local coordination scopes.

Every check/run obtains a target lock before it fetches. The lock key is the
repository's Git common directory plus the destination `refs/heads/<branch>`.
It is held through an `integrate run` push and reclaim, so two worktrees of
the same repository cannot both mutate one destination. The operation still
rechecks its source and destination before the exact push lease; it never
rebases or resolves a conflict automatically.

Machine capacity is opt-in. A repository can declare the checked gate high
load in its committed repository-root `.gz-git.yaml`:

```yaml
branch:
  resourceHighLoad: true
```

The declaration must be a boolean. `false` or an omitted key leaves the gate
light: the required gate still runs, without taking the host measurement slot.
gz-git reads both committed source and target declarations; either `true`
uses the slot. A working-tree edit cannot disable that protection.

For a high-load legacy gate, each actual source or baseline `make check`/`make lint` subprocess holds the slot and releases it before the next probe. For a
readiness contract it covers the runner process only. Fetching, Git
inspection, snapshotting and prepared-worktree creation/removal, push, and reclaim are
outside the slot. Capacity remains one per host.

`CheckReport.ResourceWaitDuration` records slot admission time separately from
`ReadinessDuration`; waiting does not consume a runner timeout. A target-lock
wait prints its common-directory and destination-ref ownership reason to the
configured command error writer.

Readiness V1 defaults to a 15-minute runner execution budget. The committed
repository-root `branch.readiness.timeout` can declare a positive Go duration
up to `2h`, for example `45m`. This target-owned value is part of the readiness
contract digest; source-only changes fail closed and must use the human-operated
readiness update transaction. Preparation and resource waiting are outside this
budget. There is no CLI, environment, profile, or global override. Empty, null,
non-string, whitespace-padded, invalid, zero, negative, and excessive values are
rejected. Omitting the field retains the original canonical contract and digest.

Each high-load admission and final OS-slot attempt shares one wait budget.
`--lock-wait` has highest precedence, followed by the repository-root
`branch.lockWait`, then the 60 minute default. The flag or declaration accepts
Go duration strings; an explicit `0` flag, `"0"` declaration, or
`unbounded` declaration waits without a limit. `branch.lockWait` is strict:
it is a non-empty string in the repository-root document only, and duplicate,
alias, merge, null, numeric, or negative forms fail closed. It is never
inherited from a profile, parent directory, or workspace configuration.

When this budget expires, check and run exit 2 because the declared gate could
not be measured. The readiness report and resource-wait duration remain visible;
an ordinary completed NOT READY verdict still exits 1. Run JSON retains schema v1
and reports `exit_code: 2`, without claiming push or reclaim. A bound execution
records the existing `readiness_rejected` pre-mutation outcome; a run without an
execution context does not invent one. The original typed expiry remains in the
error chain when recording the refusal or another check step also fails.

While a high-load request waits in FIFO admission or for the OS slot, it
refreshes the captured remote target ref at the holder-notice interval. If the
target moves, the request returns NOT READY with a re-run-check message and
elapsed resource wait, before starting verification. It does not rebase or
change the measured source commit. A transient fetch failure is reported once
and the wait continues within its original wait budget and cancellation context.

Declared high-load gates receive FIFO admission tickets before trying the existing
OS slot lock. Sequence allocation and ticket publication are serialized and use
atomic replacement. A cancelled request removes its own ticket; recovery removes
a ticket only when its process is positively proven dead. Ambiguous state fails
closed. FIFO admission and the final OS lock share one acquisition wait budget.

The OS slot lock remains the capacity authority, including alongside older
callers. FIFO ordering applies to participating callers; older processes finish
through their original path. Windows builds support process liveness and lock
release, but directory fsync is unavailable and native Windows crash-durability
has not been verified.

A gate process holding the host slot exports `HOST_SLOTS_HELD` to its children.
The value is a cooperative comma-separated list; existing names are preserved
and `integrate` is added once. An inherited `integrate` claim lets a nested
high-load gate run under its parent's slot without publishing a child FIFO ticket,
while leaving the parent's OS lock and holder sidecar untouched. The child receives a notice that its
measurement uses the parent's lock. This claim is a cooperative participant
contract; gz-git does not authenticate it.

Git target locks still apply to each child. Nested checks of another repository
have a distinct common-directory/target key. A child targeting the same local
repository and branch waits for that target owner; host-slot inheritance does
not grant Git mutation ownership.

A repository can independently classify its resolved preparation profile with
committed repository-root `branch.prepareHighLoad: true`. Either source or target
being true acquires the same slot separately for each actual source or lazy
baseline profile execution, then releases it before Make verification. The slot
covers the executor and its integrity checks; snapshotting, worktree creation,
and worktree cleanup remain outside. This declaration does not make Make gates
high load. It also applies to an existing controller-only profile, but does not
create a profile when none is resolved. Readiness runners do not execute legacy
preparation and therefore do not acquire a preparation slot.

The key must be a strict boolean and is not inherited from profiles, parents, or
workspace configuration. Malformed committed declarations fail closed even when
no preparation executes. A manifest declaring a profile or `prepareHighLoad: true` is limited to
64 KiB. Profile children receive the same preserved
`HOST_SLOTS_HELD` list as gate children. Preparation admission uses the existing
FIFO, cancellation, target-movement detection, and per-acquisition wait budget;
its waiting contributes to `ResourceWaitDuration`. Confirmed heavy paths must
commit their appropriate declaration before rollout.
