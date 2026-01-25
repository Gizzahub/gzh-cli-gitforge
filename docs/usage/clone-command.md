# gz-git clone

여러 repository를 병렬로 clone하는 명령어.

## 기본 사용법

```bash
# URL로 clone
gz-git clone --url https://github.com/user/repo1.git --url https://github.com/user/repo2.git

# 파일에서 URL 읽기
gz-git clone --file repos.txt

# Config 파일 사용
gz-git clone -c repos-config.yaml .
```

## Config 형식

### Flat 형식 (단순 목록)

```yaml
strategy: pull
parallel: 10

repositories:
  - https://github.com/user/repo1.git
  - url: https://github.com/user/repo2.git
    name: custom-name
    branch: develop
```

### Named Groups 형식 (그룹별 관리)

```yaml
strategy: pull
parallel: 10

core:
  target: "."
  repositories:
    - https://github.com/discourse/discourse_docker.git
    - url: https://github.com/discourse/discourse.git
      name: discourse_app
      branch: stable

plugins:
  target: plugins
  branch: develop
  repositories:
    - url: ssh://git@gitlab.com/org/plugin1.git
    - url: ssh://git@gitlab.com/org/plugin2.git
  hooks:
    after:
      - ./post-install.sh
```

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--url` | Clone할 URL (반복 가능) | - |
| `--file` | URL 목록 파일 | - |
| `-c, --config` | YAML config 파일 | - |
| `-g, --group` | 특정 그룹만 clone (반복 가능) | 전체 |
| `-b, --branch` | Checkout할 branch | repo default |
| `-s, --update-strategy` | 기존 repo 처리: skip, pull, reset, rebase, fetch | skip |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-n, --dry-run` | 실행하지 않고 미리보기 | false |
| `--structure` | 디렉토리 구조: flat, user | flat |
| `--depth` | Shallow clone 깊이 | 0 (전체) |
| `--submodules` | Submodule 초기화 | false |

## 그룹 선택

```bash
# core 그룹만 clone
gz-git clone -c repos-config.yaml . -g core

# 여러 그룹 선택
gz-git clone -c repos-config.yaml . -g core -g plugins
```

## Update Strategy

기존에 이미 clone된 repo가 있을 때 처리 방법:

| Strategy | 동작 |
|----------|------|
| `skip` | 건너뛰기 (기본값) |
| `pull` | git pull 실행 |
| `reset` | git fetch + reset --hard |
| `rebase` | git pull --rebase |
| `fetch` | git fetch만 실행 |

```bash
# 기존 repo는 pull로 업데이트
gz-git clone -c config.yaml . --update-strategy pull
```

## Hooks

Clone 전후에 명령 실행:

```yaml
plugins:
  target: plugins
  repositories:
    - url: https://github.com/user/repo.git
      hooks:
        before:
          - echo "Starting clone..."
        after:
          - ./setup.sh
  hooks:
    after:
      - ./install-all.sh  # 그룹 전체 완료 후 실행
```

## 예제

### GitHub organization clone

```bash
# 먼저 sync로 config 생성
gz-git sync config generate --provider github --org myorg -o repos.yaml

# 생성된 config로 clone
gz-git clone -c repos.yaml .
```

### 프로젝트 초기화 스크립트

```bash
#!/bin/bash
# bin/init-main.sh

set -e
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

gz-git clone -c "$PROJECT_ROOT/repos-config.yaml" "$PROJECT_ROOT"
```
