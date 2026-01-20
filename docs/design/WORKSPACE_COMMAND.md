# Workspace Command Design

**Date**: 2026-01-21
**Status**: Approved

## Overview

`workspace` 명령어를 새로 추가하여 로컬 config 기반 작업을 `sync`에서 분리한다.

### 목표

- **책임 분리**: `sync` = Forge API, `workspace` = 로컬 config
- **명확한 워크플로우**: config 생성 → 동기화 → 상태 확인
- **기존 명령어 유지**: 개별 bulk 명령어(clone, pull, push 등)는 그대로

## 명령어 구조

### sync (Forge 전용)

```
sync
├── from-forge         # Forge API → clone/update
├── config generate    # Forge API → config 출력
└── status             # Forge 동기화 후 상태 확인
```

### workspace (로컬 config 기반)

```
workspace
├── init               # 빈 config 생성
├── scan               # 디렉토리 스캔 → config 생성
├── sync               # config → clone/update
├── status             # health check (로컬 기준)
├── add                # repo 추가
└── validate           # config 검증
```

### 개별 명령어 (유지)

```
clone, update, pull, push, fetch, status, switch, commit, diff, ...
```

플래그 기반 빠른 작업용으로 그대로 유지.

## Config 파일

- **기본값**: `.gz-git.yaml`
- **사용자 지정**: `-c <파일명>` 플래그

```bash
# 기본 config
workspace init                      # .gz-git.yaml 생성
workspace sync                      # .gz-git.yaml 사용

# 커스텀 config
workspace init -c myworkspace.yaml
workspace sync -c myworkspace.yaml
```

## 마이그레이션

### 즉시 제거 (breaking change)

| 기존 명령어 | 신규 명령어 |
|-------------|-------------|
| `sync from-config` | `workspace sync` |
| `sync config scan` | `workspace scan` |
| `sync config init` | `workspace init` |
| `sync config validate` | `workspace validate` |
| `sync config merge` | `workspace add` 또는 제거 |

### 유지

| 명령어 | 비고 |
|--------|------|
| `sync from-forge` | Forge API 동기화 |
| `sync config generate` | Forge API → config 출력 |
| `sync status` | Forge 기준 상태 확인 |

## 명령어 상세

### workspace init

빈 config 파일 생성.

```bash
workspace init                    # .gz-git.yaml
workspace init -c custom.yaml     # 커스텀 이름
```

### workspace scan

디렉토리 스캔하여 git repository 목록을 config로 생성.

```bash
workspace scan                           # 현재 디렉토리
workspace scan ~/mydevbox                # 특정 경로
workspace scan --depth 3                 # 스캔 깊이
workspace scan --exclude "vendor,tmp"    # 제외 패턴
workspace scan -o workspace.yaml         # 출력 파일
```

기존 `sync config scan`의 기능을 그대로 이전.

### workspace sync

Config 기반으로 repository들을 clone/update.

```bash
workspace sync                    # .gz-git.yaml 사용
workspace sync -c custom.yaml     # 커스텀 config
workspace sync --dry-run          # 미리보기
workspace sync --strategy pull    # pull 전략
workspace sync --parallel 10      # 병렬 처리
```

기존 `sync from-config`의 기능을 그대로 이전.

### workspace status

로컬 workspace의 health check.

```bash
workspace status                  # .gz-git.yaml 기준
workspace status -c custom.yaml   # 커스텀 config
workspace status --verbose        # 상세 출력
```

### workspace add

Config에 repository 추가.

```bash
workspace add https://github.com/user/repo.git
workspace add --name myrepo --url git@github.com:user/repo.git
workspace add --from-current      # 현재 디렉토리의 repo 추가
```

### workspace validate

Config 파일 문법 및 구조 검증.

```bash
workspace validate                # .gz-git.yaml
workspace validate -c custom.yaml
```

## 구현 순서

1. `workspace` 루트 명령어 생성
2. `workspace scan` (기존 코드 이전)
3. `workspace sync` (기존 코드 이전)
4. `workspace status` (기존 코드 이전)
5. `workspace init` (기존 코드 이전)
6. `workspace validate` (기존 코드 이전)
7. `workspace add` (신규 또는 merge 이전)
8. 기존 `sync config *`, `sync from-config` 제거
9. CLAUDE.md, 문서 업데이트

## 파일 변경 예상

### 신규 파일

```
pkg/workspacecli/
├── factory.go
├── root_command.go
├── init_command.go
├── scan_command.go
├── sync_command.go
├── status_command.go
├── add_command.go
└── validate_command.go
```

### 수정 파일

```
cmd/gz-git/cmd/root.go      # workspace 명령어 등록
cmd/gz-git/cmd/sync.go      # from-config 관련 제거
```

### 삭제 파일

```
pkg/reposynccli/from_config_command.go
pkg/reposynccli/config_scan_command.go
pkg/reposynccli/config_init_command.go
pkg/reposynccli/config_validate_command.go
pkg/reposynccli/config_merge_command.go  # add로 대체 또는 삭제
```

## Breaking Changes

- `sync from-config` → `workspace sync`
- `sync config scan` → `workspace scan`
- `sync config init` → `workspace init`
- `sync config validate` → `workspace validate`
- `sync config merge` → `workspace add` 또는 제거

버전 업데이트 시 major 또는 minor bump 필요.
