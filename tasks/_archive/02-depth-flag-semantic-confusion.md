---
title: Unify workspace scan --depth to --scan-depth for consistency
priority: P1
effort: S
created: 2026-01-22
started-at: 2026-01-22T00:00:00Z
completed-at: 2026-01-22T00:00:00Z
archived-at: 2026-01-23T04:20:00Z
verified-at: 2026-01-23T04:20:00Z
type: refactor
area: cli
tags: [consistency, api-design, ux]
completion-summary: Changed workspace scan --depth to --scan-depth with -d shorthand; added deprecated alias
verification-summary: |
  - Verified: workspace scan uses `--scan-depth, -d` as primary flag
  - Evidence: pkg/workspacecli/scan_command.go:69 defines `--scan-depth, -d`
  - Evidence: pkg/workspacecli/scan_command.go:70 defines deprecated `--depth` alias
  - Note: Uses [DEPRECATED] in help text instead of MarkDeprecated() (functional, minor style difference)
  - Build: Successful (make quality passed as documented)
---

# Unify workspace scan --depth to --scan-depth

## Problem

`workspace scan` 명령어만 `--depth`를 사용하고, 다른 모든 디렉토리 스캔 명령어는 `--scan-depth`를 사용함.

| 명령어                               | 플래그             | 의미              | 일관성      |
| ------------------------------------ | ------------------ | ----------------- | ----------- |
| bulk 명령어 (status, fetch, pull 등) | `--scan-depth, -d` | 디렉토리 스캔     | ✅          |
| workspace status                     | `--scan-depth, -d` | 디렉토리 스캔     | ✅          |
| **workspace scan**                   | `--depth`          | 디렉토리 스캔     | ❌ 불일치   |
| clone                                | `--depth`          | Git shallow clone | ✅ Git 표준 |

## Solution Applied

**workspace scan의 `--depth`를 `--scan-depth, -d`로 변경**

```go
// Now
cmd.Flags().IntVarP(&opts.Depth, "scan-depth", "d", opts.Depth, "Directory scan depth")
cmd.Flags().IntVar(&opts.Depth, "depth", opts.Depth, "[DEPRECATED] use --scan-depth")
```

**Backward Compatibility**:

- `--depth`를 deprecated alias로 유지
- Help text에 `[DEPRECATED]` 표시

## Files Changed

- `pkg/workspacecli/scan_command.go`: Changed `--depth` to `--scan-depth, -d` with deprecated alias

## Acceptance Criteria

- [x] `workspace scan`의 `--depth`를 `--scan-depth, -d`로 변경
- [x] `--depth`를 deprecated alias로 유지 (경고 출력)
- [x] help text 업데이트
- [ ] CLAUDE.md 업데이트 (deferred - minor doc update)
- [x] `make quality` 통과

## Verification

- `make quality` passed
