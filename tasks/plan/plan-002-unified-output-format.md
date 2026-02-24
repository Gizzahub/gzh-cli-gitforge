---
reopened-at: 2026-02-23T16:32:15+09:00
reopen-reason: |
  - Issue: Plan children (TASK-006 to TASK-009) were incorrectly marked as done without implementation.
  - Required fix: Complete the child tasks.
id: PLAN-002
title: "Unified CLI Output Format System"
type: plan
priority: P1
effort: L

created-at: 2026-02-19T11:13:00Z
status: in_progress
total-tasks: 5
completed-tasks: 3
progress: 60.0

children:
  - TASK-006
  - TASK-007
  - TASK-008
  - TASK-009
  - TASK-010
---

## Purpose

CLI 전체의 출력 포맷 시스템을 일관되게 통합한다.
현재 Git 작업 명령(status, fetch, pull 등)과 히스토리 명령(stats, contributors 등)이
서로 다른 포맷 세트와 검증 함수를 사용하고 있어 불일치가 발생한다.

## Problem Statement

### 현재 상태

| 구분 | 포맷 옵션 | 검증 함수 | 위치 |
|------|-----------|-----------|------|
| Git 작업 | `default, compact, json, llm` | `validateBulkFormat()` | `cmd/bulk_common.go` |
| 히스토리 | `default, table, json, csv, markdown, llm` | `validateHistoryFormat()` | `cmd/bulk_common.go` |
| Workspace | (없음) | (없음) | `pkg/workspacecli/` |
| Root help | `llm` only | inline | `cmd/root.go` |

### 문제점

1. **포맷 분리**: Git 작업과 히스토리가 별도 포맷 세트 사용 → 사용자 혼란
2. **verbose 혼재**: `--verbose`가 일부는 포맷, 일부는 정보량 제어에 사용
3. **JSON pretty**: JSON이 pretty-print로 출력 → 기계 소비에 비효율적
4. **default 불일치**: 명령마다 default 출력이 다름 (상세 vs 요약)
5. **workspace 미지원**: `workspace sync`에 `--format` 없음
6. **포맷 로직 위치**: `cmd/` 패키지에 포맷 상수/검증이 있어 `pkg/`에서 재사용 불가

## Design

### 포맷 체계

```
Core Formats (모든 명령 필수 지원):
  default   — 사람용 가벼운 요약 (compact summary)
  compact   — 최소한의 한줄 출력 (문제있는 것만 표시)
  json      — 기계용 compact JSON (한 줄)
  llm       — LLM-friendly 구조화 텍스트 (via gzh-cli-core/cli)

Extended Formats (tabular 데이터 명령만 추가 지원):
  table     — 표 형태 출력
  csv       — CSV 출력
  markdown  — Markdown 표 출력
```

### --verbose와 --format 직교성

```
--format: 출력 형식 (HOW to present)
--verbose: 정보량 (HOW MUCH to show)

| format\verbose | 기본 | --verbose |
|----------------|------|-----------|
| default        | 요약 | 상세 (Repository Details) |
| compact        | 한줄 | 한줄 + 에러 상세 |
| json           | compact JSON | pretty JSON (들여쓰기) |
| llm            | 요약 LLM | 상세 LLM |
```

### JSON 출력 규칙

```go
// 기본: compact (한 줄)
encoder := json.NewEncoder(os.Stdout)
encoder.Encode(output)

// --verbose: pretty print
if verbose {
    encoder.SetIndent("", "  ")
}
encoder.Encode(output)
```

### 공통 코드 위치

```
pkg/cliutil/
├── format.go          # 포맷 상수, 검증, ValidFormats
├── format_test.go     # 포맷 검증 테스트
├── output.go          # 공통 출력 헬퍼 (JSON, LLM wrapper)
├── output_test.go     # 출력 헬퍼 테스트
├── help.go            # (기존) QuickStartHelp
└── doc.go             # (기존) 패키지 문서
```

### cmd/bulk_common.go 마이그레이션

```
현재: cmd/gz-git/cmd/bulk_common.go
  - CoreFormats, ValidBulkFormats, ValidHistoryFormats
  - validateBulkFormat(), validateHistoryFormat()

변경 후: 
  - pkg/cliutil/format.go로 이동
  - cmd/bulk_common.go는 pkg/cliutil를 import하여 래핑
  - 하위 호환성 유지 (type alias / wrapper)
```

## Migration Strategy

점진적 마이그레이션으로 기존 동작을 깨지 않음:

1. **Phase 1**: 공통 인프라 (`pkg/cliutil/format.go`) → TASK-006
2. **Phase 2**: `workspace sync`에 `--format` 추가 → TASK-007
3. **Phase 3**: `cmd/bulk_common.go` → `pkg/cliutil`로 이동 → TASK-008
4. **Phase 4**: JSON compact화 + `--verbose` 분리 → TASK-009
5. **Phase 5**: 기존 명령 default 출력 통일 → TASK-010

## Success Criteria

- [ ] 모든 명령이 `--format default|compact|json|llm` 지원
- [ ] `--verbose`가 모든 명령에서 정보량만 제어 (포맷과 독립)
- [ ] JSON 기본이 compact, `--verbose`로 pretty
- [ ] `workspace sync --format llm` 동작
- [ ] `pkg/cliutil/format.go`에 포맷 상수/검증 통합
- [ ] 기존 명령 하위 호환성 유지
- [ ] `make quality` 통과

## References

- 기존 포맷 로직: `cmd/gz-git/cmd/bulk_common.go`
- LLM 출력 유틸: `github.com/gizzahub/gzh-cli-core/cli`
- workspace sync: `pkg/workspacecli/sync_command.go`

## Children
- [x] TASK-006: 공통 포맷 인프라 구축 (pkg/cliutil/format.go)
- [x] TASK-007: workspace sync --format 기능 추가 (pkg/workspacecli/)
- [x] TASK-008: 공통 포맷 로직 통합 및 마이그레이션 (cmd/bulk_common.go -> pkg/cliutil/)
- [ ] TASK-009: JSON Compact 분리 및 --verbose 동작 변경
- [ ] TASK-010: 기존 명령 default 포맷 규칙 통일
