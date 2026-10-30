---
id: TASK-113
title: "Accept --record <id>@<generation> on integrate cleanup and receipt"
type: feature
priority: P2
effort: S
exec-tier: strong
created: 2026-10-31
status: done
completion-summary: "integrate cleanup와 receipt가 --record <id>@<generation>을 받도록 run과 같은 파서를 공유했다. 레거시 플래그는 deprecated 표기만 하고 유지한다."
quality-review: pass
quality-reviewed-at: "2026-10-30T17:27:45Z"
quality-review-evidence: docs/task-reviews/TASK-113/done-review.md
verification-status: passed
verification-evidence: docs/task-reviews/TASK-113/execution-evidence.md
---

## Summary

ADR-0161 decision 3 names `--record` for the execution identity. The run
subcommand has taken `--record <id>@<generation>` since v0.14.0; cleanup and
receipt still take only `--execution-id` and `--generation`, which blocks
ce-agent-kit from moving its calls (gzh-cli-devbox TASK-349 step 3) and gz-git
from later removing the legacy flags (step 4). Tracked by ISSUE-098.

## Execution Boundary

`cmd/gz-git/cmd/integrate_{cleanup,receipt,run_args}.go`, their tests, the
README and `docs/commands/integrate-cleanup-retry.md`. Nothing is removed; check,
reconcile, provision and legacy-cleanup are untouched; no version bump.

## Completion Criteria

- [x] Both forms resolve to the same execution pair on cleanup and receipt, sharing the run parser | verify: `go test ./cmd/gz-git/cmd -run TestIntegrateExecutionRecordBothFormsResolveToTheSamePair -count=1`
- [x] Mixing the forms is refused with exit 2 before any receipt is read | verify: `go test ./cmd/gz-git/cmd -run 'TestIntegrateExecutionRecordRefusesMixingTheForms|TestIntegrateRecordRefusalExitsTwoBeforeTouchingEvidence' -count=1`
- [x] A missing or truncated generation is refused as run does | verify: `go test ./cmd/gz-git/cmd -run TestIntegrateExecutionRecordRefusesMissingOrTruncatedForms -count=1`

## Review Attempts

1. Implementer `gf-record-args` (Claude Sonnet 5.5, high), reviewer
   `gf-record-review` (Claude Sonnet 5.5, high, separate agent), integrator
   Claude Opus 5.5. Reviewed `82480dc2..77068e21`; verdict
   `2026-10-30T17:27:45Z` PASS with no blocking findings. Low findings were
   recorded, not fixed: see `docs/task-reviews/TASK-113/done-review.md`.
