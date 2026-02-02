# gz-git switch

여러 repository의 브랜치를 일괄 전환하는 명령어.

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위를 develop으로 전환
gz-git switch develop

# 특정 디렉토리
gz-git switch main ~/mydevbox

# 단일 repo
gz-git switch feature/new /path/to/single/repo
```

## 출력 예시

```
Switching 5 repositories to branch 'develop'...

✓ gzh-cli                  switched to develop            120ms
✓ gzh-cli-gitforge         switched to develop            95ms
⚠ gzh-cli-quality          dirty skipped                  40ms [dirty: 2 uncommitted]
✗ gzh-cli-template         error: branch not found        60ms
= gzh-cli-mcp              already on develop             25ms

Summary: 2 switched, 1 already, 1 skipped, 1 error
```

## 브랜치 생성

`--create` 옵션으로 브랜치가 없으면 자동 생성:

```bash
# 브랜치가 없으면 생성
gz-git switch feature/new --create

# 또는 단축형
gz-git switch feature/new -c
```

**동작**:
1. 브랜치 존재 여부 확인
2. 없으면 현재 HEAD에서 새 브랜치 생성
3. 생성된 브랜치로 전환

**주의**: 이미 존재하면 단순 전환 (오류 없음)

## ⚠️ Force Switch 경고

`--force` 옵션은 **매우 위험**합니다:

```bash
# ⚠️ 위험: 수정사항 모두 버림
gz-git switch main --force
```

### 위험성

| 동작 | 결과 |
|------|------|
| **Uncommitted changes** | **영구 손실** |
| **Untracked files** | **영구 삭제** |
| **Staged changes** | **영구 손실** |

### Force 사용 전 확인사항

```bash
# 1. 상태 확인
gz-git status ~/workspace

# 2. Dirty repos 확인
# ⚠ my-project (feature-x)  dirty  [2 uncommitted, 1 untracked]

# 3. 정말 버려도 되는지 확인
# - 중요한 작업이 아닌가?
# - 백업했는가?
# - Stash나 commit으로 보존 가능한가?

# 4. 확신하면 force 사용
gz-git switch main --force ~/workspace
```

### 대안 (안전)

**Force 대신 사용**:

```bash
# 1. Stash로 보존
cd my-project
git stash push -m "WIP: temporary work"
cd ..
gz-git switch main ~/workspace

# 2. Commit으로 보존
cd my-project
git commit -m "WIP: work in progress"
cd ..
gz-git switch main ~/workspace

# 3. 패턴으로 제외
gz-git switch main --exclude "my-project" ~/workspace
```

## Dirty Repo 처리

기본적으로 dirty repo는 **skip**됨:

```bash
gz-git switch develop ~/projects

# 출력:
# ⚠ my-api (feature-auth)  dirty skipped  [dirty: 2 uncommitted]
```

### 상태별 동작

| 상태 | 동작 | 아이콘 |
|------|------|--------|
| **Clean** | 브랜치 전환 | `✓` |
| **Dirty (수정됨)** | Skip (건너뜀) | `⚠` |
| **Already on branch** | Skip | `=` |
| **Branch not found** | Error | `✗` |
| **Force (위험)** | 변경사항 버림 | `✓` (데이터 손실) |

## 상태 값

| 값 | 의미 | 아이콘 |
|-----|------|--------|
| `StatusSwitched` | 브랜치 전환 성공 | `✓` |
| `StatusAlready` | 이미 해당 브랜치에 있음 | `=` |
| `StatusDirty` | Dirty repo (skip) | `⚠` |
| `StatusError` | 오류 (브랜치 없음 등) | `✗` |
| `StatusSkipped` | 필터로 제외됨 | `-` |

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-c, --create` | 브랜치 없으면 생성 | false |
| `--force` | **⚠️ 위험: 수정사항 버림** | false |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |
| `-f, --format` | 출력 형식 | default |
| `-n, --dry-run` | 미리보기 (실행 안 함) | false |

## 출력 형식

```bash
# 기본 형식
gz-git switch develop

# 간결한 형식 (오류만 표시)
gz-git switch develop --format compact

# JSON 형식
gz-git switch develop --format json

# LLM 친화적 형식
gz-git switch develop --format llm
```

## 필터링

```bash
# 특정 패턴만 포함
gz-git switch develop --include "backend-.*"

# 특정 패턴 제외
gz-git switch develop --exclude "vendor|tmp"

# 조합
gz-git switch feature/new --include "^service-" --exclude "test"
```

## 예제

### 기본 브랜치 전환

```bash
# 모든 repos를 develop으로
gz-git switch develop ~/workspace

# Main으로 전환
gz-git switch main ~/projects

# Feature 브랜치로 전환
gz-git switch feature/new-api ~/backend-repos
```

### 브랜치 생성하며 전환

```bash
# Feature 브랜치 생성 + 전환
gz-git switch feature/authentication --create ~/workspace

# 이미 존재하면 전환만 (오류 없음)
gz-git switch feature/existing --create ~/projects
```

### Dry-run으로 미리보기

