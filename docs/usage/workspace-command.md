# gz-git workspace

로컬 config 파일 기반 workspace 관리.

## 서브커맨드

| 커맨드 | 설명 |
|--------|------|
| `init` | 디렉토리 스캔 → config 생성 |
| `sync` | Config 기반 clone/update |
| `status` | Workspace health check |
| `add` | Config에 repo 추가 |
| `validate` | Config 파일 검증 |
| `generate-config` | Forge API → config 생성 |

## init

디렉토리를 스캔하여 config 파일 생성.

```bash
# 현재 디렉토리 스캔 (기본 kind: workspace)
gz-git workspace init .

# 특정 디렉토리, 깊이 지정
gz-git workspace init ~/mydevbox -d 3

# Config kind 선택 (workspace 또는 repositories)
gz-git workspace init . --kind repositories

# Sync strategy 선택 (reset, pull, fetch, skip)
gz-git workspace init . --strategy pull

# 제외 패턴
gz-git workspace init . --exclude "vendor,tmp,node_modules"

# 빈 템플릿만 생성 (스캔 없이)
gz-git workspace init . --template

# 출력 파일 지정
gz-git workspace init . -o myworkspace.yaml
```

### Kind 옵션

| Kind | 설명 | 형식 |
|------|------|------|
| `workspace` | 계층적 구조 (기본값) | `workspaces:` map |
| `repositories` | 단순 목록 | `repositories:` array |

**주의**: `workspaces` (복수형)도 지원되지만 deprecated 경고가 표시됩니다.

### Strategy 옵션

| Strategy | 설명 |
|----------|------|
| `reset` | git fetch + reset --hard (기본값) |
| `pull` | git pull |
| `fetch` | git fetch만 |
| `skip` | 기존 repo 건너뛰기 |

### 생성되는 config

**workspace (기본값)**:

```yaml
# .gz-git.yaml
version: 1
kind: workspace

metadata:
  name: mydevbox

strategy: reset
parallel: 10

workspaces:
  project1:
    path: project1
    type: git
    url: git@github.com:user/project1.git

  project2:
    path: project2
    type: git
    url: git@github.com:user/project2.git
```

**repositories**:

```yaml
# .gz-git.yaml (--kind repositories)
version: 1
kind: repositories

metadata:
  name: mydevbox

strategy: reset
parallel: 10

repositories:
  - name: project1
    url: git@github.com:user/project1.git
    path: project1

  - name: project2
    url: git@github.com:user/project2.git
    path: project2
```

## sync

Config 파일에 정의된 repo들을 clone/update.

```bash
# 기본 (.gz-git.yaml 사용)
gz-git workspace sync

# 특정 config 파일
gz-git workspace sync -c myworkspace.yaml

# Dry-run
gz-git workspace sync --dry-run

# Strategy 지정
gz-git workspace sync --strategy reset
```

### Strategy

| Strategy | 동작 |
|----------|------|
| `pull` | git pull (기본값) |
| `reset` | git fetch + reset --hard |
| `skip` | 기존 repo 건너뛰기 |
| `rebase` | git pull --rebase |

## status

Workspace health check.

```bash
# 기본
gz-git workspace status

# 상세 출력
gz-git workspace status --verbose

# 특정 config
gz-git workspace status -c myworkspace.yaml
```

### 출력 예시

```
Workspace: mydevbox (7 repositories)
Config: .gz-git.yaml

✓ project1 (master)     clean    up-to-date
✓ project2 (develop)    clean    up-to-date
⚠ project3 (main)       2M       3↓ behind
✗ project4 (feature)    dirty    diverged

Summary: 2 clean, 1 behind, 1 dirty
```

## add

Config에 새 repo 추가.

```bash
# URL로 추가
gz-git workspace add https://github.com/user/newrepo.git

# 현재 디렉토리의 repo 추가
cd newrepo && gz-git workspace add --from-current

# Branch 지정
gz-git workspace add https://github.com/user/repo.git --branch develop

# 특정 config에 추가
gz-git workspace add https://github.com/user/repo.git -c myworkspace.yaml
```

## validate

Config 파일 종합 검증. 오류, 경고, 권장사항을 분류하여 표시.

