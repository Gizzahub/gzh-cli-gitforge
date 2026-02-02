# gz-git diff

여러 repository의 변경사항을 일괄 확인하는 명령어.

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위 스캔
gz-git diff

# 특정 디렉토리
gz-git diff ~/mydevbox

# 단일 repo
gz-git diff /path/to/single/repo
```

## 출력 예시

```
Checking diffs in 3 repositories...

=== gzh-cli-gitforge (develop) ===
Modified files: 2
  M cmd/clone.go
  M pkg/repository/client.go

diff --git a/cmd/clone.go b/cmd/clone.go
--- a/cmd/clone.go
+++ b/cmd/clone.go
@@ -42,7 +42,7 @@ func runClone(cmd *cobra.Command, args []string) error {
-    return client.Clone(url, path)
+    return client.CloneWithAuth(url, path, authToken)

=== gzh-cli-quality (main) ===
Modified files: 1
  M README.md

diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -10,0 +11,2 @@
+## New Feature
+Description here.

Summary: 3 files modified across 2 repositories
```

## Staged vs Unstaged

| 타입 | 플래그 | 대상 | 사용 시점 |
|------|--------|------|-----------|
| **Unstaged** | (기본값) | Working tree 변경사항 | 일반적인 확인 |
| **Staged** | `--staged` | Stage된 변경사항 | **Commit 전 확인** |

### Unstaged (기본값)

```bash
# Working tree의 수정사항
gz-git diff ~/workspace
```

**포함**:
- Modified 파일
- Deleted 파일
- `--include-untracked` 사용 시 untracked 파일

### Staged

```bash
# Stage된 변경사항만
gz-git diff --staged ~/workspace
```

**포함**:
- `git add`로 stage된 변경사항
- Commit될 내용

**사용 예**: Commit 전 마지막 확인

## 크기 제한

기본값: **100KB per repository**

```bash
# 기본 크기 제한 (100KB)
gz-git diff ~/workspace

# 크기 제한 늘리기 (500KB)
gz-git diff --max-size 500 ~/workspace

# 크기 제한 없음 (주의: 매우 큰 diff 가능)
gz-git diff --max-size 0 ~/workspace
```

**Truncation 동작**:
```
=== large-project (main) ===
Modified files: 15
  M src/large-file.go
  ...

[Diff truncated: exceeds 100KB limit]
Use --max-size to increase limit
```

## 파일 상태 아이콘

| 아이콘 | 상태 | 의미 |
|--------|------|------|
| `M` | Modified | 수정됨 |
| `A` | Added | 추가됨 (staged) |
| `D` | Deleted | 삭제됨 |
| `R` | Renamed | 이름 변경됨 |
| `C` | Copied | 복사됨 |
| `?` | Untracked | 미추적 (--include-untracked 시) |

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `--staged` | Staged 변경사항만 | false |
| `--include-untracked` | Untracked 파일 포함 | false |
| `-U, --context` | Diff context 줄 수 | 3 |
| `--max-size` | 최대 diff 크기 (KB) | 100 |
| `--no-content` | 요약만 (diff 내용 생략) | false |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |
| `-f, --format` | 출력 형식 | default |

## 출력 형식

```bash
# 기본 형식 (full diff)
gz-git diff

# 간결한 형식 (파일 목록만)
gz-git diff --format compact

# JSON 형식
gz-git diff --format json

# LLM 친화적 형식
gz-git diff --format llm
```

## 필터링

```bash
# 특정 패턴만 포함
gz-git diff --include "backend-.*"

# 특정 패턴 제외
gz-git diff --exclude "vendor|tmp"

# 조합
gz-git diff --include "^service-" --exclude "test"
```

## 예제

### 기본 diff - unstaged 변경사항

```bash
# Working tree 수정사항 확인
gz-git diff ~/workspace

# 출력:
# === gzh-cli-gitforge (develop) ===
# Modified files: 2
#   M cmd/clone.go
#   M pkg/repository/client.go
#
# diff --git a/cmd/clone.go ...
```

### Staged 변경사항만 - commit 전 확인

```bash
# Stage된 내용 확인 (commit될 내용)
gz-git diff --staged ~/workspace

# Commit 전 워크플로우:
# 1. git add (여러 repos에서)
# 2. gz-git diff --staged (확인)
# 3. gz-git commit (커밋)
```

### Summary only - 파일 목록만

```bash
# Diff 내용 없이 파일 목록만
gz-git diff --no-content ~/workspace

# 출력:
# === gzh-cli-gitforge (develop) ===
# Modified files: 2
#   M cmd/clone.go
#   M pkg/repository/client.go
#
# === gzh-cli-quality (main) ===
# Modified files: 1
#   M README.md
#
# Summary: 3 files modified across 2 repositories
```

**사용**: 빠른 개요 확인

### Untracked 파일 포함

```bash
# Untracked 파일도 표시
gz-git diff --include-untracked ~/workspace

# 출력:
# === my-project (main) ===
# Modified files: 2
#   M src/api.go
#   ? tmp/debug.log
#
# Untracked files:
#   tmp/debug.log
```

### JSON 형식 - 스크립팅

```bash
# JSON으로 출력
gz-git diff --format json ~/workspace > diffs.json

# jq로 파싱
cat diffs.json | jq '.repositories[] | select(.modified_files > 0)'

# 스크립트 예시:
#!/bin/bash
diffs=$(gz-git diff --format json ~/workspace)
count=$(echo "$diffs" | jq '.summary.total_files')
echo "Total modified files: $count"
```

### Context 조정

```bash
# Context 줄 수 줄이기 (간결)
gz-git diff -U 1 ~/workspace

# Context 늘리기 (상세)
gz-git diff -U 5 ~/workspace

# Context 없음 (변경사항만)
gz-git diff -U 0 ~/workspace
```

**기본값**: 3줄 (git 표준)

### 크기 제한 조정

```bash
# 기본 100KB
gz-git diff ~/workspace

# 500KB까지 허용
gz-git diff --max-size 500 ~/large-repos

# 크기 제한 없음 (주의: 매우 큰 출력)
gz-git diff --max-size 0 ~/workspace
```

**주의**: max-size 0은 메모리 문제 유발 가능

### 패턴 필터링

```bash
# Backend repos만
gz-git diff --include "backend-.*" ~/workspace

# Frontend 제외
gz-git diff --exclude "frontend-.*" ~/projects

# Service repos만, test 제외
gz-git diff \
  --include "^service-" \
  --exclude "test" \
  ~/microservices
```

### Commit 전 워크플로우

```bash
# 1. 변경사항 확인 (unstaged)
gz-git diff ~/workspace

# 2. 파일 stage
cd workspace/my-project
git add src/api.go src/utils.go
cd ../..

# 3. Staged 확인
gz-git diff --staged ~/workspace

# 4. 문제 없으면 commit
gz-git commit \
  -m "my-project:feat: add new API endpoint" \
  --yes
```

### LLM에 변경사항 전달

```bash
# LLM 친화적 형식으로 출력
gz-git diff --format llm ~/workspace > changes.txt

# LLM에게 전달:
# "다음 변경사항을 리뷰해주세요:"
# <changes.txt 내용 붙여넣기>
```

## 주의사항

### Staged vs Unstaged 구분

**기본값은 unstaged**:
```bash
# Working tree 변경사항
gz-git diff ~/workspace
```

**Commit 전에는 staged 확인**:
```bash
# Stage된 내용 확인
gz-git diff --staged ~/workspace
```

### 크기 제한

**기본 100KB** - 대부분 충분:
```
[Diff truncated: exceeds 100KB limit]
```

**해결**:
```bash
# 크기 늘리기
gz-git diff --max-size 500 ~/workspace

# Summary만 보기
gz-git diff --no-content ~/workspace
```

### Untracked 파일

**기본값**: Untracked 파일 제외

```bash
# Untracked 포함하려면
gz-git diff --include-untracked ~/workspace
```

**주의**: Build artifacts, logs 등도 표시될 수 있음

### 출력량

**Full diff는 매우 길 수 있음**:

```bash
# ✗ 너무 긴 출력
gz-git diff ~/workspace  # 수십 repos × 수백 줄

# ✓ Summary로 제한
gz-git diff --no-content ~/workspace

# ✓ 또는 compact 형식
gz-git diff --format compact ~/workspace
```

### Context 라인

**기본 3줄** - Git 표준:

```bash
# 변경 전후 3줄씩 표시
gz-git diff ~/workspace
```

**줄이기**:
```bash
# 간결하게
gz-git diff -U 1 ~/workspace
```

### Binary 파일

Binary 파일은 diff 불가:
```
Binary files a/image.png and b/image.png differ
```

### Large Files

**매우 큰 파일 주의**:

```bash
# 100KB 제한으로 truncate
gz-git diff ~/workspace

# 또는 summary만
gz-git diff --no-content ~/workspace
```

## 실용 패턴

### 빠른 변경사항 확인

```bash
# 파일 목록만 빠르게
gz-git diff --no-content --format compact ~/workspace
```

### Commit 준비

```bash
# 1. Unstaged 확인
gz-git diff ~/workspace

# 2. Stage
# ... git add ...

# 3. Staged 재확인
gz-git diff --staged ~/workspace

# 4. Commit
gz-git commit --yes
```

### 코드 리뷰 준비

```bash
# LLM 형식으로 변경사항 추출
gz-git diff --format llm --no-content ~/workspace > summary.txt

# 상세 diff가 필요하면
gz-git diff --format llm ~/workspace > full-diff.txt
```

### CI/CD 검증

```bash
# Unstaged 변경사항 감지
diffs=$(gz-git diff --format json ~/workspace)
count=$(echo "$diffs" | jq '.summary.total_files')

if [ "$count" -gt 0 ]; then
    echo "Error: Uncommitted changes detected"
    echo "$diffs" | jq '.repositories[] | select(.modified_files > 0)'
    exit 1
fi
```

## 관련 명령어

- [`gz-git status`](status-command.md) - 전체 상태 확인 (dirty repos)
- [`gz-git commit`](commit-command.md) - Diff 확인 후 커밋
- [`gz-git fetch`](fetch-command.md) - Remote와 비교 전 fetch
