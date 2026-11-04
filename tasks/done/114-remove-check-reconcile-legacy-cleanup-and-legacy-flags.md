---
id: TASK-114
title: "Remove integrate check, reconcile, legacy-cleanup and the legacy flags (gz-git v0.15.0 surface)"
type: feature
priority: P2
effort: L
exec-tier: strong
created: 2026-11-04
status: done
completion-summary: "integrate check/reconcile/legacy-cleanup 하위명령과 pre-mutation 기계, 레거시 플래그 전부를 제거해 v0.15.0 최종 표면(run/cleanup/receipt/queue/provision)만 남겼다. TASK-113 하위 발견 2건도 접었다."
quality-review: pass
quality-reviewed-at: "2026-11-04T11:10:13Z"
quality-review-evidence: docs/task-reviews/TASK-114/done-review.md
verification-status: passed
verification-evidence: docs/task-reviews/TASK-114/execution-evidence.md
---

## Summary

gz-git v0.15.0 lands the ADR-0161 final surface on this repository's side
(gzh-cli-devbox TASK-349 step 4; ce-agent-kit ADR-0205 결정 5). The `integrate check`, `integrate reconcile` and `integrate legacy-cleanup` subcommands, the
pre-mutation outcome machinery and every legacy flag spelling leave the CLI:
`integrate run` keeps `--source`, `--target <ref>@<sha>`, `--record <id>@<generation>` and `--owner <actor>/<host>/<kind>` only, and `integrate cleanup`/`receipt` keep `--record` only. The final surface is `run`, `cleanup`,
`receipt`, `queue` and `provision` (with its siblings).

Preconditions are met (ADR-0205 결정 5): ce-agent-kit master `5756e6f7c`
(TASK-518 `--record` cutover + runtime-owned release judgment, TASK-519
reconcile/legacy-cleanup removal) is released on this host as
`ce` v0.11.0-5-g5756e6f7c, the host provider cask is gz-git 0.14.2, and the
2026-11-04 zero-need proof recorded on the devbox TASK-349 path shows no
remaining caller of the removed surface. Upgrade order for any host stays
gz-git first, then ce.

Folds in the two actionable TASK-113 low findings: cleanup prints provenance
before argument parsing (`integrate_cleanup.go:77`) — parsing and refusal move
ahead of the provenance line, matching `receipt`; and the handler-level
refusal tests for a malformed `--record` are added (TASK-113 tested form
mixing only at handler level).

## Execution Boundary

`cmd/gz-git/cmd/` (the three subcommands, their tests, `integrate_run_args.go`
legacy halves, help wiring, `README.md`, `docs/commands/*`), `pkg/integrate/`
(subcommand-exclusive helpers: `pre_mutation_outcome.go`, the legacy
cleanup/reconcile bodies, check-subcommand-only helpers — traced per consumer;
anything `run`, `cleanup`, `receipt`, `queue` or `provision` still use stays,
including the run-time moved-target exit 4 and non-fast-forward guards). No
version bump or tagging in this card; the v0.15.0 release is executed after
integration and recorded on gzh-cli-devbox TASK-349.

## Completion Criteria

