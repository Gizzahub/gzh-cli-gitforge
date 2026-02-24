---
archived-at: 2026-02-24T16:10:35+09:00
verified-at: 2026-02-24T16:10:35+09:00
verification-summary: |
  - Verified: `pkg/cliutil/format.go` and `pkg/cliutil/output.go` implement required formats/validation/output helpers.
  - Evidence: Unit tests in `pkg/cliutil/format_test.go` and `pkg/cliutil/output_test.go`; `mise x -- go test ./pkg/cliutil` passed.
reopened-at: 2026-02-23T16:32:15+09:00
reopen-reason: |
  - Issue: The files `pkg/cliutil/format.go` and `pkg/cliutil/output.go` were not created.
  - Required fix: Implement the common format infrastructure as defined in the task description.
id: TASK-006
title: "공통 포맷 인프라 구축 (pkg/cliutil/format.go)"
type: feature

priority: P1
effort: S

parent: PLAN-002
depends-on: []
blocks: [TASK-007, TASK-008]

created-at: 2026-02-19T11:13:00Z
status: done
started-at: 2026-02-24T14:26:14+09:00
completed-at: 2026-02-24T14:26:14+09:00
completion-summary: "Implemented format.go, output.go and added tests"
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./pkg/cliutil/..."
    result: "pass: ok github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil 0.002s"
  - kind: automated
    command-or-step: "make build"
    result: "pass: Built gz-git successfully"
---

## Purpose

모든 CLI 명령이 공유하는 포맷 상수, 검증 함수, 출력 헬퍼를 `pkg/cliutil/`에 정의한다.
현재 `cmd/gz-git/cmd/bulk_common.go`에 있는 포맷 로직을 `pkg/` 레벨로 올려서
`pkg/workspacecli/` 등 다른 패키지에서도 재사용 가능하게 한다.

## Scope

### Must
- `pkg/cliutil/format.go` 생성
  - `CoreFormats`: `default, compact, json, llm`
  - `TabularFormats`: CoreFormats + `table, csv, markdown`
  - `ValidateFormat(format string, allowed []string) error`
  - `IsMachineFormat(format string) bool` (json, llm, csv)
- `pkg/cliutil/output.go` 생성
  - `WriteJSON(w io.Writer, v any, verbose bool) error` — compact/pretty 분기
  - `WriteLLM(w io.Writer, v any) error` — gzh-cli-core/cli 래핑
- 단위 테스트

### Must Not
- 기존 `cmd/bulk_common.go` 변경 (Phase 3에서 마이그레이션)
- 명령별 출력 로직 변경

## Definition of Done

- [x] `pkg/cliutil/format.go` 생성 및 테스트
- [x] `pkg/cliutil/output.go` 생성 및 테스트
- [x] `make quality` 통과
- [x] 기존 빌드 깨지지 않음

## Checklist

- [x] `format.go`: CoreFormats, TabularFormats 상수 정의
- [x] `format.go`: ValidateFormat() 구현
- [x] `format.go`: IsMachineFormat() 구현
- [x] `output.go`: WriteJSON() 구현 (compact/pretty)
- [x] `output.go`: WriteLLM() 구현
- [x] `format_test.go`: 포맷 검증 테스트
- [x] `output_test.go`: JSON/LLM 출력 테스트
- [x] `make quality` 통과

## Technical Notes

```go
// pkg/cliutil/format.go

package cliutil

// CoreFormats - 모든 명령이 필수 지원하는 포맷
var CoreFormats = []string{"default", "compact", "json", "llm"}

// TabularFormats - tabular 데이터 명령용 확장 포맷
var TabularFormats = []string{"default", "compact", "json", "llm", "table", "csv", "markdown"}

// ValidateFormat checks if the given format is in the allowed list.
func ValidateFormat(format string, allowed []string) error { ... }

// IsMachineFormat returns true for formats intended for machine consumption.
func IsMachineFormat(format string) bool {
    return format == "json" || format == "llm" || format == "csv"
}
```

## Estimated Effort
1-2시간
