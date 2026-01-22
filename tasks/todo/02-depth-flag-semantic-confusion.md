---
title: Unify workspace scan --depth to --scan-depth for consistency
priority: P1
effort: S
created: 2026-01-22
status: todo
type: refactor
area: cli
tags: [consistency, api-design, ux]
---

# Unify workspace scan --depth to --scan-depth

## Problem

`workspace scan` 명령어만 `--depth`를 사용하고, 다른 모든 디렉토리 스캔 명령어는 `--scan-depth`를 사용함.

| 명령어 | 플래그 | 의미 | 일관성 |
|--------|--------|------|--------|
| bulk 명령어 (status, fetch, pull 등) | `--scan-depth, -d` | 디렉토리 스캔 | ✅ |
| workspace status | `--scan-depth, -d` | 디렉토리 스캔 | ✅ |
| **workspace scan** | `--depth` | 디렉토리 스캔 | ❌ 불일치 |
| clone | `--depth` | Git shallow clone | ✅ Git 표준 |

## Current State

### workspace scan (pkg/workspacecli/scan_command.go:67)
```go
cmd.Flags().IntVar(&opts.Depth, "depth", opts.Depth, "Maximum scan depth")
```

### 다른 명령어들 (일관됨)
```go
// bulk_common.go:56
cmd.Flags().IntVarP(&flags.Depth, "scan-depth", "d", ..., "directory depth to scan")

// workspace status (status_command.go:51)
cmd.Flags().IntVarP(&opts.ScanDepth, "scan-depth", "d", ..., "Directory scan depth")
```

### clone (Git 표준 - 변경 불필요)
```go
// clone.go:70 - 이건 Git shallow clone depth로 올바름
cloneCmd.Flags().IntVar(&cloneDepth, "depth", 0, "create a shallow clone with truncated history")
```

## Proposed Solution

**workspace scan의 `--depth`를 `--scan-depth, -d`로 변경**

```go
// Before
cmd.Flags().IntVar(&opts.Depth, "depth", opts.Depth, "Maximum scan depth")

// After
cmd.Flags().IntVarP(&opts.Depth, "scan-depth", "d", opts.Depth, "Directory scan depth")
```

**Backward Compatibility**:
- `--depth`를 deprecated alias로 유지
- 사용 시 경고 메시지 출력

## Files Affected

```
pkg/workspacecli/scan_command.go:67  # --depth → --scan-depth
```

## Acceptance Criteria

- [ ] `workspace scan`의 `--depth`를 `--scan-depth, -d`로 변경
- [ ] `--depth`를 deprecated alias로 유지 (경고 출력)
- [ ] help text 업데이트
- [ ] CLAUDE.md 업데이트
- [ ] `make quality` 통과

## Priority Justification

**P1 (High)**:
- 단순 변경이지만 사용자 혼란 직접 유발
- 다른 모든 명령어와 불일치

**Effort: S (Small)**:
- 파일 1개만 수정
- 단순 플래그 이름 변경
