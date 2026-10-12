# Integration resources and target ownership

## Implemented scope

Integration separates local Git target ownership from host capacity. A target lock
keys the Git common directory and fully qualified destination ref. It is acquired
before fetch and is retained by `integrate run` through its exact-lease push and
reclaim. Source and destination are revalidated before the push; the command does
not rebase or resolve conflicts automatically.

Host capacity is opt-in through repository-root `branch.resourceHighLoad: true`.
An omitted or false declaration leaves mandatory verification running without a
host slot. A high-load source or target declaration uses the shared slot only for
the gate subprocesses; fetch, Git inspection, preparation materialization,
cleanup, push, and reclaim remain outside it.

Capacity is one. Participating high-load calls receive monotonic FIFO tickets
before attempting the existing OS lock. Ticket writes are atomic, cancellation
removes its own ticket, and only positively dead processes are recovered.
Ambiguous queue state fails closed. Existing callers that do not participate in
admission still contend on the OS lock.

## Wait limits

The wait limit applies per slot acquisition and covers FIFO admission plus the
final OS lock attempt. `--lock-wait` overrides repository-root
`branch.lockWait`, which overrides the 60 minute default. Explicit zero means
unbounded. Repository declarations accept Go duration strings, `"0"`, or
`unbounded`. Only repository-root declarations are authoritative; non-root
declarations are ignored and are not inherited. Malformed, merged, aliased,
duplicate, numeric, null, or negative root declarations fail closed.
`CheckReport` records resource waiting separately from verification duration.

## Current boundaries

Resource claims are not inherited by child processes. Preparation profiles do
not yet have an independent high-load classifier. Windows supports lock release
and process-liveness checks, but directory fsync is unavailable and native crash
durability has not been verified.
