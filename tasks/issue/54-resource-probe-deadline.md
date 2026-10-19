# ISSUE: Remote observation can truncate resource admission to five seconds

- Status: resolved — source integrated and designated installer verified on mbp
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

## Delivery evidence (2026-10-18)

Independent strong review and finite/unbounded slow-probe, cancellation and
error-preservation regressions passed. Native delivery integrated and pushed
`a1c51c9f7164f3833ca5edd1efcae77d4f10dc96` to `master`; its DONE receipt confirms
worktree, local branch and remote branch removal. Native validation took
1,950,580ms with 460ms resource wait. This is the native integration verdict,
not a claim that every individual source/baseline check was green.

`make install` from that clean committed primary checkout passed the atomic
publication and selected-path audit. Installed `gz-git version` reports
`a1c51c9f`; SHA-256 is
`15fddbccd4ef3135f88d04d02400f2d89b4ad3a2551c86591d35b0feaa50bad5`.
Primary checkout evidence: `tmp/resource-handoff-finish.json`,
`tmp/resource-handoff-native-receipt.json`, `tmp/resource-handoff-install.log`
and `tmp/resource-handoff-installed.sha256`. This records mbp adoption only;
no other host or published release is asserted. ISSUE-55 remains out of scope.

## mst adoption gap observed (2026-10-19)

The existing source resolution and mbp installation verdict above remain unchanged.
On mst, `readiness-cleanup-safety-delivery` at `846b31e` was refused before either
make gate measured: resource admission ended with `context deadline exceeded`
after 22m15.878s. The active `~/go/bin/gz-git` build reports `abee75f2`, modified=false,
which predates this correction. This establishes a host adoption gap, not proof that
this specific timeout was caused by the five-second probe. No installer or PATH
change was performed. See [the delivery record](../../docs/task-reviews/readiness-cleanup-safety/delivery.md)
and its raw BLOCKED receipt. Verify mst's managed declaration, designated installer,
and resolved executable before another delivery attempt; do not reopen the corrected
source defect or duplicate it as a new issue solely from this observation.

## mst approved adoption (2026-10-19)

The user explicitly approved installing integrated source `e1a4bb5a` and retrying
the retained delivery. A clean CE experiment ran designated `make install` with
exit 0 and `install-path-audit: OK`. The resolved `~/go/bin/gz-git` has VCS revision
`e1a4bb5a5cf2dde41860ccd7e3b3b99238dae4d8`, modified=false; SHA-256 is
`09228f57ae8dfab486a5d0cbc05e4e2d7796ba3b5f0ecb0e1891847cdbbbf244`.
Version remains 0.10.4. [Installation evidence](../../docs/task-reviews/readiness-cleanup-safety/mst-adoption.json)
preserves the audit and build settings. No PATH, Homebrew or mise changes occurred.
This resolves the observed mst adoption gap; it does not retroactively identify
the earlier timeout cause or prove the pending cleanup-safety delivery complete.
