# gz-git cleanup

Git 리소스 정리 명령어.

## 서브커맨드

| 커맨드 | 설명 |
|--------|------|
| `branch` | 브랜치 정리 (merged, stale, gone) |

## cleanup branch

Merged, stale, gone 브랜치 정리.

```bash
# Dry-run (기본값) - 삭제 대상만 표시
gz-git cleanup branch

# 실제 삭제
gz-git cleanup branch --execute

# 특정 타입만
gz-git cleanup branch --type merged
gz-git cleanup branch --type stale
gz-git cleanup branch --type gone
```

## 브랜치 타입

| 타입 | 설명 | 감지 방법 |
|------|------|----------|
| `merged` | 이미 merge된 브랜치 | `git branch --merged` |
| `stale` | 오래된 브랜치 (활동 없음) | 마지막 커밋 날짜 |
| `gone` | 원격에서 삭제된 브랜치 | `[gone]` 표시 |

## 주요 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--type` | 정리할 브랜치 타입 | 전체 |
| `--execute` | 실제 삭제 실행 | false (dry-run) |
| `--days` | Stale 기준 일수 | 90 |
| `--protected` | 보호할 브랜치 패턴 | main,master,develop |
| `-d, --scan-depth` | 스캔 깊이 | 1 |
| `-j, --parallel` | 병렬 처리 수 | 10 |
| `-f, --force` | 강제 삭제 (-D 사용) | false |

## 출력 예시

### Dry-run (기본)

```
Scanning 5 repositories for cleanup...

gzh-cli:
  [merged]  feature/old-feature     → merged to master 30 days ago
  [stale]   experiment/test         → no activity for 120 days
  [gone]    feature/deleted-remote  → remote branch deleted

gzh-cli-core:
  [merged]  fix/typo                → merged to main 7 days ago

Summary:
  merged: 2 branches
  stale:  1 branch
  gone:   1 branch
  total:  4 branches

Run with --execute to delete these branches.
```

### 실제 삭제

```bash
gz-git cleanup branch --execute
```

```
Deleting branches...

✓ gzh-cli: feature/old-feature (merged)
✓ gzh-cli: experiment/test (stale)
✓ gzh-cli: feature/deleted-remote (gone)
✓ gzh-cli-core: fix/typo (merged)

Deleted 4 branches across 2 repositories.
```

## 보호 브랜치

기본적으로 보호되는 브랜치:

- `main`
- `master`
- `develop`
- `release/*`

### 커스텀 보호 패턴

```bash
# 추가 보호
gz-git cleanup branch --protected "main,master,develop,staging,prod-*"

# Config에서 설정
# .gz-git.yaml
branch:
  protectedBranches: [main, master, develop, staging, "prod-*"]
```

## Stale 기준

```bash
# 60일 이상 활동 없는 브랜치
gz-git cleanup branch --type stale --days 60

# 180일 이상
gz-git cleanup branch --type stale --days 180
```

## 예제

### 정기 정리 스크립트

```bash
#!/bin/bash
# weekly-cleanup.sh

cd ~/mydevbox

echo "=== Branch Cleanup Preview ==="
gz-git cleanup branch

echo ""
read -p "Proceed with cleanup? (y/N) " confirm

if [[ $confirm == [yY] ]]; then
    gz-git cleanup branch --execute
    echo "Cleanup complete!"
else
    echo "Cleanup cancelled."
fi
```

### CI/CD에서 자동 정리

```bash
#!/bin/bash
# ci-cleanup.sh

# Merged 브랜치만 자동 삭제
gz-git cleanup branch --type merged --execute

# Stale은 경고만
gz-git cleanup branch --type stale
```

### 특정 타입별 정리

```bash
# Gone 브랜치만 정리 (원격에서 삭제된 것)
gz-git cleanup branch --type gone --execute

# Merged만 정리
gz-git cleanup branch --type merged --execute

# 오래된 실험 브랜치 정리 (30일 기준)
gz-git cleanup branch --type stale --days 30 --execute
```

## 주의사항

### Force 삭제

```bash
# merge되지 않은 브랜치도 삭제 (git branch -D)
gz-git cleanup branch --force --execute
```

`--force`는 데이터 손실 가능성이 있으므로 주의!

### 복구

삭제된 브랜치는 `git reflog`로 복구 가능:

```bash
# 삭제된 브랜치의 마지막 커밋 찾기
git reflog

# 복구
git checkout -b recovered-branch <commit-hash>
```

### 원격 브랜치

`cleanup branch`는 로컬 브랜치만 삭제합니다. 원격 브랜치 삭제는:

```bash
# 원격 브랜치 삭제
git push origin --delete feature/old-branch
```
