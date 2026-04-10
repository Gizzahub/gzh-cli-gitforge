# P3 | chore: Add nolint directives for RepositoryXxxResult naming convention

## Metadata
- **Type**: chore
- **Priority**: P3
- **Effort**: XS (15m)
- **Source**: session-review / lint
- **Tags**: lint, revive, naming-convention, nolint
- **State**: done
- **Progress**: 100%
- **Total Tasks**: 1
- **Completed Tasks**: 1
- **Completed At**: 2026-04-08T14:27:00+09:00
- **Completion Summary**: Added `.golangci.yml` revive exception for stutter check on exported types.

## Problem

`revive` 린터가 `repository.RepositoryXxxResult` 이름이 stutter라고 경고.
이는 **프로젝트 전체 컨벤션**이며 14개 타입 모두 동일 패턴 사용.
API 변경 없이 경고를 억제해야 함.

```
pkg/repository/bulk_clean.go:80:6: exported: type name will be used as
repository.RepositoryCleanResult by other packages, and that stutters;
consider calling this CleanResult (revive)
```

## Affected Types (14개)

`pkg/repository/` 내 모든 `Repository*Result` 타입:
- `RepositoryFetchResult`, `RepositoryPullResult`, `RepositoryPushResult`
- `RepositoryStatusResult`, `RepositoryUpdateResult`, `RepositorySwitchResult`
- `RepositoryCleanResult`, `RepositoryCleanupResult`, `RepositoryCloneResult`
- `RepositoryCommitResult`, `RepositoryDiffResult`, `RepositoryStashResult`
- `RepositoryTagResult`, `RepositoryBranchListResult`

## Proposed Fix

방법 A: 각 타입에 nolint 추가

```go
//nolint:revive // RepositoryXxx prefix is project-wide convention for clarity
type RepositoryCleanResult struct { ... }
```

방법 B: `.golangci.yml`에 revive 예외 패턴 추가

```yaml
linters-settings:
  revive:
    rules:
      - name: exported
        arguments:
          - disableStutteringCheck: true
```

**권장: 방법 B** (`.golangci.yml` 1곳만 수정, 향후 타입 추가 시 자동 적용)

## Acceptance Criteria

- [ ] `golangci-lint run --new-from-rev=HEAD ./...` 에서 revive stutter 경고 없음
- [ ] 기존 타입명 변경 없음 (API 호환)
- [ ] `make lint` 통과

## Children

- `tasks/_archive/todo-chore-disable-revive-stutter-check.md`
