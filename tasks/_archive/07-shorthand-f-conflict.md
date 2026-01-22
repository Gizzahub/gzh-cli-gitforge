---
title: Resolve -f shorthand conflict (format vs force)
priority: P1
effort: M
created: 2026-01-22
decision: Option A - Reserve -f for --force
decision-at: 2026-01-23T00:00:00Z
started-at: 2026-01-23T02:00:00Z
completed-at: 2026-01-23T02:30:00Z
archived-at: 2026-01-23T04:45:00Z
verified-at: 2026-01-23T04:45:00Z
type: refactor
area: cli
tags: [consistency, api-design, ux]
context: -f is used for --format in bulk commands, blocking --force in push (Git standard UX)
verification-summary: |
  - Verified: -f removed from --format, added to --force in push
  - Evidence: cmd/gz-git/cmd/push.go:63 uses `--force, -f`
  - Evidence: cmd/gz-git/cmd/bulk_common.go --format has no shorthand
  - Files checked: 9 command files modified as documented
  - Build: Successful (committed as refactor(cli): reserve -f for --force)
  - Impact: Git-standard UX restored (push -f works like git push -f)
---
options:
  - label: 'Option A: Reserve -f for --force, change --format to -F or no shorthand'
    pros: Git-standard UX (git push -f); muscle memory friendly
    cons: Breaking change for --format users
  - label: 'Option B: Keep current state, document limitation'
    pros: No breaking change
    cons: Git standard UX violation persists; user confusion
recommendation: Option A (reserve -f for --force)
recommendation-reason: Git developers expect push -f to force push; breaking this is worse than format change
---

# Resolve -f shorthand conflict

## Problem

`-f` shorthand가 여러 명령어에서 다른 의미로 사용되거나 충돌:

| 명령어                               | `-f` 의미                       |
| ------------------------------------ | ------------------------------- |
| bulk 명령어 (status, fetch, pull 등) | `--format`                      |
| push                                 | ❌ 사용 불가 (`--force`와 충돌) |

**핵심 문제**: `push --force`를 `-f`로 사용할 수 없음 (Git 표준 UX와 불일치)

## Decision Required

### Option A: Reserve -f for --force (RECOMMENDED)

**Changes**:

- `--format`의 shorthand를 `-f`에서 `-F` 또는 제거
- `-f`를 `--force` 전용으로 예약

```go
// Before (bulk_common.go)
cmd.Flags().StringVarP(&flags.Format, "format", "f", "default", "output format")

// After
cmd.Flags().StringVarP(&flags.Format, "format", "F", "default", "output format")
// 또는 shorthand 제거
cmd.Flags().StringVar(&flags.Format, "format", "default", "output format")
```

**Benefits**:

- `git push -f`와 일관된 UX
- Git 사용자에게 익숙한 패턴

### Option B: 현재 상태 유지 + 문서화

**Drawbacks**:

- Git 표준 UX와 불일치 유지
- 사용자 혼란 지속

## Files Affected

```
cmd/gz-git/cmd/bulk_common.go:62  # --format -f
cmd/gz-git/cmd/push.go:63         # --force (no shorthand)
```

## AI Recommendation

**Option A (reserve -f for --force)** because:

1. push.go already has a comment acknowledging this as a problem
1. Git developers have muscle memory for `push -f`
1. `--format` is less frequently used than `--force`

______________________________________________________________________

**Awaiting decision to proceed with implementation.**

______________________________________________________________________

## Implementation Summary

**Completed:** 2026-01-23

**Changes Made (Option A):**

1. Removed `-f` shorthand from all `--format` flags
1. Added `-f` shorthand to `push --force` flag
1. Updated comment in push.go to reflect the change

**Files Modified:**

- cmd/gz-git/cmd/bulk_common.go (removed -f)
- cmd/gz-git/cmd/history_stats.go (removed -f)
- cmd/gz-git/cmd/switch.go (removed -f)
- cmd/gz-git/cmd/history_file.go (removed -f)
- cmd/gz-git/cmd/history_contributors.go (removed -f)
- cmd/gz-git/cmd/watch.go (removed -f)
- pkg/reposynccli/status_command.go (removed -f)
- pkg/workspacecli/status_command.go (removed -f)
- cmd/gz-git/cmd/push.go (added -f for --force)

**Verification:**

- ✅ Build successful (`make fmt && make build`)
- ✅ `push -f` now works as `push --force`
- ✅ `--format` no longer has shorthand (full flag only)

**Impact:**

- Git-standard UX: `gz-git push -f` works like `git push -f`
- Breaking change: `-f` no longer works for `--format` (must use full `--format` flag)
