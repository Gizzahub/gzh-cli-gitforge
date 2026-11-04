---
id: TASK-115
title: "Canonicalize the execution run-lock path and refuse unknown integrate subcommands"
type: bug
priority: P1
effort: S
exec-tier: strong
created: 2026-11-04
status: done
completion-summary: "실행 락이 영수증과 같은 canonicalCommonDir로 common dir을 정규화해 파생 일관성과 fail-fast를 확보하고(자동 리뷰의 bypass 전제는 독립 리뷰가 실험적으로 반박 — 락은 flock이라 별칭 경로도 같은 inode로 이미 상호배제), integrate 상위명령이 unknown 하위명령을 exit 1로 거부하도록 했다(기존엔 exit 0으로 삼킴)."
quality-review: pass
quality-reviewed-at: "2026-11-04T12:07:18Z"
quality-review-evidence: docs/task-reviews/TASK-115/done-review.md
verification-status: passed
verification-evidence: docs/task-reviews/TASK-115/execution-evidence.md
---

## Summary

Two findings on master `f640293b` (v0.15.0 pre-release, user decision
2026-11-04: fix both before tagging):

1. **Run-lock derivation consistency (flagged security MEDIUM by automated
   review; premise corrected by the independent review).**
   `acquireExecutionRunLock` (`pkg/integrate/execution_run_lock.go`)
   derived the lock file from the raw `rev-parse --git-common-dir` value
   with none of the canonicalization `readReceipt`/`writeReceipt` apply.
   The automated review read that as a path-aliasing mutual-exclusion
   bypass; the independent review refuted that premise experimentally —
   the lock is an flock over the lock file's open file description, so
   symlinked spellings of one common dir resolve at open() to the same
   inode and already contended on one lock. What the raw derivation
   actually left open was theoretical: a relative `common` would join to
   a working-directory-dependent lock path (no production caller passes
   one — rev-parse is asked for absolute). The fix lands for derivation
   consistency with the receipts and fail-fast on an unusable common,
   not as an exploitable-hole closure. The flagged pattern pre-dates
   TASK-114 (the extraction moved the block verbatim).
1. **Removed subcommands are not refused (operational, lead smoke
   2026-11-04).** The `integrate` parent accepts an unknown subcommand
   argument and exits 0 after printing the Quick Start — observed with a
   retired name: it ran as `integrate check` and returned exit 0 with
   non-JSON output. A stale caller in the documented gz-git-first upgrade
   window (old ce on another host) would read success where the surface was
   removed. Unknown flags are already refused correctly (exit 1, "unknown
   flag"); only the subcommand slot swallows.

## Execution Boundary

`pkg/integrate/execution_run_lock.go` + its test (canonicalization via the
same helper the receipt paths use; refuse on canonicalization error; symlink
acquisition test asserting `ErrResourceBusy` while the canonical path holds
the lock), `cmd/gz-git/cmd/integrate.go` + a new test (unknown/retired
subcommand names exit non-zero with an explicit unknown-command message —
`cobra.NoArgs` or an equivalent Args validator; bare `integrate` keeps the
Quick Start). No new flags, no surface additions, no version bump.

## Completion Criteria

- [x] the run lock canonicalizes the common dir exactly as receipt paths do | verify: `go test ./pkg/integrate/ -run 'TestExecutionRunLock|TestSameExecution' -count=1` and the new symlink-aliasing test acquires via a symlinked repo path while the canonical path holds the lock and asserts ErrResourceBusy → observed 2026-11-04: both PASS (impl + independent review re-ran); per the review's premise correction the symlink test pins derivation consistency, not the flock-based exclusion (which held across spellings already)
- [x] an unknown or retired integrate subcommand exits non-zero with an explicit unknown-command message; bare `integrate` still prints the Quick Start | verify: `go test ./cmd/gz-git/cmd -run TestIntegrateRefusesUnknownSubcommands -count=1` → observed 2026-11-04: PASS — retired and nonsense names exit 1 with `unknown command` naming the arg, bare parent Quick Start exit 0, real-subcommand dispatch proven unbroken via the readonly command suite
- [x] the impact-aware commit gate passes on the changed tree | verify: `VERIFY_BASE=origin/master make verify-commit` → observed 2026-11-04: lead ran the full profile (`VERIFY_BASE=origin/master make verify-full`, superset of the verify-commit binding — no deleted paths in the diff, full profile chosen for v0.15.0 release confidence); run 1 stopped by the lead pre-verdict on a not-yet-final card (stale-tree guard, not a failure); run 2 authoritative on this exact commit tree immediately before commit → exit 0, full verification profile passed, no tree change between gate and commit

## Notes

- The guard hook on this host blocks any shell command containing the
  literal `gz-git integrate` — never type it; test through `go test` or a
  renamed built binary (`cp gz-git <tmp>/gzgit`).
- `GOWORK=off` for every go/make command in the worktree.
- `cmd/AGENTS_COMMON.md` and `cmd/gz-git/AGENTS.md` before touching `cmd/`.
- Task-114 review follow-ups (validateOwner dead code, parity supersession
  note, legacy-flag unknown coverage) are explicitly OUT of scope here; the
  unknown-subcommand refusal in criterion 2 may subsume the third only as a
  side effect.
- Release sequencing: this task lands, gates green, then `make verify-release` and the one-time user confirmation before tagging v0.15.0.

## Review

Independent review (equal grade, read-only, no repo gates, 2026-11-04):
**PASS WITH MINORS.**

- Finding 1 premise corrected by the reviewer, experimentally: the flagged
  mutual-exclusion bypass does not hold. The lock is an flock over the lock
  file's open file description, so symlinked spellings of one common dir
  resolve at open() to the same inode and already contended on one lock
  (EWOULDBLOCK, os.SameFile true). The doc comment, the alias-test comment,
  and this card's Summary were reworded to the corrected premise before
  commit; the change lands as derivation consistency with the receipts plus
  fail-fast on an unusable common, not as an exploitable-hole closure.
- Minor follow-up candidates (out of scope here, recorded per findings):
  `task_resource_fence.go:45-57` still derives from the raw common without
  canonicalization — weigh the same helper or a cross-version contention
  comment at lines 17-19; nine other group parents (branch, cleanup,
  conflict, forge, handoff, history, pr, sync, workspace) swallow unknown
  subcommand args the way `integrate` did before criterion 2.
- Reviewer confirmed dimensions: criterion-1 test command re-run PASS,
  criterion-2 refusal behavior PASS (retired and nonsense names, bare
  parent, real-subcommand dispatch unbroken via the readonly suite).
