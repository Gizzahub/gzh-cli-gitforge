---
id: TASK-006
title: "공통 포맷 인프라 구축 (pkg/cliutil/format.go)"
type: feature

priority: P1
effort: S

parent: PLAN-002
depends-on: []
blocks: [TASK-007, TASK-008]

created-at: 2026-02-19T11:13:00Z
status: todo
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

- [ ] `pkg/cliutil/format.go` 생성 및 테스트
- [ ] `pkg/cliutil/output.go` 생성 및 테스트
- [ ] `make quality` 통과
- [ ] 기존 빌드 깨지지 않음

## Checklist

- [ ] `format.go`: CoreFormats, TabularFormats 상수 정의
- [ ] `format.go`: ValidateFormat() 구현
- [ ] `format.go`: IsMachineFormat() 구현
- [ ] `output.go`: WriteJSON() 구현 (compact/pretty)
- [ ] `output.go`: WriteLLM() 구현
- [ ] `format_test.go`: 포맷 검증 테스트
- [ ] `output_test.go`: JSON/LLM 출력 테스트
- [ ] `make quality` 통과

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
