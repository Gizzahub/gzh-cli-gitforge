# gz-git commit

여러 repository에 일괄 커밋하는 명령어.

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위 스캔
gz-git commit

# 특정 디렉토리
gz-git commit ~/mydevbox

# 단일 repo
gz-git commit /path/to/single/repo
```

## 출력 예시 (Preview 모드)

```
Scanning 5 repositories for uncommitted changes...

Found 3 dirty repositories:

  gzh-cli-gitforge (develop)      2 modified, 1 untracked
  gzh-cli-quality (main)           1 modified
  gzh-cli-template (master)        3 modified, 2 untracked

Preview mode: Use --yes to commit, or -e to edit messages
```

## Preview 모드 (기본 동작)

**중요**: `--yes` 플래그 없이 실행하면 **미리보기만** 수행:

```bash
# 1. Dirty repos 스캔만 (커밋 안 함)
gz-git commit

# 2. 메시지 작성 후 미리보기
gz-git commit -m "repo1:fix: bug" -m "repo2:feat: feature"

# 3. 실제 커밋하려면 --yes 추가
gz-git commit -m "repo1:fix: bug" -m "repo2:feat: feature" --yes
```

**장점**: 실수로 잘못된 메시지 커밋 방지

## 메시지 형식

3가지 메시지 제공 방법:

| 방법 | 플래그 | 형식 | 사용 시점 |
|------|--------|------|-----------|
| **Per-repo** | `-m "repo:message"` | 각 repo마다 다른 메시지 | **가장 일반적** |
| **공통** | `--all "message"` | 모든 repo에 동일 메시지 | 단순 업데이트 |
| **파일** | `--file messages.json` | JSON 파일에서 읽기 | 자동화/스크립트 |

### 1. Per-repo 메시지 (권장)

각 repository마다 다른 커밋 메시지:

```bash
gz-git commit \
  -m "gzh-cli:feat: add new command" \
  -m "gzh-cli-gitforge:fix: fix clone bug" \
  -m "gzh-cli-quality:docs: update README" \
  --yes
```

**형식**: `-m "repository-name:commit-message"`

**장점**: 각 repo의 변경사항에 맞는 정확한 메시지

### 2. 공통 메시지

모든 repository에 동일한 메시지:

```bash
gz-git commit --all "chore: update dependencies" --yes
```

**사용 예**:
- 의존성 일괄 업데이트
- 설정 파일 동기화
- 문서 일괄 수정

### 3. JSON 파일

자동화에 적합:

```bash
gz-git commit --file messages.json --yes
```

**messages.json 형식**:
```json
{
  "gzh-cli": "feat: add wizard command",
  "gzh-cli-gitforge": "fix: fix authentication error",
  "gzh-cli-quality": "test: add integration tests"
}
```

**장점**: 스크립트/CI/CD에서 사용

## Interactive 모드

에디터에서 메시지 편집:

```bash
gz-git commit -e
```

**동작**:
1. Dirty repos 스캔
2. 각 repo마다 임시 메시지 생성
3. `$EDITOR` 실행 (vim, nano 등)
4. 편집 완료 후 확인
5. `--yes` 없으면 미리보기만

**편집기 내용 예**:
```
# Edit commit messages below
# Format: repository:message

gzh-cli:feat: add new feature
gzh-cli-gitforge:fix: fix clone issue
gzh-cli-quality:docs: update documentation
```

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-m, --message` | Per-repo 메시지 (`repo:msg`) | - |
| `--all` | 공통 메시지 (모든 repo) | - |
| `-y, --yes` | 확인 없이 실행 | false |
| `-e, --edit` | 에디터에서 메시지 편집 | false |
| `--file` | JSON 파일에서 메시지 읽기 | - |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |
| `-f, --format` | 출력 형식 | default |
| `-n, --dry-run` | 미리보기 (실행 안 함) | false |

