# gz-git status

여러 repository의 상태를 한눈에 확인하는 명령어.

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위 스캔
gz-git status

# 특정 디렉토리
gz-git status ~/mydevbox

# 단일 repo
gz-git status /path/to/single/repo
```

## 출력 예시

```
Checking 5 repositories...

✓ gzh-cli (master)              clean       up-to-date
⚠ gzh-cli-gitforge (develop)   3M 1?      2↓ behind
✗ gzh-cli-quality (main)        dirty       5↓ 2↑ diverged
⊘ gzh-cli-template (master)     -           fetch failed

Summary: 1 clean, 1 warning, 1 dirty, 1 unreachable
```

## 상태 아이콘

| 아이콘 | 상태 | 의미 |
|--------|------|------|
| `✓` | clean | Working tree가 깨끗함 |
| `⚠` | warning | Behind 또는 diverged (해결 가능) |
| `✗` | dirty/error | 수정된 파일 있음, 충돌 가능 |
| `⊘` | unreachable | Remote fetch 실패 |

## Working Tree 표시

| 표시 | 의미 |
|------|------|
| `M` | Modified (수정됨) |
| `A` | Added (추가됨) |
| `D` | Deleted (삭제됨) |
| `?` | Untracked (미추적) |
| `U` | Conflict (충돌) |

예: `3M 1?` = 수정 3개, 미추적 1개

## Divergence 표시

| 표시 | 의미 |
|------|------|
| `up-to-date` | Remote와 동일 |
| `N↓ behind` | Remote보다 N커밋 뒤처짐 |
| `N↑ ahead` | Remote보다 N커밋 앞섬 |
| `N↑ M↓ diverged` | 분기됨 (merge/rebase 필요) |

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |
| `-f, --format` | 출력 형식 | default |
| `-v, --verbose` | 상세 출력 | false |

## 출력 형식

```bash
# 기본 형식
gz-git status

# 간결한 형식
gz-git status --format compact

# JSON 형식
gz-git status --format json

# LLM 친화적 형식
gz-git status --format llm
```

## 스캔 깊이

```bash
# 현재 디렉토리만 (단일 repo처럼)
gz-git status -d 0

# 현재 + 1레벨 (기본값)
gz-git status -d 1

# 현재 + 2레벨 (org/repo 구조)
gz-git status -d 2
```

## 필터링

```bash
# 특정 패턴만 포함
gz-git status --include "gzh-cli-.*"

# 특정 패턴 제외
gz-git status --exclude "vendor|tmp"

# 조합
gz-git status --include "^agent-" --exclude "test"
```

## 예제

### CI/CD에서 상태 확인

```bash
# JSON으로 출력하여 파싱
status=$(gz-git status --format json)

# dirty repo가 있으면 실패
if echo "$status" | jq -e '.[] | select(.dirty == true)' > /dev/null; then
    echo "Uncommitted changes detected!"
    exit 1
fi
```

### 정기 모니터링

```bash
# cron job으로 상태 체크
gz-git status ~/projects --format llm | mail -s "Git Status Report" admin@example.com
```
