# ISSUE: 중단된 integrate 기준선 worktree의 회수 경로가 없다

- status: open
- priority: P2
- category: product/resource-lifecycle
- created_at: 2026-10-14
- owner: `gzh-cli-gitforge/pkg/integrate`
- discovered-in: ce-devenv `codex-default-state-isolation` delivery

## Update (2026-10-22)

ee40ef50 deleted `pkg/integrate/check_make.go` and the baseline worktree, so
the original recovery case no longer exists. The only remaining integrate
temp root in non-test code is `MkdirTemp("", "gzh-git-archive-*")` at
`pkg/integrate/archive_support.go:67`, removed only by defer, so it can leak
on SIGKILL. The remaining scope narrows to reclaiming that archive scratch
directory under an ownership and liveness check, or closing this card if
that is not worth it (owner decision).

## 관측 증거

ce-devenv의 installer archive pipe 교착 때문에 `ce task run-finish`의 task 검사와
이어진 기준선 검사를 중단했다. 확인한 Git archive 자식에 SIGTERM, 검사와 provider에
SIGINT를 보냈다. CE는 BLOCKED를 기록했고 source는 `3dcf889e`, task는 `71417a47`로
유지됐다. 이후 installer를 수정한 별도 통합은 `6b4add3a`에서 DONE으로 끝났다.

중단된 실행의 기준선 worktree는 여전히 등록돼 있다.

- 경로: `/private/var/folders/34/phvdgy_j2_b3l66stw_vssbc0000gn/T/gz-git-integrate-base-2038393615/wt`
- Git 등록: ce-devenv `.git/worktrees/wt1`
- HEAD: `3dcf889eb70c134d453e8bffcb745cdac68c5498`, detached
- 재확인: tracked/untracked 변경 없음, 해당 경로를 참조하는 실행 명령 없음
- ignored 자료: 약 14MiB의 `.venv`, pytest/Python cache. `tmp/`는 비어 있다.
- 소비자 기록: ce-devenv `devlog/2026/2026-10-14-installer-archive-pipe.md`

관측은 실제 중단 실행에 근거한다. 별도 synthetic signal 재현이나 어느 stream이 먼저
교착됐는지까지 확인한 것은 아니다. worktree 이름만으로 삭제 권한을 추정하지 않는다.

## 기대와 실제

기대: 중단된 provider의 기준선 자원도 소유·실행 종료·Git 신원을 검증한 정식 회수
경로가 있어야 한다. 검증 불가, 변경된 HEAD, 사용자 변경, 실행 중인 프로세스는 보존한다.

실제: `integrate cleanup`은 push 완료된 task의 receipt/ref 회수 전용이다.
CE `run-discard`도 owner-bound task branch를 요구하므로 detached 기준선을 표현하지 못한다.
일반 `worktree remove` 명령은 있지만 CE lifecycle만 사용하도록 한 소비자의 정책 아래에서
이 기준선을 회수하는 실행 경로는 아니다. 강제 삭제·전체 prune으로 우회하지 않았다.

소스 근거: `pkg/integrate/check_make.go`의 기준선 생성·defer 정리,
`pkg/integrate/cleanup.go`의 receipt 기반 task 회수,
`cmd/gz-git/cmd/worktree.go`의 등록 확인·dry-run·non-force 제거.
기존 generic 명령으로 단일 잔여물을 처리하는 방법과 CE가 제공할 정식 회수 계약은
구분한다. 현재 소비자 정책에 대한 일회성 예외는 사용자 확인이 필요하다.

## 후속 범위

provider의 취소·정리 경로와 이미 남은 기준선에 대한 회수 계약을 함께 검토한다.
자원 이름만 훑어 일괄 삭제하거나 task 회수의 push 증명을 완화하지 않는다.
이번 등록은 제품 기능 구현 착수를 뜻하지 않는다. ISSUE-35의 실패 진단 유실,
ISSUE-34의 baseline prepare 문제와는 별도다.

## ISSUE-84에서 이관된 범위 (2026-10-20)

[ISSUE-84](084-integrate-temp-dirs-accumulate.md)는 테스트 상태 디렉터리만
회수하고 닫았다. SIGINT/SIGTERM으로 중단된 integrate 실행은 ISSUE-82 이후 defer로
정리되지만, SIGKILL이나 테스트 바이너리 timeout으로 끝난 실행은 여전히
`pkg/integrate`가 defer로만 지우는 임시 루트를 모두 `$TMPDIR`에 남긴다.
`gz-git-integrate-prepare-*`, `gz-git-integrate-base-*`와 `gz-git-readiness-*`(둘 다
등록된 worktree 포함), `gz-git-make-outcome-*`, `gz-git-integrate-golangci-lint-*`,
`gz-git-integrate-go-env-*`, `gzh-git-archive-*`, `gz-git-integrate-make-gate-*`(Windows),
`gz-git-make-probe-*.mk`가 해당한다. 같은 `$TMPDIR`를 공유하는 동시 실행이
있으므로 시작 시 회수는 소유·생존 판정 계약이 먼저 필요하다. 이 공백을 본 이슈의
회수 계약과 함께 다룬다.

## 관측 대상의 일회성 회수 (2026-10-14)

사용자가 해당 경로 한 곳에 한해 CE-only lifecycle 정책의 방법 예외를 명시 승인했다.
신원·clean·detached·프로세스 부재를 다시 확인하고, ce-devenv에서 정확한 절대 경로에
`gz-git worktree remove --dry-run` 후 같은 경로의 non-force 제거를 실행했다.
디렉터리와 Git 등록 부재를 확인하고 비어 있는 생성 부모만 `rmdir`로 제거했다.
`--force`, prune, raw Git 제거, 재귀 shell 삭제는 사용하지 않았다.

해당 잔여물의 회수는 끝났지만 정식 lifecycle 회수 공백은 남으므로 본 이슈는 open이다.
이 승인을 다른 경로나 후속 실행의 일반 예외로 사용하지 않는다.

## 등록 변경 검증

문서 2개만 바꾸며 Go 소스·golden·도구 pin은 바꾸지 않는다. 독립 strong 리뷰와
`git diff --check`, Markdown 포맷 검사, lint, gosec, dependency 보안 검사,
임시 build와 install audit는 통과했다.

`make quality-check`는 `test-unit-quality`의 기존 CE parity reference guard에서
실패했다. 실패를 통과로 간주하지 않는다. ISSUE-41에 이미 기록된 불일치이며,
이번 문서 등록을 위해 reference 재녹화나 설치 도구 변경을 하지 않는다.
