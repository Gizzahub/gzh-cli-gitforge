# ISSUE: Remote observation can truncate resource admission to five seconds

- Status: source correction implemented; release/adoption not asserted
- Owner: `pkg/integrate`

## Reproduction

A high-load check waits in FIFO admission or on the host slot with the default
one-hour acquisition budget. Its target observer performs remote Git operations.
When these take longer than five seconds, `observeResourceWait` cancels its child
context and propagates `context deadline exceeded` as resource unavailability.
The same early failure affects an explicitly unbounded acquisition.

`TestResourceWaitSlowProbeUsesAcquisitionBudget` reproduces both cases using a
healthy six-second observation before releasing the holder. The original code
fails both cases at roughly five seconds. Existing finite-budget, cancellation
and target-movement regressions remain required.

## Correction

Use the remaining acquisition budget for a finite observation and the parent
context for an explicitly unbounded observation. Preserve every error returned
by the observer. This change does not alter the observer's existing transient
fetch warning/continue semantics, tracked in ISSUE-55 below. Git commands retain
their executor timeout and parent cancellation.

This removes only the artificial five-second limit. It does not add a final
freshness check after taking the slot; that existing gap is tracked separately
in [ISSUE-55](55-resource-slot-release-window.md).

Installation, release and consumer adoption require separate evidence.
