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

This release uses the existing OS slot lock for capacity. It does **not** yet
provide FIFO ticket admission, inherited cooperative claims, or crash-recovery
queue semantics, so it is not the rollout point for fairness guarantees.
