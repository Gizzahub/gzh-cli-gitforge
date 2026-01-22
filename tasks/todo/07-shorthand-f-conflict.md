---
title: Resolve -f shorthand conflict (format vs force vs file)
priority: P1
effort: M
created: 2026-01-22
status: todo
type: refactor
area: cli
tags: [consistency, api-design, ux]
---

# Resolve -f shorthand conflict

## Problem

`-f` shorthand가 여러 명령어에서 다른 의미로 사용되거나 충돌로 인해 사용 불가:

| 명령어 | `-f` 의미 | 파일 |
|--------|----------|------|
| bulk 명령어 (status, fetch, pull 등) | `--format` | bulk_common.go:62 |
| clone | `--config` (file) | clone.go:77 |
| push | ❌ 사용 불가 (`--force`와 충돌) | push.go:63 (주석) |

## Current State

### bulk_common.go:62
```go
cmd.Flags().StringVarP(&flags.Format, "format", "f", "default", "output format")
```

### clone.go:77
```go
cloneCmd.Flags().StringVarP(&cloneConfig, "config", "c", "", "YAML config file")
// 참고: -f가 아닌 -c 사용 중 (다행히 충돌 없음)
```

### push.go:63 (주석)
```go
// Note: no -f shorthand for force, conflicts with --format in bulk commands
pushCmd.Flags().Bool("force", false, "force push")
```

## 분석

실제 확인 결과:
- **clone**: `-c`를 `--config`에 사용 (✅ 문제 없음)
- **bulk commands**: `-f`를 `--format`에 사용
- **push**: `--force`에 `-f` 사용 불가 (bulk의 `--format`과 충돌)

**핵심 문제**: `push --force`를 `-f`로 사용할 수 없음 (Git 표준 UX와 불일치)

## Proposed Solution

### Option A: push에서 -f를 --force로 사용 (RECOMMENDED)

**Changes**:
- `--format`의 shorthand를 `-f`에서 다른 것으로 변경
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

**Changes**:
- 현재 상태 유지
- 문서에 `-f` 충돌 명시

**Drawbacks**:
- Git 표준 UX와 불일치 유지
- 사용자 혼란 지속

## Files Affected

```
cmd/gz-git/cmd/bulk_common.go:62  # --format -f
cmd/gz-git/cmd/push.go:63         # --force (no shorthand)
```

## Acceptance Criteria

- [ ] **Decision**: Option A (shorthand 변경) 또는 Option B (유지) 선택
- [ ] **Implementation** (if Option A):
  - [ ] `--format`의 `-f` shorthand 제거 또는 `-F`로 변경
  - [ ] `push --force`에 `-f` shorthand 추가
  - [ ] 기존 `-f` 사용자를 위한 deprecation 경고
- [ ] **Documentation**:
  - [ ] CLAUDE.md 업데이트
  - [ ] shorthand 변경 사항 문서화
- [ ] **Testing**:
  - [ ] 새 shorthand 테스트
  - [ ] push -f 테스트
- [ ] **Quality**:
  - [ ] `make quality` 통과

## Priority Justification

**P1 (High)**:
- Git 표준 UX (`git push -f`)와 불일치
- 개발자 muscle memory와 충돌
- push.go에 이미 문제 인식 주석 있음