- [x] the check, reconcile and legacy-cleanup subcommands are gone from the CLI surface | verify: `! ls cmd/gz-git/cmd/integrate_check.go cmd/gz-git/cmd/integrate_reconcile.go cmd/gz-git/cmd/integrate_legacy_cleanup.go cmd/gz-git/cmd/integrate_legacy_cleanup_plan.go cmd/gz-git/cmd/integrate_legacy_cleanup_apply.go 2>/dev/null` → observed 2026-11-04: `ls-rc:1` (all five paths absent)
- [x] no legacy flag spelling remains on the integrate surface | verify: `! grep -rn '"expect-source"\|"expect-target"\|"owner-actor"\|"owner-host"\|"owner-kind"\|"execution-id"\|"generation"' cmd/gz-git/cmd/` → observed 2026-11-04: `grep-rc:1` (no matches; two test files reworded to stop asserting the literals themselves)
- [x] the pre-mutation outcome machinery is gone | verify: `! grep -rn "pre_mutation_outcome" pkg/ cmd/` → observed 2026-11-04: `grep-rc:1` (no matches)
- [x] the shared run guards survive the removal | verify: `go test ./cmd/gz-git/cmd -run 'TestIntegrateRunPinnedTargetThatMovedExitsFourWithoutPushing|TestIntegrateRunNonFastForwardSourceIsRefusedWithoutPushing' -count=1` → observed 2026-11-04: both PASS (1.21s / 0.97s). Binding corrected by the lead: the card originally pointed at `./pkg/integrate/`, where the pattern matches nothing (`[no tests to run]` still exits 0) — both tests live in `cmd/gz-git/cmd/integrate_run_args_test.go:191,212` (also absent from `./pkg/integrate/` on origin/master, so this is a card-writing error, not a move).
- [x] cleanup refuses a malformed record before any provenance line or receipt read (TASK-113 low finding 2) | verify: `go test ./cmd/gz-git/cmd -run TestIntegrateCleanupRefusesMalformedRecordWithoutProvenance -count=1` → observed 2026-11-04: `ok ... 0.400s` (test exists at `integrate_provenance_test.go:142`)
- [x] handler-level refusals cover a malformed and an incomplete record (TASK-113 low finding 3) | verify: `go test ./cmd/gz-git/cmd -run 'TestIntegrateRecordRefusalExitsTwoBeforeTouchingEvidence|TestIntegrateHandlerRefusesMalformedRecord|TestIntegrateHandlerRefusesIncompleteRecord' -count=1` → observed 2026-11-04: `ok ... 0.326s` (all three exist at `integrate_record_args_test.go:83,103,122`)
- [x] the impact-aware commit gate passes on the changed tree | verify: `VERIFY_BASE=origin/master make verify-commit` → observed 2026-11-04: widened to the full profile (deleted paths, Error 3); full profile green on the code-final tree (run 3: e2e + coverage floors, exit 0) and on the review-corrected tree (run 4: exit 0), with the authoritative re-run on this exact commit tree immediately before commit (run 5: exit 0, full verification profile passed; no tree change between gate and commit)

## Notes

- `cmd/AGENTS_COMMON.md` and `cmd/gz-git/AGENTS.md` are required reading before
  modifying `cmd/`.
- The guard hook on this host blocks any shell command containing the literal
  `gz-git integrate` — verify help output through `go test` or a renamed
  built binary, never by typing that literal into a shell command.
- Worktree module resolution: export `GOWORK=off` for `go`/`make` commands in
  the worktree; if a golden test demands a local `gzh-cli-core` replace, stop
  and report instead of editing `go.mod`.
- v0.15.0 release notes (GitHub Releases, generated) must list the removals
  and the three already-landed unreleased commits since v0.14.2 — `52ef273d`,
  `0644befd` (tasksync apply co-process), `618469b7` (dva ordering). Release
  execution: `make verify-release` (GOWORK=off), then tag and publish after a
  one-time user confirmation.
- Vacuous-probe note: the `go test` bindings run in the gate runner, not in
  todo validation; the `! ls`/`! grep` bindings are read-only predicates.

## Implementation

All changes left uncommitted on
`dev/claude/mbp/feat/remove-check-reconcile-legacy-cleanup` (base origin/master
`52ef273d`).

### Deleted

CLI (`cmd/gz-git/cmd/`): `integrate_check.go`, `integrate_reconcile.go`,
`integrate_legacy_cleanup.go` + `_plan` + `_apply`, `integrate_expected_target.go`,
and their test files (`integrate_check_test.go`, `integrate_reconcile_test.go`,
`integrate_legacy_cleanup_test.go`).

Engine (`pkg/integrate/`):

- `pre_mutation_outcome.go` + test, `legacy_reconcile.go` + test
- `legacy_cleanup_plan.go` / `legacy_cleanup_apply.go` /
  `legacy_cleanup_safety.go` + plan/positive/security tests
