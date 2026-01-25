# gz-git push

여러 repository에 병렬로 push.

## 기본 사용법

```bash
# 현재 디렉토리 + 1레벨 하위 repos push
gz-git push

# 특정 디렉토리
gz-git push ~/mydevbox

# Dry-run
gz-git push --dry-run
```

## Refspec (브랜치 매핑)

로컬 브랜치를 다른 이름의 원격 브랜치로 push.

```bash
# develop → master
gz-git push --refspec develop:master

# Force push (--force-with-lease 사용)
gz-git push --refspec +develop:master

# 여러 원격지에 동시 push
gz-git push --refspec develop:master --remote origin --remote backup
```

### Refspec 형식

| 형식 | 설명 |
|------|------|
| `branch` | 같은 이름으로 push |
| `local:remote` | 로컬 → 원격 브랜치 |
| `+local:remote` | Force push |
| `refs/heads/main:refs/heads/master` | 전체 ref 경로 |

### 자동 검증

Refspec은 실행 전 자동으로 검증됩니다:

- 형식 검증 (Git 브랜치명 규칙)
- 소스 브랜치 존재 확인
- 커밋 수 계산
- 원격 브랜치 확인

### 에러 예시

```bash
# 소스 브랜치 없음
✗ my-repo (master)  failed
  ⚠ refspec source branch 'develop' not found (current: master)

# 잘못된 형식
Error: invalid refspec: contains invalid character
```

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--refspec` | 브랜치 매핑 | - |
| `--remote` | Push할 원격지 (반복 가능) | origin |
| `-f, --force` | Force push | false |
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-n, --dry-run` | 미리보기 | false |
| `--include` | 포함 패턴 (regex) | - |
| `--exclude` | 제외 패턴 (regex) | - |

## 출력 예시

```
Pushing 5 repositories...

✓ gzh-cli (master → origin)           2 commits  150ms
✓ gzh-cli-core (master → origin)      1 commit   120ms
✓ gzh-cli-gitforge (develop → origin) 3 commits  180ms
⊖ gzh-cli-quality (main)              up-to-date
✗ gzh-cli-template (master)           rejected (non-fast-forward)

Summary: 3 pushed, 1 up-to-date, 1 failed
```

## 상태 아이콘

| 아이콘 | 상태 | 의미 |
|--------|------|------|
| `✓` | success | Push 성공 |
| `⊖` | up-to-date | Push할 커밋 없음 |
| `✗` | failed | Push 실패 |
| `⊘` | skipped | 건너뜀 |

## 필터링

```bash
# 특정 패턴만
gz-git push --include "gzh-cli-.*"

# 제외
gz-git push --exclude "test|tmp"
```

## 예제

### Release 워크플로우

```bash
# develop을 master로 push (모든 repos)
gz-git push --refspec develop:master --dry-run

# 확인 후 실제 push
gz-git push --refspec develop:master
```

### 여러 원격지 동기화

```bash
# origin과 backup 둘 다 push
gz-git push --remote origin --remote backup

# GitLab mirror 설정
gz-git push --remote gitlab --refspec master:master
```

### CI/CD 연동

```bash
#!/bin/bash
# deploy.sh

# 1. 상태 확인
gz-git status --format json > status.json

# 2. Dirty repos 확인
if jq -e '.[] | select(.dirty == true)' status.json > /dev/null; then
    echo "Uncommitted changes found!"
    exit 1
fi

# 3. Push
gz-git push --refspec develop:master

# 4. 결과 확인
if [ $? -eq 0 ]; then
    echo "Deploy successful"
else
    echo "Deploy failed"
    exit 1
fi
```

## 주의사항

### Force Push

```bash
# --force-with-lease 사용 (더 안전)
gz-git push --refspec +develop:master

# --force 사용 (주의!)
gz-git push --force
```

`+` prefix는 내부적으로 `--force-with-lease`를 사용하여 다른 사람의 커밋을 덮어쓰지 않도록 보호합니다.

### Protected Branches

Protected branch로 push 시 실패할 수 있습니다:

```
✗ my-repo (main)  rejected (protected branch)
```

이 경우 Git forge 설정에서 권한을 확인하세요.
