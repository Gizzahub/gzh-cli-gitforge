# P2 | refactor: Unify bulk flag registration to prevent default-value bugs

## Metadata
- **Type**: refactor
- **Priority**: P2
- **Effort**: S (1h)
- **Source**: session-review / bug-prevention
- **Tags**: bulk-cmd, flags, watch, interval, regression-prevention

## Problem

`addBulkFlags()`를 사용하지 못하는 5개 명령이 플래그를 수동 등록.
이 과정에서 **기본값 불일치 버그**가 발생함.

### 실제 발생 버그 (clean.go)

```go
// clean.go init() 에서:
cleanCmd.Flags().DurationVar(&cleanFlags.Interval, "interval", cleanFlags.Interval, ...)
// cleanFlags.Interval = 0 (zero value) → --watch 시 time.NewTicker(0) → PANIC
```

`addBulkFlags()`의 정의:
```go
cmd.Flags().DurationVar(&flags.Interval, "interval", 5*time.Minute, ...)  // 5분 하드코딩
```

수동 등록 시 이 기본값을 복사하지 않으면 버그 발생.

### 수동 등록 명령 목록

| 명령 | 이유 |
|------|------|
| `clean.go` | `--force` 대신 dry-run (기본값 의미 다름) |
| `cleanup_branch.go` | `--dry-run` 기본값이 true (역전) |
| `switch.go` | - |
| `diff.go` | - |
| `branch_list.go` | - |

## Proposed Solution

`addBulkFlags`에 옵션 파라미터 추가:

```go
type BulkFlagOptions struct {
    SkipDryRun bool  // clean, cleanup branch처럼 --force 쓰는 경우
    SkipFetch  bool
}

func addBulkFlagsWithOpts(cmd *cobra.Command, flags *BulkCommandFlags, opts BulkFlagOptions) {
    cmd.Flags().IntVarP(&flags.Depth, "scan-depth", "d", repository.DefaultBulkMaxDepth, ...)
    cmd.Flags().IntVarP(&flags.Parallel, "parallel", "j", repository.DefaultBulkParallel, ...)
    cmd.Flags().BoolVarP(&flags.IncludeSubmodules, "recursive", "r", false, ...)
    cmd.Flags().StringVar(&flags.Include, "include", "", ...)
    cmd.Flags().StringVar(&flags.Exclude, "exclude", "", ...)
    cmd.Flags().StringVar(&flags.Format, "format", "default", ...)
    cmd.Flags().BoolVar(&flags.Watch, "watch", false, ...)
    cmd.Flags().DurationVar(&flags.Interval, "interval", 5*time.Minute, ...)  // 기본값 중앙화
    if !opts.SkipDryRun {
        cmd.Flags().BoolVarP(&flags.DryRun, "dry-run", "n", false, ...)
    }
    if !opts.SkipFetch {
        cmd.Flags().BoolVar(&flags.SkipFetch, "skip-fetch", false, ...)
    }
}
```

## Acceptance Criteria

- [ ] `addBulkFlagsWithOpts()` 또는 동등한 옵션 파라미터 방식 구현
- [ ] 5개 수동 등록 명령이 새 함수 사용
- [ ] `--watch --interval` 기본값 모든 명령에서 `5m` 일치
- [ ] 기존 플래그 동작 변경 없음
- [ ] `make build && make test` 통과