- `legacy_runtime_guard.go` + test
- `legacy_identity_birth_{linux,bsd,other}.go` + test
- `json_strict.go` (whole file) — added by the lead after the first
  `make verify-full` run failed golangci-lint `unused` on both of its
  functions; on origin/master their only consumers were in
  `legacy_reconcile.go`, so the file was an orphaned deletion-sweep miss.
  Gate history: run 3 (code-final tree) → exit 0, full verification profile
  passed (e2e 26.888s; coverage floors gitcmd 72.5%/59.1%,
  repository 59.8%/59.3%, reposync 61.0%/61.0%, workspacecli 69.4%/63.9%);
  run 4 (review-corrected card + done-review.md) → exit 0; run 5 (this
  exact commit tree, authoritative) → exit 0. Full table in
  docs/task-reviews/TASK-114/execution-evidence.md.

Docs: `docs/commands/integrate-legacy-cleanup.md`.

### Changed

- `integrate_run.go` / `integrate_run_args.go` / `integrate_cleanup.go` /
  `integrate_receipt.go`: flag sets reduced to the final surface (run keeps
  `--target --direct-to-default --release --controller-config --source --record --owner --json`;
  cleanup and receipt keep `--record` only); legacy cross-checks and spellings
  removed; cleanup handler parses the record before printing provenance
  (TASK-113 low finding 2), matching receipt.
- `integrate.go` (parent help), `help.go`, `gen_docs.go`: examples and
  subcommand lists onto the final surface.
- `capability.go` / `command_effect.go`: three removed capability constants and
  four removed command keys dropped.
- `pkg/integrate`: `run.go` (porcelain parsing extracted out; pre-mutation
  capture removed; `FormatCheck` header now `integrate run: `),
  `protocol.go` (`RunProtocol.PreMutationOutcome` field removed), `check.go`
  (pre-mutation field + capture removed), `check_preflight.go`,
  `check_target.go` (`ErrInvalidExpectSource`/`ErrInvalidExpectTarget` now name
  `--source` / `--target`), `device.go` (validateOwner moved in),
  `task_resource_fence.go` (comment).
- Tests: removed-flag/removed-command fixtures rewritten onto surviving
  commands; context-contract tests parse real flags (`ParseFlags` + reset
  helpers) since every context field now flows through `Changed()` gates;
  new named tests added (see below).
- Docs: `README.md`, `docs/commands/README.md`, `integrate-expect-source.md`
  (rewritten for `--source`/`--target <ref>@<sha>` incl. truncated-pin
  refusal), `integrate-cleanup-retry.md`, `integrate-no-fetch.md`,
  `integrate-resources.md`, `integrate-controller-config.md`,
  `integrate-prepare-profile.md`, `integrate-release.md`,
  `docs/user/guides/troubleshooting.md` (3 mentions).

### New files (extraction, zero behavior change)

- `pkg/integrate/push_porcelain.go` — porcelain push parsing moved verbatim
  from `run.go` (block verified identical via sed-extract + diff, including
  the `Done` footer rule the first extraction lacked).
- `pkg/integrate/execution_run_lock.go` + test — evidence run lock moved from
  `pre_mutation_outcome.go:106-121` (NOT `run.go` as first claimed; corrected
  after the independent review). Body byte-identical;
  `acquireExecutionRunLock`'s doc comment was reworded because the old
  rationale named the deleted pre-push-refusal machinery.
  `TestSameExecutionRefusesWhileAnotherRunOwnsIt` is the renamed surviving
  half of `TestSameExecutionCannotRecordNegativeWhileAnotherRunOwnsIt`
  (`pre_mutation_outcome_test.go:135`), minus the outcome-read assertion that
  tested the deleted machinery.
- `pkg/integrate/run_test.go` — `outcomeExecution` helper moved unchanged from
  the deleted `pre_mutation_outcome_test.go`.

### Preserved (consumer traces)

- Run guards untouched: moved-target exit 4 (porcelain stale-lease sole
  rejection → exit 4), non-fast-forward refusal, ownership checks.
- `validateOwner` → moved to `device.go`; consumer `device_test.go`.
- Shared helpers (`tryAcquireResourceLock`, `newGitRepo`, `splitPinnedTarget`,
  `cutLast`, `isHex`, `minAbbrevSHA`) kept with run/cleanup/receipt consumers.

