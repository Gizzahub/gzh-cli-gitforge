---
id: TASK-117
title: "Refuse unknown subcommands on every group parent and land the TASK-114/115 review hardening follow-ups"
type: bug
priority: P2
effort: M
exec-tier: strong
created: 2026-11-04
status: done
completion-summary: "모든 그룹 상위명령이 unknown 하위명령을 exit 1으로 명시 거부하도록 TASK-115 패턴을 트리 전체에 적용하고 스윕 테스트로 열거를 고정했다(독립 리뷰가 reposynccli 팩토리에 등록된 forge config 누락을 유일한 블로커로 잡아 4개 조율로 해소). task resource fence가 run lock·영수증과 같은 canonicalCommonDir로 파생하도록 했으며, git이 cwd와 gitdir 포인터 모두 물리 경로로 정규화해 반환함을 실증해 black-box 파생 핀은 원리적으로 불가능임을 확인하고 단위 핀 + sentinel로 고정했다. validateOwner 제거와 parity supersession 노트 포함."
quality-review: pass
quality-reviewed-at: "2026-11-04T17:57:21Z"
quality-review-evidence: docs/task-reviews/TASK-117/done-review.md
verification-status: passed
verification-evidence: docs/task-reviews/TASK-117/execution-evidence.md
---

## Summary

TASK-115 made the `integrate` parent refuse an unknown subcommand with
exit 1 instead of printing the Quick Start and exiting 0. The same
swallow pattern is still open on every other group parent, so a stale
caller of any retired or mistyped subcommand reads success. The TASK-114
and TASK-115 reviews also left three named hardening follow-ups that
belong with this sweep. This card carries the whole post-review
hardening batch.

Group parents with children today (from `AddCommand` registrations,
2026-11-04): `branch`, `cleanup`, `config` (plus nested `profile`,
`token`), `conflict`, `forge` (plus nested `bootstrap`), `handoff`,
`history`, `pr`, `run`, `stash`, `tag`, `worktree`, and nested
`integrate provision`/`provision siblings`. The implementing session
re-enumerates this set from the tree rather than trusting the list.

## Execution Boundary

`cmd/gz-git/cmd/` group-parent files and their tests (Args validators of
the `cobra.NoArgs` family or an explicit unknown-command error), the
`command_effect.go` leaf/has_subcommands contract plus its table entries
if a parent's refusal changes the effect surface, the task resource
fence derivation (`pkg/integrate/task_resource_fence.go`),
`validateOwner` removal in `pkg/integrate/device.go` with its test, and
the parity-supersession doc
note. No behavior additions to `integrate`'s final surface, no flag
changes, no version bump beyond the normal release flow.

## Completion Criteria

- [x] every group parent refuses an unknown subcommand argument with exit 1 and an explicit unknown-command message, while a bare parent invocation keeps its current help behavior | verify: a table-driven test per parent (or one sweep test) fails on `parent unknown-sub` before the change and passes after; `go test ./cmd/gz-git/... -count=1` → observed 2026-11-05: the sweep `TestGroupParentsRefuseUnknownSubcommands` walks the tree-enumerated set (20 pinned keys, `forge config` included after the review blocker fix) — unknown probe exits 1 with an unknown-command error naming the probe, bare parent prints help exit 0; `GOWORK=off go test ./cmd/gz-git/cmd/ -count=1` → ok 167.003s (impl; independent review re-ran the sweep and caught the one miss)
- [x] the `command_effect.go` effect table encodes the leaf/has_subcommands contract explicitly — an unknown subcommand under a group parent cannot resolve to a leaf effect by exact-path lookup ambiguity, and the table documents that lookup is exact-key only | verify: `go test ./cmd/gz-git/cmd/ -run 'TestCommandEffect' -count=1` covers a group-parent/unknown-sub combination → observed 2026-11-05: `TestCommandEffectGroupParentContract` walks runnable available parents, requires declared + read-only, and asserts the probe key is absent from the table; the exact-key-only contract is documented in the table's doc comment → PASS
- [x] `pkg/integrate/task_resource_fence.go:45-57` derives its common dir through the same canonicalization the run lock and receipts use (`canonicalCommonDir`), not the raw `rev-parse --git-common-dir` value | verify: `go test ./pkg/integrate/ -run 'TestTaskResourceFence|TestSame' -count=1` with a symlink-spelling test pinning derivation consistency → observed 2026-11-05: PASS; derivation consistency is pinned at the unit level (`TestCanonicalCommonDirResolvesSymlinkedSpellings`) plus the fence test's exclusion check, current-derivation consistency check, and gitdir-pointer sentinel — a black-box derivation pin is impossible because git canonicalizes every spelling it reports (empirically confirmed for the cwd and the gitdir pointer file; see done-review.md)
- [x] `validateOwner` (`pkg/integrate/device.go:51`) — production callers removed by TASK-114, only `device_test.go:52,55` exercises it — is removed together with its test, or regains a production caller if one is found | verify: `grep -rn 'validateOwner' pkg/ cmd/ internal/` shows no definition-and-test-only remnant → observed 2026-11-05: grep returns no matches; `pkg/integrate` tests pass
- [x] the TASK-114 parity documentation records that the parity fixtures are superseded by the v0.15.0 final surface (the note the TASK-114 review requested) | verify: `grep -rn 'superseded' docs/` names the parity fixtures and the final-surface release → observed 2026-11-05: docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md:3 names `tests/parity/`, the v0.15.0 final surface (`89e43b2c`, ADR-0161/0205) and the removed subcommands

