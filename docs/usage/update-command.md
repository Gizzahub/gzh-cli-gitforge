# gz-git update

안전한 업데이트 명령어 (`pull --rebase`의 간편 버전).

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위 스캔
gz-git update

# 특정 디렉토리
gz-git update ~/mydevbox

# 단일 repo
gz-git update /path/to/single/repo
```

## 출력 예시

```
Updating 5 repositories...

✓ gzh-cli (master)                 rebased 3↓                   640ms
✓ gzh-cli-gitforge (develop)       up-to-date                   120ms
⚠ gzh-cli-quality (main)            dirty skipped                45ms [dirty: 1 uncommitted]
= gzh-cli-template (master)         rebased 1↓                   890ms
✓ gzh-cli-mcp (main)                up-to-date                   105ms

Summary: 3 success, 2 up-to-date, 1 skipped
```

## Update vs Pull

| 명령어 | 동작 | 전략 | 히스토리 | 사용 시점 |
|--------|------|------|----------|-----------|
| **update** | pull --rebase | rebase (고정) | 선형 유지 | **일상적인 업데이트** |
| **pull** | fetch + integrate | merge/rebase/ff-only (선택) | 전략별 상이 | 전략 선택이 필요할 때 |

### 언제 update를 사용하나?

**✓ Update를 사용해야 할 때**:
- 일일 코드 동기화
- Feature 브랜치 최신화
- 로컬 작업만 있고 아직 push 안 함
- 깔끔한 히스토리 선호

**✗ Update를 피해야 할 때**:
- 이미 push한 커밋 (협업 중)
- Main/master 브랜치 (merge가 안전)
- Conflict 해결이 복잡할 것 같을 때

**대신 사용**: `gz-git pull -s merge` (안전)

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `--no-fetch` | Fetch 생략 (이미 fetch한 경우) | false |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |
| `-f, --format` | 출력 형식 | default |
| `-n, --dry-run` | 미리보기 (실행 안 함) | false |
| `-v, --verbose` | 상세 출력 | false |

## 출력 형식

```bash
# 기본 형식
gz-git update

# 간결한 형식
gz-git update --format compact

# JSON 형식
gz-git update --format json

# LLM 친화적 형식
gz-git update --format llm
```

## 필터링

```bash
# 특정 패턴만 포함
gz-git update --include "feature-.*"

# 특정 패턴 제외
gz-git update --exclude "vendor|tmp"

# 조합
gz-git update --include "^service-" --exclude "test"
```

## 예제

### 일일 업데이트 - 기본 사용

```bash
# 매일 아침 작업 시작 전
gz-git update ~/workspace

# 깔끔한 선형 히스토리 유지
# Feature 브랜치에 적합
```

### Fetch 후 update - fetch와 분리

```bash
# 1. 먼저 모든 repos에서 fetch
gz-git fetch ~/projects

# 2. 변경사항 확인
gz-git status ~/projects

# 3. Fetch 결과로 update (fetch 생략)
gz-git update --no-fetch ~/projects
```

**장점**: Fetch 한 번 → 여러 작업에 재사용

### Watch 모드 - 주기적 자동 업데이트

```bash
# 30초마다 자동 update
gz-git update --watch --interval 30s ~/workspace

# 개발 서버 실행 중에 자동 동기화
# Ctrl+C로 종료
```

**주의**: Dirty repo는 자동 skip됨

### CI/CD 파이프라인에서 사용

```bash
# Deployment 전 최신 코드 확인
gz-git update --format json ~/production > update-result.json

# Update 실패 시 배포 중단
if [ $? -ne 0 ]; then
    echo "Update failed: cannot deploy"
    cat update-result.json
    exit 1
fi

# 성공 시 배포 계속
./deploy.sh
```

### 패턴 필터링으로 선택적 update

```bash
# Feature 브랜치만 update
gz-git update --include "feature-.*" ~/workspace

# Main/master 제외 (안전)
gz-git update --exclude "^(main|master)$" ~/projects
```

### Dry-run으로 미리보기

```bash
# 실제 update 전에 어떤 repo가 처리될지 확인
gz-git update --dry-run ~/workspace

# 출력에 "would-update" 상태로 표시됨
```

## 주의사항

### Rebase 위험성

Update는 **항상 rebase**를 사용:

```bash
# ✗ 위험: Public 커밋을 rebase
gz-git update ~/projects  # main/master에서 실행 금지

# ✓ 안전: 로컬 feature 브랜치만 update
gz-git update --include "feature-.*" ~/workspace
```

**금지 사항**:
- 이미 push한 커밋 rebase
- Main/master 브랜치 update
- 다른 사람과 공유 중인 브랜치

**대신 사용**: `gz-git pull -s merge`

### Dirty Repository

기본적으로 dirty repo는 **skip**됨:

```bash
gz-git update ~/projects
# ⚠ my-project (feature-x)  dirty skipped  [dirty: 2 uncommitted]
```

**해결 방법**:
1. 수동으로 commit
2. `git stash` 사용
3. `gz-git pull --stash` 사용 (update는 stash 옵션 없음)

### Conflict 해결

Rebase conflict 발생 시:

```bash
gz-git update ~/projects

# 출력:
# ✗ my-api (feature-auth)  conflict  780ms
#    Rebase failed; resolve conflicts

# 해결 방법:
cd my-api

# 1. Conflict 파일 수정
git status  # 충돌 파일 확인
# ... 편집기에서 해결 ...

# 2. Stage
git add <resolved-files>

# 3. Continue
git rebase --continue

# 4. Abort (취소하려면)
git rebase --abort
```

### Fetch vs Update

**Fetch만 실행** (안전):
```bash
gz-git fetch ~/workspace  # Working tree 변경 없음
gz-git status ~/workspace  # 상태 확인
```

**Update 실행** (변경됨):
```bash
gz-git update ~/workspace  # Working tree 변경됨 (rebase)
```

**권장 워크플로우**:
1. `fetch` - 원격 변경사항 가져오기
2. `status` - Divergence 확인
3. `update` 또는 `pull` - 선택적 적용

## 관련 명령어

- [`gz-git pull`](pull-command.md) - Merge 전략 선택 가능 (merge/rebase/ff-only)
- [`gz-git fetch`](fetch-command.md) - Fetch만 실행 (안전)
- [`gz-git status`](status-command.md) - Update 전 상태 확인