### New named tests (TASK-113 fold)

`TestIntegrateCleanupRefusesMalformedRecordWithoutProvenance`,
`TestIntegrateRecordRefusalExitsTwoBeforeTouchingEvidence`,
`TestIntegrateHandlerRefusesMalformedRecord`,
`TestIntegrateHandlerRefusesIncompleteRecord`.

### Verification

- `go build ./...` → clean (also re-run clean after `make fmt`).
- Criterion bindings 1-3 → rc 1 each (see ticks above).
- `GOWORK=off go test ./cmd/gz-git/cmd/... ./pkg/integrate/... -count=1`
  → `ok cmd/gz-git/cmd 86.342s`, `ok pkg/integrate 344.507s`.
- `GOWORK=off make fmt` → complete.
- `GOWORK=off VERIFY_BASE=origin/master make verify-commit` → widened (deleted
  paths in diff): "full moved, deleted, type-changed, or unmerged path changed;
  commit profile widened; run 'make verify-full' before integration" (Error 3).
  Followed with `GOWORK=off VERIFY_BASE=origin/master make verify-full`:
  run 1 failed at `format-md-check` (the task card's own tick lines, fixed
  with `make fmt`); run 2 failed at `lint-check` — golangci-lint `unused` on
  `json_strict.go`'s two functions (deletion-sweep miss, file removed; see
  Deleted above), `go build ./...` re-verified clean afterwards; run 3
  (authoritative) → (result recorded below).

### Deviations

- `run.go` split (`push_porcelain.go` extracted from `run.go`; the run lock
  actually came from `pre_mutation_outcome.go`, see New files) to satisfy the
  ≤500-code-line file hook; moved blocks textually identical. `run.go` now
  577 total / 495 code lines.
- `TestIntegrateRunHelp`'s removed-flags map and the owner-grammar test's
  `"owner-host"` assertion were reworded (sentinel + refused-value assertions)
  so test files themselves carry none of the banned quoted literals that
  criterion 2 greps for.
- `docs/design/`, `docs/changelog/`, `docs/10-architecture/` left untouched as
  historical records; they still mention the removed subcommands (stale
  mentions listed in the report to the lead).
- `FormatCheck` report header reads `integrate run: ` now that `check` is gone.
- `docs/user/guides/troubleshooting.md` was edited via a python3 heredoc once
  (should have used the Edit tool; content verified afterwards).

## Review

Independent review (2026-11-04, opus reviewer, read-only,
docs/task-reviews/TASK-114/done-review.md): **PASS WITH MINORS** — nothing
must-fix before commit. Card claims corrected per findings 1-2 (the
execution_run_lock extraction source and the renamed-lock-test provenance,
both fixed above). Accepted as-is: near-duplicate refusal tests (finding 6),
owner-grammar test rewrite judged STRONGER than the original. Follow-up
candidates recorded without scope expansion: finding 3 `validateOwner`
(device.go:51) now has zero production callers and duplicates the inline
validation at receipt.go:127-139; finding 4
RUN_LIFECYCLE_PARITY_CONTRACT.md:371,435 still presents check-then-run as
the operative porting contract (internally pinned to CE 0.10.24 fd175fd2,
so point-in-time consistent) and the parity golden captures embed the same
sequence — needs a dated supersession note, not an edit of history; finding
5 no explicit test that a legacy spelling fails as an unknown flag (covered
implicitly by grep binding + cobra default). Independently confirmed by the
reviewer: porcelain block byte-identical (push_porcelain.go:12-143 ==
origin/master run.go:469-603), `outcomeExecution` verbatim, run.go pure
removal, build clean, all six named tests plus the rewritten contract/owner
tests pass, zero lingering legacy identifiers or flag literals, run guards
preserved, cleanup provenance reorder real, troubleshooting.md exactly 3
expected hunks, live docs clean. Not confirmable from the worktree (relied
on card precondition, verified separately by the lead): ce-agent-kit
preconditions and the verify-full outcome.
