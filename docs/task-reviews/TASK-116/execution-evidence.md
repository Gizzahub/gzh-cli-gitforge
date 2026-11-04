# TASK-116 execution evidence — prepare the v0.15.0 release

All runs on the task worktree `claude__mbp__chore__prepare-v0-15-0`
(branch `dev/claude/mbp/chore/prepare-v0-15-0`, base `944fea97`), tree
state = the four declared paths (VERSION, version.go defaultVersion,
`docs/releases/0.15.0-preparation.md`, the card) unless noted.

## Gate history

1. **`VERIFY_BASE=origin/master make verify-commit`** (2026-11-04) —
   the profile run classifies VERSION as a non-Go input and refuses the
   light profile: `full non-Go or unknown input changed: VERSION` /
   `commit profile widened; run 'make verify-full' before integration`
   (make exits with `Error 3`). This is the gate's designed escalation,
   not a quality failure: with a VERSION change in the diff the commit
   gate never passes — the satisfying evidence is the full profile.
1. **`GOWORK=off VERIFY_BASE=origin/master make verify-full`**
   (background task `bv4vbkys1`, 2026-11-04) — **exit 0**. Tail
   evidence: all tests PASS (including `tests/e2e`), critical coverage
   floors satisfied (`internal/gitcmd` 72.5% vs 59.1%, `pkg/repository`
   59.8% vs 59.3%, `pkg/reposync` 61.0% vs 61.0%, `pkg/workspacecli`
   69.4% vs 63.9%), `✅ Canonical quality gate passed!` and
   `✅ Full verification profile passed!`.
1. **Post-finalization `make fmt` + `make format-check`** (2026-11-04,
   after the card/Review/doc edits) — the only delta the finalization
   could affect is markdown formatting; the format leg re-passed clean
   on the final tree. Go inputs are unchanged from the gated tree, so
   the full-profile Go legs cannot differ.

## Criterion observations

- Criterion 1: `cat VERSION` → `0.15.0`; `grep -n 'defaultVersion = ' version.go` → `42:	defaultVersion = "0.15.0"`. Reproduced by the
  lead and by the independent review.
- Criterion 2: grep -c over the six SHAs (the card's alternation
  pattern) in `docs/releases/0.15.0-preparation.md` → **7**
  (`944fea97` twice — commit list and verification path — the other
  five SHAs once each). The review confirmed the six commits are
  exactly `git log v0.14.2..944fea97`.
- Criterion 3: gate widened by design (run 1 above); satisfied by the
  full-profile exit 0 (run 2) per the gate's own instruction.

## Pre-existing release evidence this task relies on

- `make verify-release` passed on master `944fea97` earlier on
  2026-11-04 (release-ci-check confirmed the `944fea97` master CI
  completed and green after a watch-then-rerun; the full gate and
  check-only snapshot packaging passed after two stale `tmp/` blocker
  directories were removed from the primary checkout).
