# gz-git pull

원격 저장소에서 변경사항을 가져와서 로컬 브랜치에 통합하는 명령어 (fetch + integrate).

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위 스캔
gz-git pull

# 특정 디렉토리
gz-git pull ~/mydevbox

# 단일 repo
gz-git pull /path/to/single/repo
```

## 출력 예시

```
Pulling 5 repositories...

✓ gzh-cli (master)                 merged 3↓                    540ms
✓ gzh-cli-gitforge (develop)       up-to-date                   120ms
⚠ gzh-cli-quality (main)            dirty skipped                50ms [dirty: 2 uncommitted, 1 untracked]
✗ gzh-cli-template (master)         conflict                     340ms
= gzh-cli-mcp (main)                rebased 2↓                   890ms

Summary: 2 success, 1 up-to-date, 1 conflict, 1 skipped

⚠ Warning: 1 repository(ies) have uncommitted changes
✗ Conflict detected in 1 repository(ies):
   • gzh-cli-template (resolve manually)
```

## Merge 전략

| 전략 | 플래그 | 동작 | 히스토리 | 사용 시점 |
|------|--------|------|----------|-----------|
| **merge** | `-s merge` (기본값) | Merge commit 생성 | 분기 보존 | 일반적인 협업 |
| **rebase** | `-s rebase` | 커밋 재배치 | 선형 유지 | 깔끔한 히스토리 선호 |
| **ff-only** | `-s ff-only` | Fast-forward만 허용 | 선형 유지 | 안전한 업데이트 |

### 전략별 상세 설명

#### merge (기본값)

```bash
gz-git pull -s merge ~/projects
# 또는
gz-git pull  # merge가 기본값
```

**동작**:
- Remote 변경사항을 merge commit으로 통합
- 브랜치 히스토리 그대로 보존
- Conflict 발생 시 수동 해결 필요

**장점**: 협업 시 누가 어떤 브랜치에서 작업했는지 명확
**단점**: Merge commit이 많으면 히스토리 복잡

#### rebase

```bash
gz-git pull -s rebase ~/projects
```

**동작**:
- 로컬 커밋을 remote 위로 재배치
- 선형 히스토리 유지
- Conflict 발생 시 각 커밋마다 해결

**장점**: 깔끔한 선형 히스토리
**단점**: Conflict 해결이 복잡할 수 있음

**주의**: 이미 push한 커밋은 rebase 금지 (협업 규칙)

#### ff-only (Fast-forward only)

```bash
gz-git pull -s ff-only ~/repos
```

**동작**:
- Fast-forward 가능한 경우에만 pull
- Diverged 상태면 실패 (안전)
- Merge commit 없음

**장점**: 가장 안전 (히스토리 손상 없음)
**단점**: Diverged 상태에서는 실패

**사용**: CI/CD, 프로덕션 배포 전

## Stash 동작

`--stash` 옵션으로 dirty repo를 자동 처리:

```bash
gz-git pull --stash ~/projects
```

**동작 순서**:
1. Dirty repo 감지
2. `git stash push` 실행
3. Pull 수행
4. `git stash pop` 실행 (자동 복원)

**주의**: Stash pop 시 conflict 가능 → 수동 해결 필요

### Dirty Repo 처리 비교

| 옵션 | Dirty Repo 동작 | 안전성 |
|------|----------------|--------|
| **기본 (--stash 없음)** | Skip (건너뜀) | 안전 |
| **--stash** | Stash → Pull → Pop | 주의 필요 |

## 충돌 해결

### Conflict 발생 시

```bash
gz-git pull ~/projects

# 출력:
# ✗ my-project (main)  conflict  450ms
#    Auto-merge failed; fix conflicts manually
```

### 해결 방법

```bash
# 1. 해당 repo로 이동
cd my-project

# 2. Conflict 파일 확인
git status

# 3. 수동으로 conflict 해결 (편집기 사용)
# <<<<<<< HEAD
# ...
# =======
# ...
# >>>>>>> remote/branch

# 4. 해결 후 stage
git add <resolved-files>

# 5. Merge 완료
git commit  # merge의 경우
git rebase --continue  # rebase의 경우
```

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-s, --merge-strategy` | Merge 전략 (merge/rebase/ff-only) | merge |
| `--stash` | Dirty repo 자동 stash | false |
| `-p, --prune` | 삭제된 원격 브랜치 정리 | false |
| `-t, --tags` | 모든 태그 가져오기 | false |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |
| `-f, --format` | 출력 형식 | default |
| `-n, --dry-run` | 미리보기 (실행 안 함) | false |

## 출력 형식

```bash
# 기본 형식
gz-git pull

# 간결한 형식 (오류만 표시)
gz-git pull --format compact

# JSON 형식
gz-git pull --format json

# LLM 친화적 형식
gz-git pull --format llm
```

## 필터링

```bash
# 특정 패턴만 포함
gz-git pull --include "backend-.*"

# 특정 패턴 제외
gz-git pull --exclude "vendor|tmp"

# 조합
gz-git pull --include "^service-" --exclude "test"
```

