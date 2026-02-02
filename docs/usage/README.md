# gz-git 사용법

gz-git은 bulk-first Git CLI로, 여러 repository를 병렬로 관리합니다.

## 명령어 가이드

### 핵심 명령어

| 명령어 | 설명 | 문서 |
|--------|------|------|
| `clone` | 여러 repo 병렬 clone | [clone-command.md](clone-command.md) |
| `status` | 여러 repo 상태 확인 | [status-command.md](status-command.md) |
| `fetch` | 원격 변경사항 가져오기 | [fetch-command.md](fetch-command.md) |
| `pull` | 변경사항 가져오기 + 병합 | [pull-command.md](pull-command.md) |
| `update` | 안전한 업데이트 (pull --rebase) | [update-command.md](update-command.md) |
| `push` | 여러 repo 병렬 push | [push-command.md](push-command.md) |

### 개발 작업

| 명령어 | 설명 | 문서 |
|--------|------|------|
| `commit` | 여러 repo 일괄 커밋 | [commit-command.md](commit-command.md) |
| `switch` | 브랜치 전환 | [switch-command.md](switch-command.md) |
| `diff` | 변경사항 확인 | [diff-command.md](diff-command.md) |
| `cleanup` | 브랜치 정리 | [cleanup-command.md](cleanup-command.md) |

### 고급 기능

| 명령어 | 설명 | 문서 |
|--------|------|------|
| `forge` | Forge API 동기화 | [forge-command.md](forge-command.md) |
| `workspace` | 로컬 config 기반 관리 | [workspace-command.md](workspace-command.md) |
| `config` | Profile 및 설정 관리 | [config-command.md](config-command.md) |

## 빠른 시작

### 1. 설치

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

### 2. 기본 사용

```bash
# 상태 확인 (현재 디렉토리 + 1레벨 하위)
gz-git status

# 모든 repos fetch
gz-git fetch

# 모든 repos pull
gz-git pull
```

### 일일 워크플로우

```bash
# 아침: 변경사항 가져오기
gz-git fetch ~/workspace    # 안전하게 fetch만
gz-git status ~/workspace   # 상태 확인
gz-git update ~/workspace   # Rebase로 업데이트

# 개발 중: 변경사항 확인
gz-git diff ~/workspace           # 전체 diff
gz-git diff --no-content ~/workspace  # 파일 목록만

# 저녁: 커밋 & 푸시
gz-git commit ~/workspace    # Preview 모드
gz-git commit -m "..." --yes # 실제 커밋
gz-git push ~/workspace      # Push
```

### 3. Config 기반 관리

```bash
# 디렉토리 스캔 → config 생성
gz-git workspace init .

# Config 기반 동기화
gz-git workspace sync
```

### 4. Forge 연동

```bash
# GitLab org에서 config 생성
gz-git forge config generate --provider gitlab --org myorg -o .gz-git.yaml

# 동기화
gz-git workspace sync
```

## Config 형식

### Flat (단순 목록)

```yaml
repositories:
  - url: https://github.com/user/repo1.git
  - url: https://github.com/user/repo2.git
    branch: develop
```

### Named Groups (그룹별)

```yaml
core:
  target: "."
  repositories:
    - https://github.com/org/core.git

plugins:
  target: plugins
  repositories:
    - https://github.com/org/plugin1.git
    - https://github.com/org/plugin2.git
```

## 공통 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-n, --dry-run` | 미리보기 | false |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |
| `-f, --format` | 출력 형식 | default |
| `-v, --verbose` | 상세 출력 | false |
| `-q, --quiet` | 에러만 출력 | false |

## 기타 명령어

| 명령어 | 설명 | 상태 |
|--------|------|------|
| `stash` | Stash 관리 | 사용 가능 |
| `tag` | Tag 관리 | 사용 가능 |
| `branch` | 브랜치 관리 | 사용 가능 |
| `info` | Repository 정보 | 사용 가능 |
| `watch` | 변경 모니터링 | 사용 가능 |

## 도움말

```bash
# 전체 도움말
gz-git --help

# 명령어별 도움말
gz-git clone --help
gz-git forge --help

# Config 스키마 참조
gz-git schema
```
