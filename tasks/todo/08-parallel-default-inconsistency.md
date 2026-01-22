---
title: Standardize --parallel default value across commands
priority: P2
effort: S
created: 2026-01-22
status: todo
type: refactor
area: cli
tags: [consistency, api-design]
---

# Standardize --parallel default value

## Problem

`--parallel` 플래그의 기본값이 명령어마다 다름:

| 명령어 | 기본값 | 파일 |
|--------|--------|------|
| bulk 명령어 (status, fetch, pull, push 등) | **10** | bulk_common.go:57 |
| `sync from-forge` | **4** | from_forge_command.go:49 |

**문제**: 2.5배 차이, 이유 불명확

## Current State

### bulk_common.go:57
```go
cmd.Flags().IntVarP(&flags.Parallel, "parallel", "j", repository.DefaultBulkParallel,
    "number of parallel operations")
```

### repository/types.go
```go
const DefaultBulkParallel = 10
```

### from_forge_command.go:49
```go
Parallel: 4,  // default value in FromForgeOptions
```

## 분석

**sync from-forge가 4인 이유 추정**:
- API rate limiting 고려?
- 네트워크 부하 고려?
- 명시적 이유 없음 (주석 없음)

**bulk 명령어가 10인 이유**:
- 로컬 git 작업은 빠름
- 병렬 처리 효율성

## Proposed Solution

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
- API rate limiting이 필요하면 별도 옵션 추가

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

## Acceptance Criteria

- [ ] **Decision**: Option A (통일) 또는 Option B (구분) 선택
- [ ] **Implementation**:
  - [ ] `sync from-forge` 기본값을 `DefaultBulkParallel`로 변경
  - [ ] 또는 별도 상수 정의 및 문서화
- [ ] **Documentation**:
  - [ ] CLAUDE.md에 기본값 명시
  - [ ] 기본값 선택 이유 주석 추가
- [ ] **Quality**:
  - [ ] `make quality` 통과

## Priority Justification

**P2 (Medium)**:
- 기능에 영향 없음 (사용자가 명시적 지정 가능)
- 일관성 개선 목적
- 빠른 수정 가능 (파일 1개)
