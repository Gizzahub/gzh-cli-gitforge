---
archived-at: 2026-02-24T16:10:35+09:00
verified-at: 2026-02-24T16:10:35+09:00
verification-summary: |
  - Verified: sync update strategy errors include git stderr/exit code and stale `.git/index.lock` cleanup runs before pull/reset/rebase.
  - Evidence: `pkg/repository/update.go` uses `...failed (exit %d): %s` patterns and `removeStaleIndexLock`; `mise x -- go test ./pkg/repository` passed.
id: TASK-011
title: "sync 실패 시 git stderr를 에러 메시지에 포함"
type: bug

priority: P1
effort: S

parent:
depends-on: []
blocks: []

created-at: 2026-02-23T00:00:00Z
status: done
started-at: 2026-02-24T14:58:00+09:00
completed-at: 2026-02-24T15:05:00+09:00
completion-summary: "Appended stderr to error messages for fetch, pull, reset, and rebase strategies. Added lock file removal check before git operations."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./pkg/repository/..."
    result: "pass: ok github.com/gizzahub/gzh-cli-gitforge/pkg/repository 0.854s"
---

## Problem

`gz-git workspace sync`에서 git 명령이 실패할 때, 실제 원인(git stderr)이 사용자에게 전달되지 않는다.

### 재현

1. `.git/index.lock` 파일이 잔존하는 repo가 있는 상태에서 `gz-git workspace sync` 실행
2. 출력: `✗ gizzahub-infocenter  reset failed: exit status 128`
3. 실제 git stderr: `fatal: Unable to create '.git/index.lock': File exists.`

사용자는 `exit status 128`만 보고 원인을 알 수 없다.

### 근본 원인

`pkg/repository/update.go`의 `applyResetStrategy`와 `applyPullStrategy`에서 `resetResult.Error`를 래핑할 때 exit code만 전달되고 stderr 내용이 누락됨.

```go
// update.go:360-361
if resetResult.ExitCode != 0 {
    return nil, fmt.Errorf("reset failed: %w", resetResult.Error)
}
```

`applyPullStrategy`(line 320-321)도 동일한 패턴.

## Proposed Fix

### 1. (필수) 에러 메시지에 git stderr 포함

executor의 결과에서 stderr 내용을 추출하여 에러 메시지에 포함.

```
Before: reset failed: exit status 128
After:  reset failed (exit 128): Unable to create '.git/index.lock': File exists.
```

영향 범위: `applyResetStrategy`, `applyPullStrategy`, `applyRebaseStrategy` 공통 적용.

### 2. (선택) stale lock 파일 사전 감지

reset/pull 실행 전 `.git/index.lock` 존재 여부를 확인하고 경고 또는 자동 제거.
`applyRebaseStrategy`에는 dirty working tree 사전 체크가 있지만, reset/pull에는 없음.

## Notes

- parallel sync 환경에서 lock 파일 잔존 확률이 높음 (프로세스 crash, kill 등)
- config에 `strategy: pull` 설정인데 출력이 "Successfully reset"인 부분은 별도 확인 필요
