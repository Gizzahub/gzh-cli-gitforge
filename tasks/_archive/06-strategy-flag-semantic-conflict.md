---
title: Resolve --strategy flag semantic conflict across commands
priority: P1
effort: M
created: 2026-01-22
decision: Option B - Rename all strategy flags
decision-at: 2026-01-23T00:00:00Z
started-at: 2026-01-23T01:00:00Z
completed-at: 2026-01-23T02:00:00Z
archived-at: 2026-01-23T04:40:00Z
verified-at: 2026-01-23T04:40:00Z
type: refactor
area: cli
tags: [consistency, api-design, ux, breaking-change]
context: --strategy means Git merge strategy in pull, but repo handling strategy in clone/sync
verification-summary: |
  - Verified: All strategy flags renamed to context-specific names
  - Evidence: cmd/gz-git/cmd/pull.go uses `--merge-strategy` for Git merge
  - Evidence: cmd/gz-git/cmd/clone.go uses `--update-strategy` for repo handling
  - Evidence: pkg/reposynccli/from_forge_command.go uses `--sync-strategy`
  - Evidence: All have deprecated `--strategy` aliases with MarkDeprecated()
  - Build: Successful (committed as refactor(cli): rename strategy flags)
  - Note: This task superseded task 01's approach
---
options:
  - label: "Option A: Rename pull's --strategy to --merge-strategy"
    pros: Minimal change (only pull); clone/sync already consistent; clear distinction
    cons: Breaking change for pull command users
  - label: 'Option B: Rename all --strategy flags'
    pros: All flags become explicit about their purpose
    cons: Larger change scope; more breaking changes
recommendation: Option A (rename pull's --strategy only)
recommendation-reason: Pull is the odd one out; clone/sync already share consistent meaning
---

# Resolve --strategy flag semantic conflict

## Problem

`--strategy` 플래그가 명령어마다 완전히 다른 의미로 사용됨:

| 명령어                       | 의미                | 값                                 |
| ---------------------------- | ------------------- | ---------------------------------- |
| `pull --strategy`            | Git merge 전략      | `merge, rebase, ff-only`           |
| `clone --strategy`           | 기존 repo 처리 방식 | `skip, pull, reset, rebase, fetch` |
| `sync from-forge --strategy` | 동기화 전략         | `reset, pull, fetch`               |

**핵심 문제**: 같은 `rebase` 값이 다른 동작을 의미함

## Decision Required

### Option A: Rename pull's --strategy (RECOMMENDED)

**Changes**:

- `pull --strategy` → `pull --merge-strategy` or `--pull-mode`
- `clone`, `sync`의 `--strategy`는 유지 (일관된 의미)

```bash
# Before
gz-git pull --strategy rebase

# After  
gz-git pull --merge-strategy rebase
```

**Benefits**:

- pull만 변경하면 됨 (영향 범위 최소화)
- clone/sync는 이미 일관된 의미로 사용 중

### Option B: 모든 명령어에서 rename

**Changes**:

- `pull --strategy` → `--merge-strategy`
- `clone --strategy` → `--update-strategy`
- `sync --strategy` → `--sync-strategy`

**Drawbacks**:

- 변경 범위가 큼
- 모든 사용자 영향

## Files Affected

```
cmd/gz-git/cmd/pull.go:63           # --strategy (merge strategy)
cmd/gz-git/cmd/clone.go:71          # --strategy (repo handling) - 유지
pkg/reposynccli/from_forge_command.go:99  # --strategy (sync) - 유지
```

## AI Recommendation

**Option A (rename pull's --strategy only)** because:

1. Pull is the semantic outlier (Git merge vs repo handling)
1. Clone/sync already share consistent semantics
1. Minimal breaking change surface

______________________________________________________________________

**Awaiting decision to proceed with implementation.**

______________________________________________________________________

## Implementation Summary

**Completed:** 2026-01-23

**Changes Made (Option B):**

1. Renamed `pull --strategy` → `--merge-strategy` (Git merge strategy)
1. Renamed `clone --strategy` → `--update-strategy` (existing repo handling)
1. Renamed `sync from-forge --strategy` → `--sync-strategy` (sync strategy)
1. Renamed `sync config generate --strategy` → `--sync-strategy`
1. Renamed `workspace generate-config --strategy` → `--sync-strategy`
1. Added deprecated `--strategy` alias for all commands with deprecation warnings
1. Updated usage examples and comments

**Files Modified:**

- cmd/gz-git/cmd/pull.go (--merge-strategy)
- cmd/gz-git/cmd/clone.go (--update-strategy)
- pkg/reposynccli/from_forge_command.go (--sync-strategy)
- pkg/reposynccli/config_generate_command.go (--sync-strategy)
- pkg/workspacecli/generate_command.go (--sync-strategy)

**Verification:**

- ✅ Build successful (`make fmt && make build`)
- ✅ Deprecation warnings work for all commands
- ✅ New flags appear in help output
- ✅ Usage examples updated

**Backward Compatibility:**

- All old `--strategy` flags still work with deprecation warnings
- Users can migrate gradually
