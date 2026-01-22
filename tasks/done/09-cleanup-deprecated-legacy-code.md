---
title: Cleanup deprecated, legacy, and unused code
priority: P2
effort: S
created: 2026-01-22
started-at: 2026-01-22T00:00:00Z
completed-at: 2026-01-22T00:00:00Z
type: chore
area: cleanup
tags: [deprecated, legacy, technical-debt]
completion-summary: "Deleted unused pkg/sync/ stub package"
---

# Cleanup deprecated, legacy, and unused code

## Completed Work

### 1. pkg/sync/ 패키지 삭제 ✅

**위치**: `pkg/sync/syncer.go`
**상태**: 미사용, 미구현 stub
**영향**: 없음 (어디서도 import 안 함)

**Verification**:
```bash
grep -r "pkg/sync" --include="*.go" .
# 결과 없음 확인 ✅
```

---

## Deferred Items (Future Cleanup)

### 2. Deprecated --update 플래그/필드 제거

| 위치 | 항목 |
|------|------|
| pkg/repository/bulk_clone.go:44 | `BulkCloneOptions.Update` 필드 |
| cmd/gz-git/cmd/clone.go:30 | `cloneUpdate` 변수 |
| cmd/gz-git/cmd/clone.go:72 | `--update` CLI 플래그 |
| cmd/gz-git/cmd/clone.go:385 | `CloneConfig.Update` 필드 |

**제거 시점**: --strategy 플래그 도입 후 2-3 릴리즈

### 3. Deprecated URLs 필드 제거

| 위치 | 항목 |
|------|------|
| pkg/workspacecli/config_loader.go:49 | `URLs []string` 필드 |

**제거 시점**: 다음 major 버전

### 4. StatusNothingToPush 제거

| 위치 | 항목 |
|------|------|
| pkg/repository/status.go:67 | `StatusNothingToPush` 상수 |

**제거 시점**: 다음 major 버전

---

## Acceptance Criteria

- [x] `pkg/sync/` 디렉토리 삭제
- [x] `make quality` 통과
- [ ] (향후) deprecated 필드들 제거 일정 계획

## Verification

- `make quality` passed
- No import errors after deletion
