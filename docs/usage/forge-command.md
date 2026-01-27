# gz-git forge

Git Forge (GitHub, GitLab, Gitea) API를 통한 repository 동기화/설정 생성.

## 서브커맨드

| 커맨드 | 설명 |
|--------|------|
| `from` | Forge에서 직접 clone/update |
| `config generate` | Forge API → config 파일 생성 |
| `status` | Repository health 진단 |
| `setup` | Interactive 설정 마법사 |

## from

Forge API에서 organization의 모든 repo를 직접 동기화.

```bash
# GitHub
gz-git forge from \
  --provider github \
  --org myorg \
  --path ~/repos \
  --token $GITHUB_TOKEN

# GitLab (self-hosted)
gz-git forge from \
  --provider gitlab \
  --org mygroup \
  --path ~/repos \
  --base-url https://gitlab.company.com \
  --token $GITLAB_TOKEN \
  --include-subgroups

# Gitea
gz-git forge from \
  --provider gitea \
  --org myorg \
  --path ~/repos \
  --base-url https://gitea.company.com \
  --token $GITEA_TOKEN
```

### 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--provider` | github, gitlab, gitea | - |
| `--org` | Organization/Group 이름 | - |
| `--path` | Clone 대상 디렉토리 | - (required) |
| `--base-url` | API endpoint (self-hosted) | provider default |
| `--token` | 인증 토큰 | - |
| `--clone-proto` | Clone 프로토콜: ssh, https | ssh |
| `--ssh-port` | SSH 포트 (비표준) | - |
| `--include-subgroups` | GitLab 하위 그룹 포함 | false |
| `--subgroup-mode` | flat, nested | flat |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-n, --dry-run` | 미리보기 | false |

### Subgroup Mode

GitLab 하위 그룹 처리 방식:

```bash
# flat: 대시로 연결 (parent-child-repo)
gz-git forge from --include-subgroups --subgroup-mode flat

# nested: 디렉토리 구조 (parent/child/repo)
gz-git forge from --include-subgroups --subgroup-mode nested
```

## config generate

Forge API에서 config 파일 생성 (이후 `workspace sync`로 사용).

```bash
# 기본 (compact 출력)
gz-git forge config generate \
  --provider gitlab \
  --org devbox \
  -o .gz-git.yaml

# 모든 필드 포함
gz-git forge config generate \
  --provider gitlab \
  --org devbox \
  -o .gz-git.yaml \
  --full

# stdout으로 출력
gz-git forge config generate --provider github --org myorg
```

### 생성되는 config 형식

```yaml
# .gz-git.yaml
version: 1
kind: repositories

metadata:
  name: devbox
  repository: https://gitlab.company.com/devbox

strategy: pull
parallel: 10

repositories:
  - name: project1
    url: ssh://git@gitlab.company.com:2224/devbox/project1.git
    branch: master

  - name: project2
    url: ssh://git@gitlab.company.com:2224/devbox/project2.git
    branch: develop
```

## status

Repository health 진단 (모든 remote fetch 포함).

```bash
# Config 기반
gz-git forge status -c .gz-git.yaml

# 디렉토리 스캔
gz-git forge status --path ~/repos --scan-depth 2

# 빠른 체크 (fetch 생략)
gz-git forge status --skip-fetch

# Custom timeout
gz-git forge status --timeout 60s

# 상세 출력
gz-git forge status --verbose
```

### 출력 예시

```
Checking repository health...

✓ gzh-cli (master)              healthy     up-to-date
⚠ gzh-cli-gitforge (develop)   warning     3↓ 2↑ diverged
  → Diverged: 2 ahead, 3 behind. Use 'git pull --rebase'
✗ gzh-cli-quality (main)        error       dirty + 5↓ behind
  → Commit or stash 3 modified files, then pull
⊘ gzh-cli-template (master)     timeout     fetch failed (30s)
  → Check network connection

Summary: 1 healthy, 1 warning, 1 error, 1 unreachable
```

### Health Status

| Status | 의미 | 권장 조치 |
|--------|------|----------|
| `healthy` | 최신, clean | 없음 |
| `warning` | diverged/behind | pull/rebase |
| `error` | dirty + behind | stash/commit → pull |
| `timeout` | 네트워크 문제 | 연결 확인 |

## setup

Interactive 설정 마법사.

```bash
gz-git forge setup
```

단계별로 provider, org, 인증 등을 설정하고 config 파일 생성.

## 워크플로우 예제

### 1. 새 프로젝트 시작

```bash
# 1. Config 생성
gz-git forge config generate --provider gitlab --org myteam -o .gz-git.yaml

# 2. Clone
gz-git workspace sync

# 3. 상태 확인
gz-git forge status
```

### 2. 정기 동기화

```bash
#!/bin/bash
# daily-sync.sh

cd ~/mydevbox

# Health check 먼저
gz-git forge status --timeout 30s

# 문제 없으면 동기화
if [ $? -eq 0 ]; then
    gz-git workspace sync
fi
```

### 3. Profile과 함께 사용

```bash
# Profile 설정
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token $WORK_TOKEN

gz-git config profile use work

# 이제 --provider, --token 생략 가능
gz-git forge from --org myteam --path ~/work
```
