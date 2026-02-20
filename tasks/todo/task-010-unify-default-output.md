---
id: TASK-010
title: "기존 명령 default 출력 통일 (요약 우선)"
type: refactor

priority: P3
effort: L

parent: PLAN-002
depends-on: [TASK-009]
blocks: []

created-at: 2026-02-19T11:13:00Z
status: todo
---

## Purpose

모든 명령의 `default` 출력을 가벼운 요약(summary-first) 스타일로 통일한다.
현재 일부 명령은 기본이 상세 출력이고, `workspace sync`만 요약 스타일이다.
사용자 경험의 일관성을 위해 모든 명령에서 동일한 원칙을 적용한다.

## Design Principle

```
default:           간결 요약 (1-3줄 summary + 문제 있는 항목만)
default --verbose: 상세 출력 (Repository Details, 전체 항목)
compact:           최소 출력 (문제 항목만 한줄씩)
```

### 예시: `gz-git fetch` (default)

```
# 변경 전 (현재)
=== Fetch Results ===
Total scanned:   6 repositories
Total processed: 6 repositories
Duration:        1.2s

Summary by status:
  ✓ up-to-date:     4
  ✓ fetched:         2

Repository details:
  ✓ repo-a (main)    up-to-date    0.2s
  ✓ repo-b (main)    up-to-date    0.2s
  ...

# 변경 후 (default = 요약)
Fetched 6 repositories  [✓4 up-to-date  ↓2 fetched]  1.2s

# 변경 후 (default --verbose = 상세)
=== Fetch Results ===
Total scanned:   6 repositories
...Repository details...
```

## Scope

### Must
- 모든 git 작업 명령의 default 출력을 요약 스타일로 변경:
  - `status`: 건강 요약 + 문제 항목만
  - `fetch`: 결과 요약 + 에러만
  - `pull`: 결과 요약 + 실패만
  - `push`: 결과 요약 + 실패만
  - `update`: 결과 요약 + 실패만
  - `switch`: 결과 요약 + 실패만
  - `diff`: 변경 요약 + 주요 항목
- --verbose로 현재의 상세 출력이 동작
- 히스토리 명령은 기본이 table이므로 변경 불필요

### Must Not
- compact, json, llm 포맷 변경
- 명령 플래그 구조 변경
- 히스토리 명령 변경

## Definition of Done

- [ ] 모든 git 작업 명령의 default가 1-3줄 요약
- [ ] --verbose로 기존 상세 출력 접근 가능
- [ ] 에러/문제 항목은 항상 표시 (verbose 무관)
- [ ] 기존 테스트 통과 (출력 테스트 업데이트 포함)
- [ ] `make quality` 통과

## Checklist

### 각 명령 출력 리팩토링
- [ ] `status.go`: displayDiagnosticResults() → 요약 우선
- [ ] `fetch.go`: displayFetchResults() → 요약 우선
- [ ] `pull.go`: displayPullResults() → 요약 우선
- [ ] `push.go`: displayPushResults() → 요약 우선
- [ ] `update.go`: displayUpdateResults() → 요약 우선
- [ ] `switch.go`: displaySwitchResults() → 요약 우선
- [ ] `diff.go`: displayDiffResults() → 요약 우선

### 공통 헬퍼
- [ ] `cliutil.WriteSummaryLine()` — "Fetched 6 repos [✓4 ↓2] 1.2s" 형식
- [ ] 각 명령에 적용

### 테스트 업데이트
- [ ] 출력 변경에 따른 기존 테스트 수정
- [ ] 새 요약 출력 테스트 추가

## Technical Notes

이 태스크는 가장 큰 변경이지만, Phase 1-4가 완료된 후에는
각 명령의 `displayXxxResults()` 함수를 하나씩 수정하는 반복 작업이다.

각 명령당 30-60분 예상. 전체 ~5-7시간.

병렬로 작업 가능: 명령 간 의존성 없음.

## Estimated Effort
5-7시간
