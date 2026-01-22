---
title: Standardize --parallel default value across commands
priority: P2
effort: S
created: 2026-01-22
decision: Option A - Unify to 10 for all commands
decision-at: 2026-01-23T00:00:00Z
started-at: 2026-01-23T03:30:00Z
completed-at: 2026-01-23T04:00:00Z
archived-at: 2026-01-23T04:50:00Z
verified-at: 2026-01-23T04:50:00Z
type: refactor
area: cli
tags: [consistency, api-design]
context: --parallel default is 10 for bulk commands but 4 for sync from-forge
verification-summary: |
  - Verified: sync from-forge now uses repository.DefaultBulkParallel (10)
  - Evidence: pkg/reposynccli/from_forge_command.go:50 uses DefaultBulkParallel
  - Evidence: repository package imported for constant access
  - Build: Successful (committed as refactor(cli): standardize parallel default)
  - Impact: All bulk operations now use consistent default (10)
---
options:
  - label: 'Option A: Unify to 10 for all commands'
    pros: Consistent; single constant to manage; faster sync
    cons: May hit API rate limits on sync from-forge
  - label: 'Option B: Keep distinction (local=10, network=4) with documentation'
    pros: Respects network/API constraints
    cons: Users must remember different defaults
recommendation: Option A (unify to 10)
recommendation-reason: Modern APIs handle concurrency well; rate limiting should be explicit not implicit
---

# Standardize --parallel default value

## Problem

`--parallel` 플래그의 기본값이 명령어마다 다름:

| 명령어                                     | 기본값 |
| ------------------------------------------ | ------ |
| bulk 명령어 (status, fetch, pull, push 등) | **10** |
| `sync from-forge`                          | **4**  |

**문제**: 2.5배 차이, 이유 불명확

## Decision Required

### Option A: 모두 10으로 통일 (RECOMMENDED)

**Changes**:

- `sync from-forge`의 기본값을 10으로 변경
- 모든 명령어가 `DefaultBulkParallel` 상수 사용

```go
// from_forge_command.go
Parallel: repository.DefaultBulkParallel,  // 10
```

**Benefits**:

- 일관된 사용자 경험
- 하나의 상수로 관리

**Considerations**:

- API rate limiting이 필요하면 별도 `--rate-limit` 옵션 추가 가능

### Option B: 명령어 특성에 따라 구분

**Changes**:

- 로컬 작업: 10 (bulk)
- 네트워크 작업: 4 (sync from-forge)
- 문서에 이유 명시

**Drawbacks**:

- 사용자가 기본값 차이를 기억해야 함

## Files Affected

```
pkg/reposynccli/from_forge_command.go:49  # Parallel: 4 → 10
pkg/repository/types.go:12               # DefaultBulkParallel = 10 (유지)
```

## AI Recommendation

**Option A (unify to 10)** because:

1. Modern forge APIs (GitHub, GitLab) handle concurrency well
1. If rate limiting is needed, it should be explicit (--rate-limit flag)
1. Simpler mental model for users

______________________________________________________________________

**Awaiting decision to proceed with implementation.**

---

## Implementation Summary

**Completed:** 2026-01-23

**Changes Made (Option A):**

1. Updated `sync from-forge` default parallel value from 4 to 10
1. Changed to use `repository.DefaultBulkParallel` constant
1. Added import for repository package

**Files Modified:**

- pkg/reposynccli/from_forge_command.go:
  - Added `repository` import
  - Changed `Parallel: 4` → `Parallel: repository.DefaultBulkParallel`

**Verification:**

- ✅ Build successful (`make fmt && make build`)
- ✅ All commands now use consistent parallel default (10)
- ✅ Single constant (`DefaultBulkParallel`) manages default value

**Impact:**

- Consistent user experience across all bulk operations
- Faster sync from forge (2.5x default parallelism increase)
- Single source of truth for parallel default value
