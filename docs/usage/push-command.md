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

| 형식                                | 설명               |
| ----------------------------------- | ------------------ |
| `branch`                            | 같은 이름으로 push |
| `local:remote`                      | 로컬 → 원격 브랜치 |
| `+local:remote`                     | Force push         |
| `refs/heads/main:refs/heads/master` | 전체 ref 경로      |

### 자동 검증

Refspec은 실행 전 자동으로 검증되며, **순서가 정해져 있습니다**:

1. 형식 검증 (Git 브랜치명 규칙)
1. 커밋 유무 확인 — 커밋이 하나도 없으면 `no-commits`로 보고하고 멈춥니다
1. 소스 브랜치 존재 확인
1. 커밋 수 계산 / 원격 브랜치 확인

2번이 3번보다 먼저인 이유는 진단이 달라지기 때문입니다. 커밋이 없는 저장소에
`--refspec HEAD:master`를 주면 "소스 브랜치 'HEAD'가 없다"는 말은 문자상 맞지만,
"없는 브랜치를 지정했다"로 읽혀 refspec을 고치게 만듭니다. 실제 사실은 "아직
아무것도 없다"이고, 그건 refspec을 바꿔서 해결되지 않습니다.

반대로 1번이 2번보다 먼저인 이유는 잘못된 refspec 형식은 **어느 저장소에서든
똑같이 틀린** 호출자의 오류이기 때문입니다. 빈 저장소라는 이유로 그 오류가
가려지면, 커밋이 있는 다음 저장소에서 같은 명령이 다시 실패합니다.

### 에러 예시

```bash
# 소스 브랜치 없음
✗ my-repo (master)  failed
  ⚠ refspec source branch 'develop' not found (current: master)

# 잘못된 형식
Error: invalid refspec: contains invalid character
```

## 주요 옵션

| 옵션               | 설명                      | 기본값 |
| ------------------ | ------------------------- | ------ |
| `--refspec`        | 브랜치 매핑               | -      |
| `--remote`         | Push할 원격지 (반복 가능) | origin |
| `-f, --force`      | Force push                | false  |
| `-d, --scan-depth` | 스캔 깊이                 | 1      |
| `-j, --parallel`   | 병렬 처리 수              | 10     |
| `-n, --dry-run`    | 미리보기                  | false  |
| `--include`        | 포함 패턴 (regex)         | -      |
| `--exclude`        | 제외 패턴 (regex)         | -      |

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

| 아이콘 | 상태        | 의미                                       |
| ------ | ----------- | ------------------------------------------ |
| `✓`    | success     | Push 성공                                  |
| `⊖`    | up-to-date  | Push할 커밋 없음                           |
| `✗`    | failed      | Push 실패                                  |
| `⊘`    | skipped     | 건너뜀                                     |
| `⚠`    | no-commits  | 커밋이 하나도 없어 push할 대상 자체가 없음 |
| `⚠`    | no-remote   | 원격지가 설정되지 않음                     |
| `⚠`    | no-upstream | upstream이 설정되지 않음                   |

`no-commits` / `no-remote` / `no-upstream`은 실패가 아니라 **상태**입니다. 셋 다
`gz-git push`가 고칠 수 있는 것이 아니라 저장소가 아직 그 단계에 이르지 못한
것이므로, 종료 코드를 실패로 만들지 않습니다. 새로 만들어 두고 아직 아무것도
커밋하지 않은 디렉터리가 스캔에 섞여 있을 때 전체 실행이 빨갛게 물드는 것을
막기 위함입니다.

## 필터링

`--include` / `--exclude`는 **이번 실행 한 번**의 대상만 좁힙니다. 매번 같은
regex를 붙이고 있다면 그것은 그 트리의 성질이지 이번 실행의 성질이 아니므로,
`.gz-git.yaml`의 `defaults.scan.exclude`(탐색에서 제외)나 `access: read-only`
(스캔은 하되 push 거부)로 옮기는 편이 맞습니다. 어느 쪽을 골라야 하는지는
[ad-hoc bulk push의 범위를 가르는 선언](workspace-command.md#ad-hoc-bulk-push%EC%9D%98-%EB%B2%94%EC%9C%84%EB%A5%BC-%EA%B0%80%EB%A5%B4%EB%8A%94-%EC%84%A0%EC%96%B8)을
보세요.

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
