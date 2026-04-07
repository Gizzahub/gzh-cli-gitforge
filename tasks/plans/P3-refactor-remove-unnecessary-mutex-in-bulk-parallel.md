# P3 | refactor: Remove unnecessary sync.Mutex in bulk parallel processing

## Metadata
- **Type**: refactor
- **Priority**: P3
- **Effort**: S (30m)
- **Source**: session-review / code-analysis
- **Tags**: performance, concurrency, mutex, bulk-cmd

## Problem

14개 `processXxxRepositories()` 함수에서 `sync.Mutex`로 `results[i]` 쓰기를 보호하지만,
각 goroutine이 **고유 인덱스 `i`에만 쓰므로 경합이 없음** — mutex 불필요.

```go
// 현재 패턴 (14곳 동일):
var mu sync.Mutex
for i, repoPath := range repos {
    i, repoPath := i, repoPath
    g.Go(func() error {
        result := c.processXxx(...)
        mu.Lock()
        results[i] = result  // i는 goroutine별 유일값
        mu.Unlock()
        return nil
    })
}
```

## Affected Files

`pkg/repository/`:
- `bulk.go` (7곳: Fetch, Pull, Push, Status, Update, Switch, Commit)
- `bulk_clean.go`, `bulk_cleanup.go`, `bulk_commit.go`
- `bulk_diff.go`, `bulk_stash.go`, `bulk_switch.go`, `bulk_branch_list.go`

총 **14곳**

## Proposed Fix

```go
// 수정 후: mutex 제거
for i, repoPath := range repos {
    i, repoPath := i, repoPath
    g.Go(func() error {
        results[i] = c.processXxx(gctx, ...)
        return nil
    })
}
```

Go spec 보장: slice 원소들은 별개의 메모리 주소이며 독립적으로 쓰기 가능.
단, `results` slice 자체의 재할당은 없어야 함 (이미 `make([]T, len(repos))`로 사전 할당됨 ✓).

## Acceptance Criteria

- [ ] 14곳에서 `var mu sync.Mutex`, `mu.Lock()`, `mu.Unlock()` 제거
- [ ] `import "sync"` 미사용 시 제거
- [ ] `go test -race ./pkg/repository/...` 통과 (race detector)
- [ ] 기능 동일성 확인

## Children

- `tasks/todos/todo-refactor-remove-bulk-mutex.md`
