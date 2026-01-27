# gz-git config

Profile 및 설정 관리.

## 서브커맨드

| 커맨드 | 설명 |
|--------|------|
| `init` | Config 디렉토리 초기화 |
| `profile` | Profile 관리 (create/use/list/show/delete) |
| `show` | 현재 설정 표시 |
| `hierarchy` | Config 계층 트리 표시 |

## init

Config 디렉토리 초기화.

```bash
gz-git config init
```

생성되는 구조:

```
~/.config/gz-git/
├── config.yaml           # Global config
├── profiles/
│   └── default.yaml      # Default profile
└── state/
    └── active-profile.txt
```

## profile

### create

새 profile 생성.

```bash
# Interactive
gz-git config profile create work

# 플래그로 직접 지정
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token ${WORK_TOKEN} \
  --clone-proto ssh \
  --ssh-port 2224 \
  --parallel 10
```

### use

활성 profile 전환.

```bash
gz-git config profile use work
gz-git config profile use personal
```

### list

모든 profile 목록.

```bash
gz-git config profile list
```

출력:

```
Available profiles:
  default
* work      (active)
  personal
  opensource
```

### show

Profile 상세 정보.

```bash
gz-git config profile show work
```

### delete

Profile 삭제.

```bash
gz-git config profile delete old-profile
```

## show

현재 설정 표시.

```bash
# Project config (.gz-git.yaml)
gz-git config show

# Effective config (모든 레이어 병합)
gz-git config show --effective

# JSON 형식
gz-git config show --format json
```

### Effective Config

5-layer precedence를 적용한 최종 설정:

```bash
gz-git config show --effective
```

출력:

```
Effective Configuration:
========================
provider: gitlab          (from: profile:work)
baseURL: https://gitlab.company.com  (from: profile:work)
token: ***                (from: env:GITLAB_TOKEN)
cloneProto: ssh           (from: project:.gz-git.yaml)
parallel: 10              (from: default)
```

## hierarchy

Config 계층 트리 표시.

```bash
gz-git config hierarchy
```

출력:

```
Config Hierarchy:
=================
~/.config/gz-git/config.yaml (global)
└── ~/.config/gz-git/profiles/work.yaml (active profile)
    └── ~/mydevbox/.gz-git.yaml (project)
        └── ~/mydevbox/subproject/.gz-git.yaml (nested)
```

## 설정 우선순위

높은 우선순위 → 낮은 우선순위:

1. **Command flags** (`--provider gitlab`)
2. **Project config** (`.gz-git.yaml`)
3. **Active profile** (`~/.config/gz-git/profiles/work.yaml`)
4. **Global config** (`~/.config/gz-git/config.yaml`)
5. **Built-in defaults**

## Profile 파일 형식

```yaml
# ~/.config/gz-git/profiles/work.yaml
name: work
provider: gitlab
baseURL: https://gitlab.company.com
token: ${GITLAB_TOKEN}      # 환경변수 참조
cloneProto: ssh
sshPort: 2224
parallel: 10
includeSubgroups: true
subgroupMode: flat

# Command-specific overrides
sync:
  strategy: reset
  maxRetries: 3

branch:
  defaultBranch: develop
  protectedBranches: [main, master]
```

## Project Config 형식

```yaml
# .gz-git.yaml (프로젝트 루트)
profile: work               # 사용할 profile

# Profile 설정 override
sync:
  strategy: pull            # work profile의 reset을 pull로 override
  parallel: 5

metadata:
  team: backend
  repository: https://gitlab.company.com/backend/myproject
```

## 환경변수

Config 파일에서 `${VAR_NAME}` 문법으로 환경변수 참조:

```yaml
token: ${GITLAB_TOKEN}
baseURL: ${GITLAB_URL}
sshKeyPath: ${HOME}/.ssh/id_ed25519_work
```

## 보안

- Profile 파일: 0600 권한 (소유자만 읽기/쓰기)
- Config 디렉토리: 0700 권한
- Token은 환경변수로 관리 권장
- Shell 명령 실행 없음 (`${VAR}` 확장만)

## 사용 예제

### 멀티 환경 설정

```bash
# 1. Profile 생성
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token $WORK_TOKEN

gz-git config profile create personal \
  --provider github \
  --token $GITHUB_TOKEN

# 2. 환경 전환
gz-git config profile use work
gz-git forge from-forge --org backend    # work profile 사용

gz-git config profile use personal
gz-git forge from-forge --org my-projects  # personal profile 사용

# 3. 일회성 override
gz-git forge from-forge --profile work --org backend
```

### 프로젝트별 설정

```bash
# 프로젝트에 .gz-git.yaml 생성
cat > .gz-git.yaml << EOF
profile: work
sync:
  strategy: pull
  parallel: 3
EOF

# 이제 이 디렉토리에서는 work profile + 로컬 override 적용
gz-git workspace sync
```

### 설정 디버깅

```bash
# 현재 적용된 설정 확인
gz-git config show --effective

# 계층 구조 확인
gz-git config hierarchy

# 특정 값이 어디서 오는지 확인
gz-git config show --effective | grep provider
```
