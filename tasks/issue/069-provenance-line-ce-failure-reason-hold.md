---
id: ISSUE-69
title: "Hold releases with the run provenance line until CE skips it as a failure reason"
type: bug
priority: P1
severity: medium
discovered-in: "review of the ISSUE-57/58 change"
discovered-at: 2026-10-20
created: 2026-10-20
---

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
