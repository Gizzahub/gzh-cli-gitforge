---
id: ISSUE-69
title: "Hold releases with the run provenance line until CE skips it as a failure reason"
type: bug
priority: P1
severity: medium
discovered-in: "review of the ISSUE-57/58 change"
discovered-at: 2026-10-20
created: 2026-10-20
resolution: fixed
resolved-at: 2026-10-22T06:55:38Z
resolution-summary: "Resolved as fixed."
status: done
---

## Update (2026-10-22)

The release hold did not hold in practice: tags v0.11.0 and v0.11.1 shipped
with the provenance line (`pkg/integrate/device.go:119`,
`integrate: run device=...`). The fix belongs to ce-agent-kit
(`finish_reason.go`); close this card once that fix is installed and its
commit is cited.

## Re-check (2026-10-22)

The CE fix has not landed, and no ce-agent-kit card tracks it (ce-agent-kit
master `a8a63782`, installed ce 0.10.18):

- `failingCheckReason` in `internal/usecase/taskruntime/finish_reason.go`
  still falls back to the first non-empty stderr line. It has not changed
  since this card was written; it names a BLOCKED run that is not stale and
  whose reason is still "integration did not complete".
- gz-git master `86f02984` still prints the line first (built at
  `pkg/integrate/device.go:119`, written to stderr by
  `cmd/gz-git/cmd/integrate.go:54`), and `pkg/integrate/check_target.go:331`
  still refuses a non-integration target without a `FAIL ` line.
- v0.12.0 also shipped with the line, so the release hold no longer holds
  anything back. What is left is registering and making the CE fix in a
  ce-agent-kit session.
- ISSUE-93's resolution says ISSUE-069 is closed; this card is still open.

## Summary

ISSUE-57 makes `integrate check|run|cleanup` and
`integrate readiness update review|auto|run` print
`integrate: run device=<label> gz-git=<version>` as their first stderr
line. ISSUE-58 added the hint line that follows it when the declared
device is invalid; an undeclared device gets one too.

ce-agent-kit `internal/usecase/taskruntime/finish_reason.go` picks the
recorded failure reason in this order: a stdout `FAIL ` line, a stderr
`FAIL ` line, then the first non-empty stderr line. A failed integrate
run whose output has no `FAIL ` line, such as a `--target` that is not the integration branch (exit 1,
"target must be the integration branch ..."), now records the
provenance line as its one-line reason instead of the refusal. With an
uncontended target lock no other notice precedes that refusal. Where a notice already came first, such
as the host lock wait notice, the provenance line replaces that notice. CE
handles a stale base, exit 3 and exit 4 itself and is not affected
there. The full stderr stays in the receipt diagnostics, so only the
summary misleads.

## Reproduction

1. Build gz-git from a commit that contains the ISSUE-57 change.
1. Run `ce task run-finish` on a task whose `--target` gz-git refuses,
   for example after `branch.integrationBranch` in `.gz-git.yaml` has
   drifted from the run's recorded source branch (a non-default branch;
   a default-branch source is refused with "promoting onto the default
   branch requires --release" instead).
1. The recorded reason reads
   `integrate-run failed: integrate: run device=...` rather than
   `target must be the integration branch ...`.

## Expected vs Actual

- Expected: the finish reason names the failure.
- Actual: it names the device and build.

## Scope

- The fix belongs to ce-agent-kit: skip the provenance and hint lines
  when choosing the reason (or prefer the last non-empty stderr line).
  It is registered and made in a ce-agent-kit session; record its
  issue ID here once registered.
- Hold every release path in this repository while master contains the
  ISSUE-57 change and the CE fix is not installed: no manual `v*.*.*`
  tag push, and no `VERSION` bump once the ISSUE-56 workflow (which
  releases on a bump) is on master. That includes ISSUE-56's first
  automated release; whichever of ISSUE-56 and ISSUE-57 lands second
  adds the back-reference to this card in card 056.
- Landing on master before the CE fix is accepted: DevBox sessions run
  `gz-git` from `tmp/bin` and other devices that `make install` from
  master get the line on their next rebuild, but the cost is one
  misleading summary line with the cause still in the diagnostics. The
  hold keeps it out of the Homebrew channel.
- Close this card when the CE fix is installed on the devices that run
  `ce task run-finish`, citing the CE issue and commit.

## Resolution (2026-10-22)

Fixed in ce-agent-kit as ISSUE-460 (master `effd3181`, fix commits
`f5d86cd6` and `3a1d30c1`). `failingCheckReason` now skips the provenance
line, both device hint lines, the wait notices (`integrate: waiting `) and
non-fatal warnings (`integrate: warning: `) before it falls back to a stderr
line, so a refusal without a `FAIL ` line is recorded as the refusal. ce
0.10.20 built from `effd3181` is installed on mst with `make install_cli`.
Other devices that run `ce task run-finish` pick it up on their next CE
install. The release hold is lifted.
