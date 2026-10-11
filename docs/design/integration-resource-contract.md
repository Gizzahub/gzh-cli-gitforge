# Integration resources and target ownership

Status: proposed public behavior contract. These guarantees are not implemented yet.

## Scope

Integration separates machine capacity from local Git mutation ownership. Only verification
explicitly declared high load acquires a machine resource slot, including high-load preparation.
Unclassified verification uses no resource slot. All required readiness gates still execute.
Fetch, Git inspection, push and reclaim run outside the resource slot.

Initial high-load capacity is one. A local target lock independently protects mutations for
the same resolved Git common directory and destination full ref. Different local targets may
run concurrently. Separate clones may still race remotely; exact push leases remain mandatory.

## Admission and ownership

Direct calls and adapter-managed calls participate in the same machine-local resource admission
queue. The resource provider allocates monotonically ordered tickets under a short admission
lock. The earliest live eligible ticket alone may attempt the OS resource lock. OS locking
enforces capacity; ticket order enforces FIFO. Neither lock is held while polling another lock.

Cancellation removes only the cancelling owner's ticket under the admission lock. A dead waiter
is skipped only after positive process-liveness evidence; elapsed time and holder display files
alone do not prove death. Recovery preserves the order of surviving tickets. Ambiguous ownership
remains visible and is not silently reclaimed.

Nested verification may inherit a parent's cooperative resource claim through explicit child
environment. This applies only to the resource slot: target ownership, required gates and exact
Git checks remain mandatory. Do not mutate the process-wide environment to propagate a claim.

## Git freshness and reporting

After acquiring target ownership and again before push, compare the approved source commit and
the remote destination commit to actual Git facts. A change requires revalidation; never rebase,
resolve conflicts or change the approved commit automatically. Push uses an exact destination
lease. Local tracking refs alone are not proof of the remote destination.

The default resource wait for new processes is 60 minutes per acquisition. Explicit execution
options override repository-root configuration, which overrides the default. Explicit zero
selects unbounded waiting. Invalid or negative limits fail before any verification runs.
Report the effective limit and its source, wait duration and verification duration separately.
Waiting consumes no verification timeout budget.

A successful remote push and reclaim are separate outcomes. Preserve the pushed commit when
local or remote cleanup fails. Retry cleanup only after confirming the actual remote result;
do not rerun integration to repair cleanup. Cancellation after launch is cancel-requested or
reconciling until the provider has stopped and the remote destination has been checked. An
uncertain outcome cannot become terminal cancelled solely because a probe saw no push yet.

## Compatibility and verification

Existing processes finish using their original behavior. Enable new resource classification
only after confirmed high-load verification paths have declarations. Do not delete existing
lock inodes, force-stop foreign processes or reclaim another session's worktree for migration.

Synthetic tests in isolated resource directories must demonstrate:

- Light integration completes while another repository performs high-load verification.
- High-load capacity stays at one and surviving waiters retain FIFO order.
- Concurrent same-target calls cannot duplicate push or reclaim.
- Source or destination movement requires revalidation without an automatic rebase.
- Crash, restart, cancellation and partial push preserve uncertainty until reconciled.
- Cleanup failure preserves successful push and recovery does not relaunch integration.

Public configuration keys and capability declarations for resource classification and target
ownership will be documented with their implementation. This proposal changes no CLI schema
and claims no current FIFO or recovery capability.
