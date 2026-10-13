# Quality baseline repair

This task follows sibling-provision source delivery `45935984` and is rebased on
`14f6d890294bb180b9dddc408cd4cd04d82e969b`. It records the clean installed CE
reference `cd0e1d19c3211b85a514ac03383ae6166a614f07`. Concurrent source adoption
`3547349` advanced Go to 1.27.1 and lint to 2.14.0. The lint declaration is retained;
Go advances to the reported fixed patch 1.27.2 after managed-runtime verification.
No workstation pin is rolled back and no product version is raised.

## Adoption blocker (2026-10-14)

The updated user policy requires explicit selection before adopting an unreleased
development tool. The tested gosec pseudo-version below is a candidate only:
this task has not been integrated or installed. Independent technical review is
compatibility evidence, not authorization for that policy exception.

The latest official stable remains [v2.29.0](https://github.com/securego/gosec/releases/tag/v2.29.0).
Its [x/tools v0.49.0 declaration](https://raw.githubusercontent.com/securego/gosec/v2.29.0/go.mod)
matches the reproduced Go 1.27.2 export-format failure. Integration and the
dependent EE native sibling-adoption proof wait for either an explicit choice of
`v2.29.1-0.20261009120814-7b1b5cebe007` or a compatible official stable release.
Existing successful candidate checks do not establish delivery completion.

## Behavioral scope

- Bare finish selects only the current owner's ACTIVE delivery; zero or multiple
  candidates fail closed. Experiments and foreign executions cannot be selected.
- Status/list preserve externally reclaimed ACTIVE history as BLOCKED without
  rewriting execution records or start receipts (CE ADR-0073).
- The concurrent owning-source contract accepts exactly the installed Worktrunk
  0.80.0 report, while the inventory schema remains separately checked.
  Later releases require separate compatibility evidence before adoption.
- Golden provenance requires a clean, full CE revision from Go build metadata.
  Twelve fixtures were captured from the reference executable; semantic guards
  cover single/ambiguous bare finish and observational reclaimed history.
- Admission test synchronization now uses explicit child handshakes rather than
  an incidental startup deadline. The finite FIFO+OS-lock budget assertion and
  production admission code remain intact.

## Verification and limits

Pre-rebase runtime/admission regressions and all twelve CE-reference parity
scenarios passed against clean CE `7d2bebec`. Independent actual-diff reviews
found no blockers in the runtime changes,
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
The rebased candidate and an isolated archive of exact baseline `f012c9f` both report
the same nine reachable Go 1.27.1 standard-library findings (6603, 6604, 6605,
6607, 6608, 6610, 6611, 6613, 6617); the task adds no ID. Their reported fixed
patch is Go 1.27.2. The canonical workstation owner's source and deployed snapshot
both declare Go `1.27` (latest patch), agree at revision `0de2fe64892addc3f45f8492f6dc589c0cbaa3ae`,
and need no source edit. Official `mise upgrade go --no-prune` installed 1.27.2;
source Git remained clean and both the owner and candidate resolve Go 1.27.2.
The concurrent dependency update required one additional generated x/tools
module checksum in go.work.sum. It was produced by the gate's isolated
`go list -mod=readonly -m all`; regeneration changed no dependency requirements,
go.sum entries, or language floor.
Current runtask/admission focused regressions pass. At an intermediate checkpoint,
the installed CE advanced
independently to clean `a188a503`; source/installed identity agrees, and its three
commits since `7d2bebec` change no task-runtime or task CLI lifecycle code. The
exact reference was advanced and all twelve fixtures recaptured without changing
their semantic guards. Current runtask and parity verification passed (0.579s /
33.680s); stamped receipt provenance is the only additional fixture difference.
Format, lint 2.14.0, gosec and private build also passed under Go 1.27.1.
The first Go 1.27.2 run exposed stale Go 1.26.9-built gosec: type import errors
caused exit 1 despite zero reported findings. `security-code` and `security-json`
now depend on the existing pinned `install-gosec` identity guard before scanning.
Rebuilding stable gosec v2.29.0 under Go 1.27.2 still fails: its x/tools v0.49.0
decoder cannot read export format 5. The official upstream compatibility commit
`7b1b5cebe007d62fb58eacb90fc571112939ec30` resolves to immutable module version
`v2.29.1-0.20261009120814-7b1b5cebe007` and supports Go 1.27.2 via x/tools v0.51.0.
Independent review validated this candidate after verifying official source and
CI; it is 32 upstream commits beyond stable v2.29.0, not a stable release or a
local fork. Its actual full repository scan has zero findings and type errors.
The candidate pin stays exact; a future stable successor must be verified separately.
Govulncheck v1.1.4 rebuilt under Go 1.27.2 reports zero called vulnerabilities.
Current Go 1.27.2 runtask/parity also pass (0.973s / 51.008s). The final canonical
`make quality-check` completed with exit zero under Go 1.27.2 and CE cd0e1d19
(`quality-go1272-cd0.log`). Unit, integration, E2E and critical coverage stages
passed; coverage remains 59.6% / 59.3% / 61.0% / 65.4%, with no floor change.
An earlier coverage unit run passed pkg/integrate in 959.955s, including the
held-slot budget regression. Its hold now starts on the actual waiting notice;
three focused repetitions also passed. The coverage-heavy canonical unit command
has a bounded 20-minute package timeout instead of Go's default 10 minutes.
That whole run stopped at the parity guard because the installed CE advanced to
clean `cd0e1d19`. This advance changes lifecycle source (offline observation and
global worktree defaults); it is not justified as a source stamp-only update.
All twelve scenarios were recaptured (39.611s), and independent structured review,
including JSON embedded in command stdout, found no status, reason, action, owner,
worktree, cleanup or diagnostic behavioral changes. Only provenance and stamped
receipt versions changed. Offline/global-default behavior and full port parity
remain outside this bounded evidence.
Both final scanner contract modes passed. The Go 1.26.9/1.27.2 build-and-scan
matrix has four zero-finding cells. The explicit full comparison reported
baseline make=2/scanner=1/findings=3 and current make=0/scanner=0/findings=0:
new findings=0 and resolved findings=3. Make's wrapped failure is accepted only
when valid scanner JSON reports positive findings and its count matches Make's
summary; missing JSON, type errors, unrelated Make failures, and masked findings
still fail. These fixture compilers test the repository's declared language floor;
the installed default stays Go 1.27.2. No issue receipt was rewritten automatically.
Installation and real consumer adoption are separate from source validation.
The first integration attempt after this verification stopped on source freshness:
14f6d89 added the exact Worktrunk contract and tests, lease classification, and
the already-generated workspace checksum. Rebase conflicts were limited to the
Worktrunk declaration and doctor comparison. Independent review chose to retain
the upstream exact gate and remove the task's now-redundant version-floor helper;
upstream diagnostics and real installed-Worktrunk lifecycle tests are preserved.
Rebased source validation passed: the non-short runtask suite (20.944s) includes
the real Worktrunk fixture, and parity passed in 54.762s. The current canonical
`make quality-check` exited zero (`quality-rebased-14f6.log`); pkg/integrate passed
in 1009.570s with coverage 78.0%, and every critical coverage floor is unchanged.
The earlier full gate is not reused as a verdict for this changed source graph.
Full target-mode port parity (purpose, resume, provider capabilities and receipt
fields) remains tracked by ISSUE-42; passing CE-reference goldens does not close it.
