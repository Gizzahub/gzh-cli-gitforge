# ISSUE: Target observation can become stale before slot acquisition

- Status: open; source-derived finding, no runtime reproduction claimed
- Owner: `pkg/integrate`

## Finding

`acquireAdmission` returns when its ticket becomes first, and the OS-lock loop
breaks immediately when acquisition succeeds. Neither branch requires another
target observation. A target can therefore move after the last periodic probe
but before the holder releases the slot, allowing the gate to start before the
post-gate freshness check rejects the changed target.

Likewise, a transient fetch error currently warns and continues waiting. If the
slot becomes available immediately afterwards, a gate can start without a newly
confirmed remote target. Post-gate and pre-integration checks still apply; this
finding does not claim a stale merge or push was observed.

## Follow-up scope

Design a strict target refresh and exact identity comparison after actual slot
acquisition and before the gate starts. Unknown state must release the slot and
fail closed. Preserve acquisition budgets, cancellation and target locks.
Cover the FIFO/OS release window and final-refresh failure with gate markers.

This is separate from the observation-budget correction in
[ISSUE-54](54-resource-probe-deadline.md). No remediation is included there.
