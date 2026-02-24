---
archived-at: 2026-02-24T16:10:35+09:00
verified-at: 2026-02-24T16:10:35+09:00
verification-summary: |
  - Verified: bulk/history format constants and validators in `cmd/gz-git/cmd/bulk_common.go` are migrated to `pkg/cliutil` wrappers.
  - Evidence: `CoreFormats`, `ValidBulkFormats`, `ValidHistoryFormats`, `validate*Format`, and `shouldShowProgress` call `cliutil`; `mise x -- go test ./cmd/gz-git/cmd` passed.
reopened-at: 2026-02-23T16:32:15+09:00
reopen-reason: |
  - Issue: Feature is not implemented, depends on incomplete TASK-006.
  - Required fix: Implement format infrastructure migration.
id: TASK-008
title: "cmd/bulk_common.go 포맷 로직을 pkg/cliutil로 마이그레이션"
type: refactor

priority: P2
effort: M

parent: PLAN-002
depends-on: [TASK-006]
blocks: [TASK-009]

created-at: 2026-02-19T11:13:00Z
status: done
started-at: 2026-02-24T14:32:00+09:00
completed-at: 2026-02-24T14:48:00+09:00
completion-summary: "Migrated format logic from cmd/bulk_common.go to pkg/cliutil and fixed tests."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./tests/integration"
    result: "pass: ok github.com/gizzahub/gzh-cli-gitforge/tests/integration 1.164s"
---

## Purpose

`cmd/gz-git/cmd/bulk_common.go`에 있는 포맷 상수와 검증 함수를
`pkg/cliutil/format.go`의 공통 구현으로 교체한다.
기존 명령의 동작을 변경하지 않으면서 코드 중복을 제거한다.

## Scope

### Must
- `bulk_common.go`의 `CoreFormats`, `ValidBulkFormats`, `ValidHistoryFormats`를
  `cliutil.CoreFormats`, `cliutil.TabularFormats`로 교체
- `validateBulkFormat()`, `validateHistoryFormat()`을
  `cliutil.ValidateFormat()` 래퍼로 교체
- 모든 기존 명령이 동일하게 동작 (하위 호환성)
- `shouldShowProgress()`가 `cliutil.IsMachineFormat()` 활용

### Must Not
- 명령별 출력 포맷 변경 (Phase 4/5에서 처리)
- 새 기능 추가

## Definition of Done

- [x] `bulk_common.go`의 포맷 상수가 `cliutil` 참조
- [x] `validateBulkFormat()` → `cliutil.ValidateFormat(_, cliutil.CoreFormats)` 래핑
- [x] `validateHistoryFormat()` → `cliutil.ValidateFormat(_, cliutil.TabularFormats)` 래핑
- [x] 기존 모든 테스트 통과
- [x] `make quality` 통과
- [x] 기존 명령 출력 변경 없음

## Checklist

- [x] `bulk_common.go`의 포맷 변수를 cliutil alias로 교체
- [x] `validateBulkFormat()` → cliutil 래퍼
- [x] `validateHistoryFormat()` → cliutil 래퍼
- [x] `shouldShowProgress()` → cliutil.IsMachineFormat() 사용
- [x] 각 명령의 import 업데이트 (필요 시)
- [x] 기존 테스트 전체 통과 확인
- [x] `make quality` 통과

## Technical Notes

```go
// cmd/gz-git/cmd/bulk_common.go (변경 후)

import "github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"

// Backward compatibility aliases
var CoreFormats = cliutil.CoreFormats
var ValidBulkFormats = cliutil.CoreFormats        // compact은 CoreFormats에 포함
var ValidHistoryFormats = cliutil.TabularFormats

func validateBulkFormat(format string) error {
    return cliutil.ValidateFormat(format, cliutil.CoreFormats)
}

func validateHistoryFormat(format string) error {
    return cliutil.ValidateFormat(format, cliutil.TabularFormats)
}
```

## Estimated Effort
1-2시간