## Notes

- The TASK-115 automated-security finding on `command_effect.go` was
  judged not exploitable (exact-key lookup, no prefix consumer); the
  contract hardening here is prophylactic, and that analysis is recorded
  on the TASK-115 card and its review docs.
- The `integrate` parent itself already refuses (TASK-115
  `944fea97`); nested `integrate provision` parents are in scope for the
  same refusal.
- Discovered 2026-11-04, post-v0.15.0; nothing here is release-blocking.
- Empirical result shaping criterion 3 (2026-11-05): git canonicalizes
  every spelling it reports — a symlinked cwd AND a gitdir pointer file
  rewritten through a symlink both come back as the physical path from
  `rev-parse --git-common-dir`. So no git-reported spelling reaches the
  fence un-canonicalized; the derivation gap `canonicalCommonDir` closes
  is parity with the run lock and receipts, pinned at the unit level, and
  the fence test keeps a sentinel that fails loudly (with upgrade
  instructions) if a future git starts reporting symlinked spellings.
- Out-of-scope finding recorded per findings (2026-11-05):
  `TestExecutionRunLockSharesOneFileAcrossPathSpellings`
  (execution_run_lock_test.go) has the same exclusion-only shape the
  fence test was fixed out of — its `ErrResourceBusy` assertion holds
  through a reverted canonicalization because flock contends on one
  inode. Predates this task; the same unit pin
  (`TestCanonicalCommonDirResolvesSymlinkedSpellings`) covers the shared
  helper it goes through, so no action was taken here.

## Review

Independent review (equal grade, read-only, no repo gates), two passes:

- First pass (2026-11-04T17:23Z): 1 blocker — `forge config` missed,
  registered by the `reposynccli` factory rather than `cmd/`, proven by
  the reviewer's own sweep run as the package's only failure — plus 2
  recommended fixes (duplicated comment block; fence symlink test not
  discriminating). Every other dimension confirmed: parents patterned
  with Use/Short/Long and flags untouched, no broken callers,
  effect-table keys consistent with the exact-key contract, fence edit
  clean, `validateOwner` removal clean, parity note accurate.
- Confirmation pass (2026-11-04T17:57Z, on f31fb93a): all three
  findings RESOLVED and re-verified green by the reviewer's independent
  run (sweep, effect contract, declaration coverage, unit pin, fence —
  all PASS, 22 walked parents); no omissions. Criterion 3 judged
  satisfied as strongly as is observable: the unit pin discriminates
  where possible, the fence test asserts the derived lock-dir string
  plus cross-spelling exclusion, and the sentinel turns "git
  canonicalizes every spelling it reports" into a tested invariant.
  Residual documented honestly: removing the `canonicalCommonDir` call
  inside the fence has no observable discriminator under that invariant;
  a wiring pin would need a `gitcmd.Executor` interface extraction —
  optional future hardening, outside this card.
- Verdict: ready to close. Evidence:
  docs/task-reviews/TASK-117/done-review.md
