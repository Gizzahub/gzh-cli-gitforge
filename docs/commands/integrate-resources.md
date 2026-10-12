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
inspection, preparation materialization and cleanup, push, and reclaim are
outside the slot. A preparation profile that itself becomes a confirmed
high-load executable needs a separate narrow declaration before rollout;
this key does not classify it yet. Capacity remains one per host.

`CheckReport.ResourceWaitDuration` records slot admission time separately from
`ReadinessDuration`; waiting does not consume a runner timeout. A target-lock
wait prints its common-directory and destination-ref ownership reason to the
configured command error writer.

Each high-load admission and final OS-slot attempt shares one wait budget.
`--lock-wait` has highest precedence, followed by the repository-root
`branch.lockWait`, then the 60 minute default. The flag or declaration accepts
Go duration strings; an explicit `0` flag, `"0"` declaration, or
`unbounded` declaration waits without a limit. `branch.lockWait` is strict:
it is a non-empty string in the repository-root document only, and duplicate,
alias, merge, null, numeric, or negative forms fail closed. It is never
inherited from a profile, parent directory, or workspace configuration.

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

Preparation profiles currently execute outside the acquired gate slot, so they
receive no newly manufactured held-slot claim. Independent high-load preparation
classification and real high-load declarations remain rollout prerequisites.
