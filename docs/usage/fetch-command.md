# gz-git fetch

원격 저장소에서 변경사항을 가져오는 명령어 (working tree 변경 없음).

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위 스캔
gz-git fetch

# 특정 디렉토리
gz-git fetch ~/mydevbox

# 단일 repo
gz-git fetch /path/to/single/repo
```

## 출력 예시

```
Fetching 5 repositories...

✓ gzh-cli (master)                 up-to-date                    120ms
✓ gzh-cli-gitforge (develop)       3↓ fetched                   340ms
= gzh-cli-quality (main)            up-to-date                    95ms
⚠ gzh-cli-template (master)         up-to-date                   110ms [dirty: 2 uncommitted, 1 untracked]
✗ gzh-cli-mcp (main)                failed                       560ms

Summary: 3 success, 1 up-to-date, 1 error

🔐 Authentication required for 1 repository(ies):
   • gzh-cli-mcp

💡 To fix: Configure credential helper or switch to SSH
   git config --global credential.helper cache
```

## Remote 옵션

| 모드            | 플래그                   | 설명                  | 사용 시점           |
| --------------- | ------------------------ | --------------------- | ------------------- |
| **All Remotes** | `--all-remotes` (기본값) | 모든 remote에서 fetch | 다중 remote 사용 시 |
| **Origin Only** | `--all-remotes=false`    | origin에서만 fetch    | CI/CD, 단일 remote  |

```bash
# 모든 remote에서 fetch (기본값)
gz-git fetch

# Origin만 fetch (CI/CD에 적합)
gz-git fetch --all-remotes=false ~/workspace
```

## 상태 아이콘

| 아이콘 | 상태          | 의미                          |
| ------ | ------------- | ----------------------------- |
| `✓`    | fetched       | 새 커밋을 가져옴              |
| `=`    | up-to-date    | 이미 최신 상태                |
| `⚠`    | dirty         | 수정된 파일 있음 (fetch 성공) |
| `✗`    | error         | Fetch 실패                    |
| `🔐`   | auth-required | 인증 필요                     |

## Divergence 표시

| 표시            | 의미                       |
| --------------- | -------------------------- |
| `N↓ fetched`    | Remote에서 N개 커밋 가져옴 |
| `up-to-date`    | Remote와 동일              |
| `up-to-date N↑` | Local이 N커밋 앞섬         |
| `N↓ N↑ fetched` | 분기됨 (fetch는 성공)      |

## 주요 옵션

| 옵션               | 설명                    | 기본값  |
| ------------------ | ----------------------- | ------- |
| `-d, --scan-depth` | 스캔 깊이               | 1       |
| `-j, --parallel`   | 병렬 처리 수            | 10      |
| `--all-remotes`    | 모든 remote에서 fetch   | true    |
| `-p, --prune`      | 삭제된 원격 브랜치 정리 | false   |
| `-t, --tags`       | 모든 태그 가져오기      | false   |
| `--include`        | 포함 패턴 (regex)       | -       |
| `--exclude`        | 제외 패턴 (regex)       | -       |
| `-f, --format`     | 출력 형식               | default |
| `-n, --dry-run`    | 미리보기 (실행 안 함)   | false   |

## 출력 형식

```bash
# 기본 형식 (상세)
gz-git fetch

# 간결한 형식 (오류만 표시)
gz-git fetch --format compact

# JSON 형식
gz-git fetch --format json

# LLM 친화적 형식
gz-git fetch --format llm
```

## 필터링

```bash
# 특정 패턴만 포함
gz-git fetch --include "gzh-cli-.*"

# 특정 패턴 제외
gz-git fetch --exclude "vendor|tmp"

# 조합
gz-git fetch --include "^agent-" --exclude "test"
```

## 예제

### 일상 업데이트 - 모든 repos fetch

```bash
# 개발 환경에서 아침 첫 작업
gz-git fetch ~/mydevbox

# 모든 remote에서 fetch (upstream, origin 등)
gz-git fetch ~/projects --all-remotes
```

### Origin만 fetch - CI/CD 환경

```bash
# CI/CD에서는 origin만 필요
gz-git fetch --all-remotes=false ~/workspace

# JSON으로 출력하여 파싱
gz-git fetch --all-remotes=false --format json | jq '.repositories[] | select(.status == "error")'
```

### Prune과 함께 - 정리하면서 fetch

```bash
# 삭제된 원격 브랜치 정리
gz-git fetch --prune ~/projects

# Tags도 함께 가져오기
gz-git fetch --prune --tags ~/repos
```

### 인증 오류 처리

```bash
# Fetch 실행 후 인증 오류 확인
gz-git fetch ~/workspace

# 출력에서 🔐 아이콘과 인증 가이드 확인:
# 💡 To fix: Configure credential helper or switch to SSH
#    git config --global credential.helper cache

# SSH로 전환하거나 credential helper 설정
git config --global credential.helper cache
```

### 패턴 필터링으로 선택적 fetch

```bash
# 특정 조직의 repo만
gz-git fetch --include "myorg-.*" ~/workspace

# 테스트/실험 repo 제외
gz-git fetch --exclude "test|experiment|tmp" ~/projects

# 특정 깊이로 제한
gz-git fetch -d 2 --include "backend-" ~/monorepo
```

### Dry-run으로 미리보기

```bash
# 실제 fetch 전에 어떤 repo가 처리될지 확인
gz-git fetch --dry-run ~/workspace

# 출력에 "would-fetch" 상태로 표시됨
```

### Config profile 사용

```bash
# Work 프로필 적용 (fetch 설정 포함)
gz-git config profile use work
gz-git fetch ~/work-projects

# Profile에서 all-remotes, prune 설정 자동 적용
```

## 주의사항

### Fetch vs Pull

| 명령어    | 동작                   | Working Tree | 사용 시점            |
| --------- | ---------------------- | ------------ | -------------------- |
| **fetch** | 원격 변경사항만 가져옴 | 변경 없음    | 안전한 업데이트 확인 |
| **pull**  | fetch + merge/rebase   | 변경됨       | 로컬에 적용          |

**권장**: 먼저 `fetch`로 변경사항 확인 → 필요시 `pull` 또는 `update`

### Dirty Repository

Fetch는 working tree를 변경하지 않으므로 dirty repo에서도 안전하게 실행 가능:

```bash
# 수정사항이 있어도 fetch는 안전
gz-git fetch

# ⚠ 아이콘으로 dirty 상태 표시됨
# [dirty: 2 uncommitted, 1 untracked]
```

### 인증 관련

**HTTPS 사용 시** credential helper 필요:

```bash
# Cache 방식 (기본 15분)
git config --global credential.helper cache

# Store 방식 (영구 저장, 주의)
git config --global credential.helper store
```

**권장**: SSH 키 사용 (인증 문제 없음)

```bash
# HTTPS → SSH로 변경
git remote set-url origin git@github.com:user/repo.git
```

## 관련 명령어

- [`gz-git pull`](pull-command.md) - Fetch + integrate (merge/rebase)
- [`gz-git update`](update-command.md) - Fetch + pull --rebase (안전한 업데이트)
- [`gz-git status`](status-command.md) - 전체 상태 확인