**중요**: `--yes` 없으면 미리보기만 수행 (실제 커밋 안 함)

## 출력 형식

```bash
# 기본 형식
gz-git commit --yes

# 간결한 형식
gz-git commit --format compact --yes

# JSON 형식
gz-git commit --format json --yes

# LLM 친화적 형식
gz-git commit --format llm --yes
```

## 필터링

```bash
# 특정 패턴만 포함
gz-git commit --include "backend-.*" --yes

# 특정 패턴 제외
gz-git commit --exclude "vendor|tmp" --yes

# 조합
gz-git commit --include "^service-" --exclude "test" --yes
```

## 예제

### Preview 모드 - 커밋 전 확인

```bash
# 1. 어떤 repos가 dirty한지 확인 (커밋 안 함)
gz-git commit

# 출력:
# Found 3 dirty repositories:
#   gzh-cli-gitforge (develop)  2 modified
#   gzh-cli-quality (main)       1 modified
#   gzh-cli-template (master)    3 modified
# Preview mode: Use --yes to commit

# 2. 메시지 준비 후 다시 미리보기
gz-git commit \
  -m "gzh-cli-gitforge:fix: auth bug" \
  -m "gzh-cli-quality:test: add tests" \
  -m "gzh-cli-template:docs: update"

# 3. 확인 후 실제 커밋
gz-git commit \
  -m "gzh-cli-gitforge:fix: auth bug" \
  -m "gzh-cli-quality:test: add tests" \
  -m "gzh-cli-template:docs: update" \
  --yes
```

### Per-repo 메시지 - 각기 다른 메시지

```bash
# 각 repo의 변경사항에 맞는 메시지
gz-git commit \
  -m "gzh-cli:feat: add wizard command" \
  -m "gzh-cli-gitforge:fix: fix clone authentication" \
  -m "gzh-cli-quality:refactor: improve code quality" \
  -m "gzh-cli-template:docs: update README" \
  --yes
```

**메시지 형식**: `repository-name:conventional-commit-message`

### 공통 메시지 - 모두 같은 메시지

```bash
# 의존성 일괄 업데이트
gz-git commit --all "chore: update dependencies to latest" --yes

# 설정 파일 동기화
gz-git commit --all "config: sync configuration files" --yes

# Lint 자동 수정
gz-git commit --all "style: apply linter auto-fixes" --yes
```

### Interactive 모드 - 에디터 사용

```bash
# 에디터에서 메시지 작성
gz-git commit -e

# 에디터 열림 (vim/nano 등):
# gzh-cli:feat: add new feature
# gzh-cli-gitforge:fix: fix clone issue
# gzh-cli-quality:docs: update docs

# 저장 후 종료 → 미리보기
# 확인 후 --yes로 재실행:
gz-git commit -e --yes
```

**$EDITOR 설정**:
```bash
export EDITOR=vim
export EDITOR=nano
export EDITOR="code --wait"  # VS Code
```

### JSON 파일 사용 - 자동화

**messages.json** 생성:
```json
{
  "gzh-cli": "feat: implement parallel execution",
  "gzh-cli-gitforge": "fix: handle authentication errors",
  "gzh-cli-quality": "test: add unit tests for parser",
  "gzh-cli-template": "docs: add usage examples"
}
```

**실행**:
```bash
gz-git commit --file messages.json --yes
```

**스크립트 예**:
```bash
#!/bin/bash
# 자동 커밋 스크립트

# 메시지 파일 생성
cat > /tmp/commit-messages.json <<EOF
{
  "backend": "feat: add API endpoint",
  "frontend": "feat: add UI component",
  "docs": "docs: update API documentation"
}
EOF

# 커밋 실행
gz-git commit --file /tmp/commit-messages.json --yes
```

### CI/CD에서 사용

