# Quality baseline repair

This task follows sibling-provision source delivery `45935984` and is rebased on
`f012c9f8599f3d5de9ff3ab6b2dce799ad541cba`. It records the clean installed CE
reference `7d2bebecf95a8f7bbe5bc21112c21868173c4a23`. Concurrent source adoption
`3547349` advanced Go to 1.27.1 and lint to 2.14.0; those declarations are retained
unchanged. No workstation pin is rolled back and no product version is raised.

## Behavioral scope

- Bare finish selects only the current owner's ACTIVE delivery; zero or multiple
  candidates fail closed. Experiments and foreign executions cannot be selected.
- Status/list preserve externally reclaimed ACTIVE history as BLOCKED without
  rewriting execution records or start receipts (CE ADR-0073).
- Stable Worktrunk versions from 0.80 onward are accepted while the inventory
  schema remains separately checked. Malformed/prerelease reports are rejected.
- Golden provenance requires a clean, full CE revision from Go build metadata.
  Twelve fixtures were captured from the reference executable; semantic guards
  cover single/ambiguous bare finish and observational reclaimed history.
- Admission test synchronization now uses explicit child handshakes rather than
  an incidental startup deadline. The finite FIFO+OS-lock budget assertion and
  production admission code remain intact.

## Verification and limits

Runtime/admission regressions and all twelve CE-reference parity scenarios
pass. Independent actual-diff reviews found no blockers in the runtime changes,
provenance checks, or admission failure-path tests. The pre-rebase candidate's
gosec reported zero issues and govulncheck reported no vulnerabilities under Go
1.26.9. These results do not establish security acceptance for Go 1.27.1.

Two initial broad runs hit the nearly full workstation Data volume. Subsequent
validation uses a task-owned temporary/build cache and log on the separate cache
volume. Archive/filter regressions passed 40 isolated process-filter repetitions
and 20 coverage-enabled archive repetitions; no archive product code or security
assertion was changed in response to a truncated SIGPIPE log.

The external-volume rerun exposed ancestor mise config discovery despite isolated
HOME/XDG. Fixture-only `MISE_CEILING_PATHS` now stops parent config discovery at
the sandbox root. It does not trust host configuration or change executable PATH;
the observed provider and Worktrunk versions remain 0.9.1 and 0.80.0.

Before source freshness changed, `make quality-check` exited zero on 2026-10-13
with Go 1.26.9, external TMPDIR/GOCACHE and log storage. Its source-non-mutating
workspace, format, unlimited lint, security,
private build, install audit, coverage-enabled unit, integration, E2E and critical
coverage stages all passed. Critical coverage was 59.6% / 59.3% / 61.0% / 65.4%
for gitcmd / repository / reposync / workspacecli, respectively; no floor changed.
This historical pass is not reused as a whole-gate verdict after the rebase.
Upstream `3547349` records nine reachable Go 1.27.1 standard-library findings;
current-source verification and comparison against the same baseline are pending.
Installation and real consumer adoption are separate from source validation.
Full target-mode port parity (purpose, resume, provider capabilities and receipt
fields) remains tracked by ISSUE-42; passing CE-reference goldens does not close it.
