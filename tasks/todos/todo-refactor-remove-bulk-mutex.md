# todo-refactor-remove-bulk-mutex

## Metadata
- **id**: todo-refactor-remove-bulk-mutex
- **title**: Remove unnecessary sync.Mutex in bulk operations
- **type**: refactor
- **priority**: P3
- **effort**: S
- **parent**: tasks/plans/P3-refactor-remove-unnecessary-mutex-in-bulk-parallel.md
- **created-at**: 2026-04-07T11:00:00+09:00

## Objective
Remove unused `sync.Mutex` (`mu.Lock()` / `mu.Unlock()`) that is protecting non-overlapping slice index assignments in the 14 `processXxxRepositories()` parallel loops in `pkg/repository/bulk*.go`.

## Verification
- `sync.Mutex` and its usage are removed from 14 locations in `pkg/repository/`.
- Unused `sync` imports are removed.
- `go test -race ./pkg/repository/...` passes without race conditions.

## Linkage
- **parent**: tasks/plans/P3-refactor-remove-unnecessary-mutex-in-bulk-parallel.md
