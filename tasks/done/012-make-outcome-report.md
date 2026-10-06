---
id: TASK-012
title: "Compare explicit per-check Make outcomes without file:line diagnostics"
type: feature
priority: P2
effort: M
exec-tier: strong
created: 2026-10-06
status: done
completion-summary: "검사별 Make 결과의 strict 비교와 불완전 증거 차단을 구현·검증했다. 기존 parity toolchain 실패는 별도 기록했다."
quality-review: pass
quality-reviewed-at: "2026-10-06T15:22:13Z"
quality-review-evidence: docs/task-reviews/TASK-012/done-review.md
verification-status: passed
verification-evidence: docs/task-reviews/TASK-012/execution-evidence.md
---

## Summary

파일·행 진단이 없는 Make 실패도 저장소가 명시적으로 선언한 검사별 결과로
비교한다. 기존 실패 유지와 개선은 구별하고 새 실패·검사 삭제·불완전한 실행은
차단한다. 최초 도입은 기존 비교를 유지하며 커밋에 기록된 루트 선언만 사용한다.

## Execution Boundary

`pkg/config/`, `pkg/integrate/`, 관련 명령 문서와 이 카드의 검증 기록을 수정한다.
기존 진단 귀속·개수 증가 차단을 유지한다. 미선언 소비자의 게이트를 변경하거나
검사 실행 없이 보고서만 생성하지 않는다.

## Completion Criteria

- [x] A complete explicit report distinguishes unchanged, improved and new plain failures and rejects incomplete evidence | verify: human — source tests and independent review

## Review Attempts

1. Reviewer `/root/retention_review` strong, executor root strong. YAML 범위,
   legacy alias 호환성, 순환 참조, inline merge와 FIFO open race를 지적했다.
   각각 실제 회귀와 수정 후 재검토했으며 최종 판정은
   `2026-10-06T15:22:13Z` PASS다. 기존 source 실패는 기능 성공과 구분했다.
