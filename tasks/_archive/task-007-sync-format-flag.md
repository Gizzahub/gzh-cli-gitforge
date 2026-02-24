---
archived-at: 2026-02-24T16:10:35+09:00
verified-at: 2026-02-24T16:10:35+09:00
verification-summary: |
  - Verified: `workspace sync` has `--format` support (`default|compact|json|llm`) with machine-output handling and JSON/LLM renderers.
  - Evidence: `pkg/workspacecli/sync_command.go` format validation/output path, new tests in `pkg/workspacecli/sync_output_test.go`, and `mise x -- go test ./pkg/workspacecli` passed.
reopened-at: 2026-02-23T16:32:15+09:00
reopen-reason: |
  - Issue: Feature is not implemented. `format` flag does not exist in `pkg/workspacecli/sync_command.go` and it is blocked by TASK-006.
  - Required fix: Implement sync change format flag as described.
id: TASK-007
title: "workspace sync에 --format 플래그 추가"
type: feature

priority: P1
effort: M

parent: PLAN-002
depends-on: [TASK-006]
blocks: []

created-at: 2026-02-19T11:13:00Z
status: done
started-at: 2026-02-24T14:26:40+09:00
completed-at: 2026-02-24T14:31:00+09:00
completion-summary: "Added --format flag to workspace sync with JSON/LLM support"
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./pkg/workspacecli/..."
    result: "pass: ok github.com/gizzahub/gzh-cli-gitforge/pkg/workspacecli 0.014s"
  - kind: automated
    command-or-step: "make build"
    result: "pass: Built gz-git successfully"
---

## Purpose

`gz-git workspace sync` 명령에 `--format` 플래그를 추가하여
`default`, `compact`, `json`, `llm` 포맷을 지원한다.
TASK-006에서 만든 `pkg/cliutil` 공통 인프라를 사용하는 첫 사례가 된다.

## Scope

### Must
- `workspace sync --format default` (기본, 현재 요약 출력)
- `workspace sync --format compact` (한줄 결과만)
- `workspace sync --format json` (compact JSON 결과)
- `workspace sync --format llm` (LLM-friendly 구조화 텍스트)
- `--verbose`와 `--format` 직교: verbose는 정보량, format은 형식
- JSON/LLM 모드에서는 in-place progress 비활성 (machine output)

### Must Not
- 다른 명령 변경
- in-place progress 로직 자체 변경

## Definition of Done

- [x] `workspace sync --format json` 동작 (compact JSON)
- [x] `workspace sync --format llm` 동작
- [x] `workspace sync --format compact` 동작
- [x] JSON/LLM 모드에서 ANSI escape 없음
- [x] `--verbose`와 조합 동작 (json+verbose = pretty JSON)
- [x] 테스트 추가
- [x] `make quality` 통과

## Checklist

- [x] sync_command.go에 `format` 변수 및 `--format` 플래그 추가
- [x] `cliutil.ValidateFormat()` 으로 포맷 검증
- [x] SyncResult 타입 정의 (JSON/LLM 출력용 구조체)
- [x] `displaySyncResultsJSON()` 구현
- [x] `displaySyncResultsLLM()` 구현
- [x] format이 machine일 때 consoleProgress 사용 (in-place 비활성)
- [x] format이 machine일 때 프리뷰 생략
- [x] 테스트: JSON 출력 파싱 검증
- [x] `make quality` 통과

## Technical Notes

```go
// sync 결과 JSON 구조
type SyncResultJSON struct {
    Total     int              `json:"total"`
    Succeeded int              `json:"succeeded"`
    Failed    int              `json:"failed"`
    Duration  int64            `json:"duration_ms"`
    Repos     []SyncRepoJSON   `json:"repositories"`
}

type SyncRepoJSON struct {
    Name    string `json:"name"`
    Action  string `json:"action"`
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
    Error   string `json:"error,omitempty"`
}
```

## Estimated Effort
2-3시간