```bash
# 기본 (.gz-git.yaml 자동 탐지)
gz-git workspace validate

# 특정 파일
gz-git workspace validate -c myworkspace.yaml

# 상세 출력 (모든 권장사항 포함)
gz-git workspace validate --verbose
```

### 검증 항목

**Errors (필수 수정)**:
- `kind` 필드 누락
- 잘못된 `kind` 값
- 잘못된 `strategy` 값
- Repository/Workspace 필수 필드 누락 (url)
- 중복 name 검사

**Warnings (권장 수정)**:
- Deprecated kind 사용 (`repository` → `repositories`, `workspaces` → `workspace`)
- kind와 실제 구조 불일치 (`kind: workspace`인데 `repositories:` 사용)

**Suggestions (개선 권장)**:
- `version` 필드 추가 권장
- `strategy` 필드 추가 권장

### 출력 예시

**오류가 있는 경우**:

```
Errors:
  ✗ missing 'kind' field: must be 'workspace' or 'repositories'

Suggestions:
  → Add 'kind: workspace' for hierarchical config or 'kind: repositories' for flat list
  → Add 'strategy: reset' (or pull, fetch, skip) to specify sync behavior

Found: 1 error(s)
Error: validation failed with 1 error(s)
```

**경고만 있는 경우**:

```
Warnings:
  ⚠ 'kind: repository' is deprecated, use 'kind: repositories' (plural)

Found: 1 warning(s)

✓ Configuration is valid: myconfig.yaml
```

**정상인 경우**:

```
No issues found.

✓ Configuration is valid: myconfig.yaml
```

### Kind 값

| 값 | 상태 | 설명 |
|---|---|---|
| `workspace` | 권장 | 계층적 구조 (`workspaces:` map) |
| `repositories` | 권장 | 단순 목록 (`repositories:` array) |
| `workspaces` | deprecated | `workspace` 사용 권장 |
| `repository` | deprecated | `repositories` 사용 권장 |

### Strategy 값

| 값 | 설명 |
|---|---|
| `reset` | git fetch + reset --hard |
| `pull` | git pull |
| `fetch` | git fetch만 |
| `skip` | 기존 repo 건너뛰기 |

## generate-config

Forge API에서 config 생성 (`sync config generate`와 동일).

```bash
gz-git workspace generate-config \
  --provider gitlab \
  --org mygroup \
  -o .gz-git.yaml
```

## Config 형식

### 기본 형식

```yaml
version: 1
kind: repositories

metadata:
  name: myworkspace
  description: My development workspace
  team: backend

strategy: pull
parallel: 10

repositories:
  - name: repo1
    url: git@github.com:org/repo1.git
    branch: master

  - name: repo2
    url: git@github.com:org/repo2.git
    branch: develop
    assumePresent: true  # 이미 clone됨으로 간주
```

### Repository 필드

| 필드 | 설명 | 필수 |
|------|------|------|
| `name` | 디렉토리 이름 | URL에서 추출 |
| `url` | Git URL | Yes |
| `branch` | Checkout branch | No |
| `assumePresent` | Clone 스킵 | No |
| `path` | 상대 경로 | No |

## 워크플로우 예제

### 새 워크스페이스 설정

```bash
# 1. 기존 repos 스캔
gz-git workspace init ~/mydevbox

# 2. 수동으로 config 편집 (필요시)
vim .gz-git.yaml

# 3. 검증
gz-git workspace validate

# 4. 동기화
gz-git workspace sync
```

### 팀 공유

```bash
# 1. Config 생성
gz-git workspace init . -o team-workspace.yaml

# 2. Git에 커밋
git add team-workspace.yaml
git commit -m "Add workspace config"

# 3. 팀원이 clone 후
gz-git workspace sync -c team-workspace.yaml
```

### Forge와 연동

```bash
# GitLab org에서 config 생성
gz-git workspace generate-config \
  --provider gitlab \
  --org myteam \
  --include-subgroups \
  -o .gz-git.yaml

# 정기 동기화
gz-git workspace sync
```

## workspace vs sync 차이

| 명령 | 용도 | 데이터 소스 |
|------|------|------------|
| `workspace` | 로컬 config 관리 | `.gz-git.yaml` 파일 |
| `sync` | Forge API 직접 호출 | GitHub/GitLab/Gitea API |

일반적인 워크플로우:
1. `sync config generate` → config 생성
2. `workspace sync` → config 기반 동기화
