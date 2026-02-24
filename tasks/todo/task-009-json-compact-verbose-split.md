---
reopened-at: 2026-02-23T16:32:15+09:00
reopen-reason: |
  - Issue: Feature is not implemented, depends on incomplete TASK-008.
  - Required fix: Implement JSON formatting updates.
id: TASK-009
title: "JSON compact화 + --verbose와 --format 직교성 분리"
type: refactor

priority: P2
effort: M

parent: PLAN-002
depends-on: [TASK-008]
blocks: [TASK-010]

created-at: 2026-02-19T11:13:00Z
status: todo
---

## Purpose

모든 명령의 JSON 출력을 기본 compact(한 줄)로 변경하고,
`--verbose` 플래그를 `--format`과 독립적으로 동작하도록 분리한다.

## Scope

### Must
- **JSON compact화**: `--format json` 기본이 한 줄 compact JSON
- **Pretty JSON**: `--format json --verbose`일 때만 들여쓰기 적용
- **--verbose 분리**: 모든 명령에서 verbose가 정보량만 제어
  - `default`: 기본=요약, verbose=상세
  - `compact`: 기본=한줄, verbose=한줄+에러상세
  - `json`: 기본=compact, verbose=pretty+상세필드
  - `llm`: 기본=요약, verbose=상세
- 대상 명령: status, fetch, pull, push, update, switch, diff, watch, history *

### Must Not
- 포맷 상수/검증 로직 변경 (TASK-008에서 완료)
- 새 포맷 추가

## Definition of Done

- [ ] 모든 JSON 출력이 기본 compact (SetIndent 제거)
- [ ] `--verbose` + json = pretty JSON
- [ ] `--verbose`가 정보량만 제어하는지 각 명령 확인
- [ ] 기존 테스트 통과
- [ ] `make quality` 통과

## Checklist

### JSON Compact화 (각 명령)
- [ ] `status.go`: `displayStatusResultsJSON()` — compact 기본
- [ ] `fetch.go`: `displayFetchResultsJSON()` — compact 기본
- [ ] `pull.go`: `displayPullResultsJSON()` — compact 기본
- [ ] `push.go`: `displayPushResultsJSON()` — compact 기본
- [ ] `update.go`: `displayUpdateResultsJSON()` — compact 기본
- [ ] `switch.go`: `displaySwitchResultsJSON()` — compact 기본
- [ ] `diff.go`: `displayDiffResultsJSON()` — compact 기본

### --verbose 직교성 (각 명령)
- [ ] 각 명령에서 verbose의 역할 감사 및 문서화
- [ ] JSON verbose = pretty print 적용
- [ ] LLM verbose = 상세 필드 포함 적용
- [ ] default verbose = 상세 출력 (현재 대부분 유지)
- [ ] `cliutil.WriteJSON(w, v, verbose)` 사용으로 통일

## Technical Notes

```go
// 변경 전 (모든 명령)
encoder := json.NewEncoder(os.Stdout)
encoder.SetIndent("", "  ")  // 항상 pretty
encoder.Encode(output)

// 변경 후
cliutil.WriteJSON(os.Stdout, output, verbose)
// verbose=false → compact한줄
// verbose=true  → pretty들여쓰기
```

## Estimated Effort
3-4시간 (명령 수가 많아서)
