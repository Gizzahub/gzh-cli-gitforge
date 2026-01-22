---
title: Resolve --strategy flag semantic conflict across commands
priority: P1
effort: M
created: 2026-01-22
status: todo
type: refactor
area: cli
tags: [consistency, api-design, ux, breaking-change]
---

# Resolve --strategy flag semantic conflict

## Problem

`--strategy` 플래그가 명령어마다 완전히 다른 의미로 사용됨:

| 명령어 | 의미 | 값 | 파일 |
|--------|------|-----|------|
| `pull --strategy` | Git merge 전략 | `merge, rebase, ff-only` | pull.go:63 |
| `clone --strategy` | 기존 repo 처리 방식 | `skip, pull, reset, rebase, fetch` | clone.go:71 |
| `sync from-forge --strategy` | 동기화 전략 | `reset, pull, fetch` | from_forge_command.go:99 |
| `workspace sync --strategy` | 동기화 전략 | (sync와 동일) | sync_command.go |

**핵심 문제**:
- `pull --strategy rebase` = Git rebase로 merge
- `clone --strategy rebase` = 기존 repo를 rebase로 업데이트
- 같은 `rebase` 값이 다른 동작을 의미함

## Current State

### pull command (Git merge strategy)
```go
// cmd/gz-git/cmd/pull.go:63
pullCmd.Flags().StringVar(&pullStrategy, "strategy", "rebase",
    "pull strategy: merge, rebase, ff-only")
```

### clone command (Existing repo handling)
```go
// cmd/gz-git/cmd/clone.go:71
cloneCmd.Flags().StringVarP(&cloneStrategy, "strategy", "s", "",
    "existing repo strategy: skip (default), pull, reset, rebase, fetch")
```

### sync from-forge (Sync strategy)
```go
// pkg/reposynccli/from_forge_command.go:99
cmd.Flags().StringVar(&opts.Strategy, "strategy", "reset",
    "Sync strategy (reset, pull, fetch)")
```

## Proposed Solution

### Option A: Rename pull's --strategy (RECOMMENDED)

**Changes**:
- `pull --strategy` → `pull --merge-strategy` 또는 `--pull-mode`
- `clone`, `sync`의 `--strategy`는 유지 (일관된 의미: repo 처리 방식)

```bash
# Before
gz-git pull --strategy rebase

# After
gz-git pull --merge-strategy rebase
# 또는
gz-git pull --pull-mode rebase
```

**Benefits**:
- pull만 변경하면 됨 (영향 범위 최소화)
- clone/sync는 이미 일관된 의미로 사용 중
- Git 용어와 구분됨

### Option B: 모든 명령어에서 rename

**Changes**:
- `pull --strategy` → `--merge-strategy`
- `clone --strategy` → `--update-strategy` 또는 `--existing-strategy`
- `sync --strategy` → `--sync-strategy`

**Drawbacks**:
- 변경 범위가 큼
- clone/sync는 이미 task #01에서 정의된 strategy 사용 중

## Files Affected

```
cmd/gz-git/cmd/pull.go:63           # --strategy (merge strategy)
cmd/gz-git/cmd/clone.go:71          # --strategy (repo handling) - 유지
pkg/reposynccli/from_forge_command.go:99  # --strategy (sync) - 유지
pkg/workspacecli/sync_command.go    # --strategy (sync) - 유지
```

## Acceptance Criteria

- [ ] **Decision**: Option A (pull만 변경) 또는 Option B 선택
- [ ] **Implementation**:
  - [ ] pull의 `--strategy`를 `--merge-strategy`로 변경
  - [ ] `--strategy`를 deprecated alias로 유지 (경고 출력)
  - [ ] help text 업데이트
- [ ] **Documentation**:
  - [ ] CLAUDE.md 업데이트
  - [ ] 명령어별 strategy 의미 문서화
- [ ] **Testing**:
  - [ ] 새 플래그 테스트
  - [ ] deprecated alias 테스트
- [ ] **Quality**:
  - [ ] `make quality` 통과

## Breaking Change 고려

**Backward Compatibility**:
```go
// pull.go - deprecated alias 추가
pullCmd.Flags().StringVar(&pullStrategy, "merge-strategy", "rebase", "...")
pullCmd.Flags().StringVar(&pullStrategyDeprecated, "strategy", "", "[DEPRECATED] use --merge-strategy")
```

## Priority Justification

**P1 (High)**:
- 같은 플래그명에 다른 의미 = 사용자 혼란 직접 유발
- `rebase` 값이 두 곳에서 다른 동작 수행
