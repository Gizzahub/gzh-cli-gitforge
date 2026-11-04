---
id: TASK-116
title: "Prepare the v0.15.0 release"
type: chore
priority: P2
effort: S
exec-tier: strong
created: 2026-11-04
status: done
completion-summary: "VERSION과 version.go 기본 버전을 0.15.0으로 올리고, 준비 문서에 minor 사유(ADR-0161 최종 표면·ADR-0205 결정 5의 공개 표면 제거), 포함 6커밋, 검증 경로를 기록했다. 독립 리뷰(PASS WITH MINORS)가 준비 문서의 0644befd 서술 오류(해당 커밋이 숨김 task-sync-apply stdio 명령을 이미 등록)를 지목해 커밋 전에 바로잡았고 나열 순서도 실제 로그 순서로 정정했으며, 커밋 게이트가 VERSION(non-Go 입력)으로 verify-full로 확장된 만큼 확장 프로파일을 이 트리에서 exit 0로 통과시켰다."
quality-review: pass
quality-reviewed-at: "2026-11-04T13:59:03Z"
quality-review-evidence: docs/task-reviews/TASK-116/done-review.md
verification-status: passed
verification-evidence: docs/task-reviews/TASK-116/execution-evidence.md
---

## Summary

User confirmed the v0.15.0 public release on 2026-11-04 after
`make verify-release` passed on master `944fea97` (CI, full gate,
snapshot packaging). The release removes public surface (ADR-0161 final
surface, ADR-0205 decision 5), so the minor version advances: VERSION
and the unstamped version fallback move 0.14.2 to 0.15.0, and
`docs/releases/0.15.0-preparation.md` records the bump rationale, the
included commits with SHAs, and the verification path. After this task
integrates, the master CI success makes `release.yml` tag the SHA and
publish the release and Homebrew cask in the same job (VERSION newer
than every existing tag).

## Execution Boundary

`VERSION`, `version.go` (defaultVersion only), the new
`docs/releases/0.15.0-preparation.md`, this card. No code, no tag
creation by hand, no cask work (host cask upgrade stays a separate
devenv task).

## Completion Criteria

- [x] VERSION and the unstamped version fallback both name 0.15.0 | verify: `cat VERSION && grep -n 'defaultVersion = ' version.go` → observed 2026-11-04: `0.15.0` / `42:	defaultVersion = "0.15.0"` (lead and independent review both reproduced)
- [x] the preparation doc records the minor-bump rationale, every included commit since v0.14.2 with its SHA, and the verification path | verify: `test -f docs/releases/0.15.0-preparation.md && grep -c '944fea97\|f640293b\|17403bb4\|0644befd\|52ef273d\|618469b7' docs/releases/0.15.0-preparation.md` → observed 2026-11-04: 7 — `944fea97` appears twice (commit list and verification path), the other five SHAs once each; the review confirmed the six commits are exactly `git log v0.14.2..944fea97` with no omissions or extras
- [x] the impact-aware commit gate passes on the changed tree | verify: `VERIFY_BASE=origin/master make verify-commit` → observed 2026-11-04: the recipe exits 3 by design — VERSION is a non-Go input, so the profile widens and the gate instructs `make verify-full` (`.make/quality.mk:397-398`); the satisfying evidence is `GOWORK=off VERIFY_BASE=origin/master make verify-full`, which passed exit 0 on this exact tree, and `format-check` (markdown leg) re-passed on the finalized tree

## Notes

- `make verify-release` already passed on master `944fea97`; this
  commit is version-and-docs only, so that gate evidence applies. The
  impact-aware gate nonetheless classified VERSION as a non-Go input
  and widened the commit profile to verify-full; the full profile
  passed exit 0 on this tree and is the authoritative evidence.
- After integration, watch the master CI run and then the release
  workflow; verify tag SHA, archive checksums, and the cask version
  and digest after publishing.
- The release-notes intro (removal summary) is added to the generated
  GitHub Release body after publication through the owner-scoped gh
  wrapper. The generated notes exclude `docs:`/`chore:` subjects
  (`.goreleaser.yaml` changelog filters), so the intro should also
  name `17403bb4`, `52ef273d`, and `618469b7` to keep the release
  record complete, per TASK-114's note.

## Review

Independent review (read-only opus subagent, 2026-11-04): PASS WITH
MINORS. Verified: diff scope is exactly the four declared paths;
version consistency (no stale 0.14.2 references; release builds stamp
via ldflags so defaultVersion is the unstamped fallback only); the
six-commit list matches `git log v0.14.2..944fea97` exactly; the
surface-removal claims match the live CLI (five `AddCommand`
registrations; the three removed subcommand files absent; all seven
legacy flag literals gone) and ADR-0205 decision 5; the release.yml
description matches the workflow (workflow_run on master CI success,
tag-if-newer, GoReleaser + cask in the same job). Findings applied
before commit: the 0644befd line falsely said the tasksync co-process
was not CLI-wired (the commit adds the hidden `task-sync-apply` stdio
command) — reworded; list order corrected to actual log order; the
verification-path section now records the widened-profile full-gate
pass it previously omitted. Accepted minors (left as-is): the ADR-0161
attribution covers the removal claim, with provision out of ADR-0161
scope but retained (TASK-114 card states it precisely); the flag list
enumerates the contract arguments, not every retained flag;
"5756e6f7c 릴리스" is loose (no tag contains it; released host build
was v0.11.0-5-g5756e6f7c) — every ce reading still satisfies the
zero-need premise because v0.11.0 itself dropped the calls.