```bash
# CI/CD 파이프라인에서 자동 커밋
gz-git commit \
  --all "ci: automated commit from CI pipeline" \
  --format json \
  --yes > commit-result.json

# 결과 확인
if [ $? -eq 0 ]; then
    echo "Commit successful"
    cat commit-result.json | jq '.summary'
else
    echo "Commit failed"
    exit 1
fi
```

### 패턴 필터링으로 선택적 커밋

```bash
# Feature 브랜치만 커밋
gz-git commit \
  --include "feature-.*" \
  --all "feat: implement new feature" \
  --yes

# Backend repos만 커밋
gz-git commit \
  --include "backend-.*" \
  -m "backend-api:feat: add endpoint" \
  -m "backend-db:fix: fix migration" \
  --yes

# Test/vendor 제외
gz-git commit \
  --exclude "test|vendor|tmp" \
  --all "chore: update dependencies" \
  --yes
```

### Dry-run으로 미리보기

```bash
# 실제 커밋 없이 어떤 repos가 처리될지 확인
gz-git commit \
  --all "test message" \
  --dry-run

# 출력에 "would-commit" 상태로 표시됨
```

## 주의사항

### Preview 모드 기본 동작

**기본값은 미리보기**:

```bash
# ✗ 커밋 안 됨 (미리보기만)
gz-git commit -m "repo:message"

# ✓ 실제 커밋
gz-git commit -m "repo:message" --yes
```

**이유**: 실수로 잘못된 메시지 커밋 방지

### 메시지 형식 주의

**Per-repo 형식**:
```bash
# ✓ 올바름
-m "gzh-cli:feat: add feature"

# ✗ 잘못됨
-m "feat: add feature"  # repo 이름 없음
-m "gzh-cli feat: add feature"  # 콜론(:) 누락
```

### Repo 이름 매칭

Repository 이름은 **디렉토리 이름**과 일치해야 함:

```bash
# 디렉토리 구조:
# ~/workspace/
#   gzh-cli/
#   gzh-cli-gitforge/

# ✓ 올바름
-m "gzh-cli:feat: feature"
-m "gzh-cli-gitforge:fix: bug"

# ✗ 잘못됨
-m "cli:feat: feature"  # 이름 불일치
```

### 공통 메시지 vs Per-repo

**언제 --all 사용?**:
- 모든 repos에 동일한 변경 (의존성, 설정)
- 단순 업데이트 (lint, format)

**언제 -m 사용?**:
- 각 repo마다 다른 변경사항
- 정확한 커밋 메시지 필요

### CI/CD 주의사항

**자동 커밋 시 확인**:
```bash
# ✓ 안전: 결과 확인
gz-git commit --all "ci: update" --yes --format json > result.json
if jq -e '.summary.success > 0' result.json; then
    echo "Commit successful"
fi

# ✗ 위험: 결과 미확인
gz-git commit --all "ci: update" --yes  # 실패해도 모름
```

### Interactive 모드 편집기

**편집기 미설정 시**:
```bash
# 오류 발생: $EDITOR not set
gz-git commit -e

# 해결:
export EDITOR=vim
gz-git commit -e
```

## 워크플로우 예시

### 일반적인 커밋 워크플로우

```bash
# 1. 상태 확인
gz-git status ~/workspace

# 2. Dirty repos 미리보기
gz-git commit

# 3. 메시지 준비 (미리보기)
gz-git commit \
  -m "backend-api:feat: add endpoint" \
  -m "frontend-app:feat: add UI component" \
  -m "docs:docs: update API docs"

# 4. 확인 후 실제 커밋
gz-git commit \
  -m "backend-api:feat: add endpoint" \
  -m "frontend-app:feat: add UI component" \
  -m "docs:docs: update API docs" \
  --yes

# 5. Push
gz-git push ~/workspace
```

## 관련 명령어

- [`gz-git status`](status-command.md) - Commit 전 상태 확인
- [`gz-git push`](push-command.md) - Commit 후 push
- [`gz-git diff`](diff-command.md) - 변경사항 확인
