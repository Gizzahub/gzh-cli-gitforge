# gz-git 사용법

gz-git은 bulk-first Git CLI로, 여러 repository를 병렬로 관리합니다.

## 명령어 가이드

| 명령어 | 설명 | 문서 |
|--------|------|------|
| `clone` | 여러 repo 병렬 clone | [clone-command.md](clone-command.md) |
| `status` | 여러 repo 상태 확인 | [status-command.md](status-command.md) |
| `sync` | Forge API 동기화 | [sync-command.md](sync-command.md) |
| `workspace` | 로컬 config 기반 관리 | [workspace-command.md](workspace-command.md) |
| `config` | Profile 및 설정 관리 | [config-command.md](config-command.md) |
| `push` | 여러 repo 병렬 push | [push-command.md](push-command.md) |
| `cleanup` | 브랜치 정리 | [cleanup-command.md](cleanup-command.md) |

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
gz-git sync config generate --provider gitlab --org myorg -o .gz-git.yaml

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

## 추가 명령어

| 명령어 | 설명 |
|--------|------|
| `fetch` | 모든 repos fetch |
| `pull` | 모든 repos pull |
| `commit` | 모든 dirty repos commit |
| `switch` | 브랜치 전환 |
| `diff` | 모든 repos diff |
| `update` | 안전한 업데이트 (pull --rebase) |
| `stash` | Stash 관리 |
| `tag` | Tag 관리 |
| `branch` | 브랜치 관리 |
| `info` | Repository 정보 |
| `watch` | 변경 모니터링 |

## 도움말

```bash
# 전체 도움말
gz-git --help

# 명령어별 도움말
gz-git clone --help
gz-git sync --help

# Config 스키마 참조
gz-git schema
```