```bash
# 실제 전환 전에 어떤 repos가 처리될지 확인
gz-git switch develop --dry-run ~/workspace

# 출력:
# ✓ gzh-cli          would switch to develop
# ⚠ my-api           dirty skipped
# ✗ old-project      error: branch not found
```

### 패턴 필터링으로 일부만 전환

```bash
# Backend repos만 develop으로
gz-git switch develop --include "backend-.*" ~/workspace

# Frontend는 제외
gz-git switch main --exclude "frontend-.*" ~/projects

# Service repos만, test 제외
gz-git switch feature/v2 \
  --include "^service-" \
  --exclude "test" \
  ~/microservices
```

### 위험한 force switch (주의사항 포함)

```bash
# ⚠️ 경고: 이 명령은 모든 수정사항을 버립니다

# 1. 먼저 상태 확인 (필수)
gz-git status ~/workspace

# 2. 정말 버려도 되는지 확인
# - 중요한 작업이 없는가?
# - 백업했는가?

# 3. 확신하면 force 사용
gz-git switch main --force ~/workspace

# 결과:
# ✓ gzh-cli          switched to main (CHANGES DISCARDED)
# ✓ my-api           switched to main (CHANGES DISCARDED)
```

**데이터 손실 위험**: Uncommitted changes 영구 손실

### 브랜치 없을 때 처리

```bash
# 브랜치 없으면 오류
gz-git switch feature/new ~/workspace

# 출력:
# ✗ gzh-cli          error: branch 'feature/new' not found

# 해결: --create 옵션 사용
gz-git switch feature/new --create ~/workspace

# 결과:
# ✓ gzh-cli          created and switched to feature/new
```

### Dirty repos 수동 처리

```bash
# 1. Switch 시도
gz-git switch develop ~/workspace

# 출력:
# ⚠ my-api (feature-auth)  dirty skipped  [dirty: 2 uncommitted]

# 2. 해당 repo로 이동
cd workspace/my-api

# 3. 옵션 선택:

# 옵션 A: Stash
git stash push -m "WIP: before switch"
git switch develop
# 나중에: git stash pop

# 옵션 B: Commit
git commit -m "WIP: work in progress"
git switch develop

# 옵션 C: 버리기 (확신하면)
git switch develop --force

# 4. 다시 bulk switch
cd ../..
gz-git switch develop ~/workspace
```

### CI/CD에서 사용

```bash
# Deployment 브랜치로 전환
gz-git switch release/v1.0 --format json ~/deployment > switch-result.json

# 결과 확인
if jq -e '.summary.error > 0' switch-result.json; then
    echo "Switch failed for some repos"
    cat switch-result.json | jq '.repositories[] | select(.status == "error")'
    exit 1
fi

# 성공 시 배포 계속
./deploy.sh
```

### 여러 단계 워크플로우

```bash
# 1. 상태 확인
gz-git status ~/workspace

# 2. Clean repos만 먼저 switch
gz-git switch develop ~/workspace

# 3. Dirty repos 확인
gz-git status ~/workspace | grep dirty

# 4. Dirty repos 수동 처리
# ... (위 예제 참고) ...

# 5. 다시 switch 시도
gz-git switch develop ~/workspace

# 6. 모두 전환 확인
gz-git status ~/workspace
```

## 주의사항

### Force 절대 금지 상황

**절대 사용 금지**:
- 중요한 작업 중
- 백업 없이
- 여러 repos에 일괄 적용
- CI/CD 자동화

**대신 사용**: Stash, Commit, 또는 패턴 제외

### Dirty Repository

**기본 동작**: Dirty repo는 **항상 skip**

```bash
# Clean repos만 전환됨
gz-git switch develop ~/workspace
```

**Force 옵션 위험성**:
```bash
# ⚠️ 위험: 모든 수정사항 영구 손실
gz-git switch develop --force ~/workspace
```

### 브랜치 생성 위치

`--create`는 **현재 HEAD**에서 브랜치 생성:

```bash
# 현재 main에 있으면
# feature/new는 main에서 생성됨
gz-git switch feature/new --create
```

**특정 시점에서 생성하려면**:
```bash
cd my-repo
git switch -c feature/new origin/develop
cd ..
```

### 오류 처리

**브랜치 없음**:
```bash
# 오류 발생
gz-git switch nonexistent ~/workspace

# 해결: --create 사용
gz-git switch nonexistent --create ~/workspace
```

**권한 오류**:
```bash
# 읽기 전용 repo
✗ readonly-project  error: permission denied
```

### Parallel 처리 주의

병렬 처리 시 출력 순서 보장 안 됨:

```bash
# 순서대로 출력 안 될 수 있음
gz-git switch develop -j 10 ~/workspace

# 순서 보장 필요시 -j 1
gz-git switch develop -j 1 ~/workspace
```

## 관련 명령어

- [`gz-git status`](status-command.md) - Switch 전 상태 확인
- [`gz-git fetch`](fetch-command.md) - Switch 전 최신 브랜치 정보
- [`gz-git commit`](commit-command.md) - Dirty repo 커밋 후 switch
