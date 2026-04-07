# P2 | refactor: Extract shared JSON/LLM output builder for bulk commands

## Metadata
- **Type**: refactor
- **Priority**: P2
- **Effort**: M (2-3h)
- **Source**: session-review / code-analysis
- **Tags**: bulk-cmd, duplication, output, json, llm

## Problem

현재 10개 bulk 명령(fetch, pull, push, status, clean, commit, diff, switch, update, tag)에서
`displayXxxResultsJSON()` / `displayXxxResultsLLM()` 함수가 **output 빌딩 로직을 완전 중복**으로 가짐.

두 함수는 1~2줄(직렬화 방식)만 다르고 나머지 ~25줄(레포지토리 루프 + 구조체 매핑)이 동일.

- **중복 규모**: 명령당 ~30줄 × 10개 = ~300줄
- **위험**: 출력 필드 추가 시 각 명령 2곳 수정 필요 → 누락 가능성

## Affected Files

| 파일 | 중복 위치 |
|------|-----------|
| `cmd/gz-git/cmd/fetch.go` | `displayFetchResultsJSON` / `displayFetchResultsLLM` |
| `cmd/gz-git/cmd/pull.go` | `displayPullResultsJSON` / `displayPullResultsLLM` |
| `cmd/gz-git/cmd/push.go` | `displayPushResultsJSON` / `displayPushResultsLLM` |
| `cmd/gz-git/cmd/status.go` | `displayStatusResultsJSON` (LLM 없음) |
| `cmd/gz-git/cmd/clean.go` | `displayCleanResultsJSON` / `displayCleanResultsLLM` |
| `cmd/gz-git/cmd/commit.go` | `displayCommitResultsJSON` |
| `cmd/gz-git/cmd/diff.go` | `displayDiffResultsJSON` / `displayDiffResultsLLM` |
| `cmd/gz-git/cmd/switch.go` | `displaySwitchResultsJSON` / `displaySwitchResultsLLM` |
| `cmd/gz-git/cmd/update.go` | `displayUpdateResultsJSON` / `displayUpdateResultsLLM` |
| `cmd/gz-git/cmd/tag.go` | `displayTagResultsJSON` / `displayTagResultsLLM` |

## Proposed Solution

output 빌딩을 단일 함수로 분리, 직렬화만 포맷별 분기:

```go
// cmd/gz-git/cmd/bulk_common.go에 추가
func writeBulkOutput(format string, output any) {
    switch format {
    case "json":
        if err := cliutil.WriteJSON(os.Stdout, output, verbose); err != nil {
            fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
        }
    case "llm":
        var buf bytes.Buffer
        out := cli.NewOutput().SetWriter(&buf).SetFormat("llm")
        if err := out.Print(output); err != nil {
            fmt.Fprintf(os.Stderr, "Error encoding LLM format: %v\n", err)
            return
        }
        fmt.Print(buf.String())
    }
}

// 각 명령에서:
func displayFetchResults(result *repository.BulkFetchResult) {
    // ...
    case "json", "llm":
        writeBulkOutput(fetchFlags.Format, buildFetchOutput(result))  // 1개 함수만
}
```

## Acceptance Criteria

- [ ] `writeBulkOutput(format, output)` 헬퍼가 `bulk_common.go`에 추가됨
- [ ] 10개 명령에서 JSON/LLM 중복 루프 제거
- [ ] 출력 결과 동일 (기존 테스트 통과)
- [ ] `make build && make test` 통과

## Children

- `tasks/todos/todo-refactor-bulk-output-create.md`
- `tasks/todos/todo-refactor-bulk-output-remove.md`