## 예제

### 기본 pull (merge 전략)

```bash
# 일반적인 개발 워크플로우
gz-git pull ~/workspace

# 협업 프로젝트 (merge commit 허용)
gz-git pull -s merge ~/team-projects
```

### Rebase로 pull - 깔끔한 히스토리

```bash
# 개인 프로젝트 또는 feature 브랜치
gz-git pull -s rebase ~/my-projects

# 아직 push하지 않은 로컬 작업만 rebase
gz-git pull -s rebase --include "feature-.*" ~/workspace
```

### Fast-forward만 허용 - 안전한 업데이트

```bash
# CI/CD 환경 (안전성 최우선)
gz-git pull -s ff-only ~/production

# Diverged 상태면 실패하여 안전
# 실패 시 수동 확인 필요
```

### Dirty repo stash 후 pull

```bash
# 작업 중인 변경사항 자동 보존
gz-git pull --stash ~/projects

# 주의: Stash pop 시 conflict 가능
# Conflict 발생 시:
# 1. cd <repo>
# 2. git status
# 3. 수동 해결
# 4. git add <files>
# 5. (stash conflict이므로 commit 불필요)
```

### Config profile 사용

```bash
# Work 프로필 - rebase 전략 자동 적용
gz-git config profile use work
gz-git pull ~/work-projects

# Profile 설정 예시 (~/.config/gz-git/profiles/work.yaml):
# pull:
#   rebase: true
#   prune: true
```

### 충돌 해결 워크플로우

```bash
# 1. Pull 실행 (conflict 발생)
gz-git pull ~/projects

# 출력:
# ✗ my-api (develop)  conflict  560ms
#    Auto-merge failed; fix conflicts manually

# 2. 해당 repo로 이동
cd my-api

# 3. Conflict 확인
git status
# Unmerged paths:
#   both modified:   src/api.go

# 4. 편집기에서 conflict 마커 해결
# <<<<<<< HEAD
# 로컬 변경
# =======
# 원격 변경
# >>>>>>> origin/develop

# 5. 해결 완료 후
git add src/api.go

# 6. Merge 완료
git commit  # 에디터에서 merge 메시지 확인/수정

# 7. 다른 repos 계속 처리
cd ..
gz-git pull --exclude "my-api" ~/projects
```

### Prune과 함께 pull

```bash
# 삭제된 원격 브랜치도 정리
gz-git pull --prune ~/projects

# Tags도 함께 가져오기
gz-git pull --prune --tags ~/repos
```

### Dry-run으로 미리보기

```bash
# 실제 pull 전에 어떤 repo가 처리될지 확인
gz-git pull --dry-run ~/workspace

# 출력에 "would-pull" 상태로 표시됨
```

## 주의사항

### Pull vs Fetch vs Update

| 명령어 | 동작 | Working Tree | 안전성 |
|--------|------|--------------|--------|
| **fetch** | 원격 변경사항만 가져옴 | 변경 없음 | 매우 안전 |
| **pull** | fetch + merge/rebase | 변경됨 | 주의 필요 |
| **update** | pull --rebase (간편) | 변경됨 | 주의 필요 |

**권장 워크플로우**:
1. `fetch` - 변경사항 확인
2. `status` - 상태 체크
3. `pull` 또는 `update` - 적용

### Dirty Repository

기본적으로 dirty repo는 **skip**됨:

```bash
gz-git pull ~/projects
# ⚠ my-project (main)  dirty skipped  [dirty: 2 uncommitted]
```

**해결 방법**:
1. `--stash` 옵션 사용 (자동 처리)
2. 수동으로 commit 또는 stash 후 pull

### Rebase 주의사항

**절대 금지**: 이미 push한 커밋은 rebase하지 말 것

```bash
# ✗ 위험: Public 브랜치를 rebase
gz-git pull -s rebase ~/projects  # main/master에서 실행 금지

# ✓ 안전: 로컬 feature 브랜치만 rebase
gz-git pull -s rebase --include "feature-.*" ~/workspace
```

**이유**: Rebase는 커밋 히스토리를 재작성 → 협업자와 충돌

### Conflict 해결 시

**Merge 충돌**:
- 한 번만 해결
- `git commit`으로 완료

**Rebase 충돌**:
- 각 커밋마다 해결 가능
- `git rebase --continue` 반복

**Abort 옵션**:
```bash
# Merge 취소
git merge --abort

# Rebase 취소
git rebase --abort
```

### CI/CD에서 사용

```bash
# Fast-forward만 허용 (안전)
gz-git pull -s ff-only --format json ~/deployment

# Diverged 상태면 실패 → 알림
if [ $? -ne 0 ]; then
    echo "Pull failed: manual merge required"
    exit 1
fi
```

## 관련 명령어

- [`gz-git fetch`](fetch-command.md) - 변경사항만 가져오기 (안전)
- [`gz-git update`](update-command.md) - Pull --rebase 간편 명령어
- [`gz-git status`](status-command.md) - Pull 전 상태 확인
- [`gz-git push`](push-command.md) - Pull 후 push
