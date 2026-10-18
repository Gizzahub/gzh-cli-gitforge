# Readiness cleanup 실패 보존 계약

## 확인한 문제

2026-10-18 catalog `singlepageweb-devbox`의 ISSUE-059에서 readiness runner의
검사와 `ready` payload 뒤에 worktree 정리 오류가 발생했다. 통합 provider는 이를
`measurement unavailable`로 판정했으며 source 통합은 실행되지 않았다.
원시 진단과 CE receipt의 소유자는 해당 소비 저장소다.

기준 source `f96bdca4`의 `mergeCleanupError`는 독립 30초 context로 Git worktree를
제거하며, Git 오류 뒤에도 `os.RemoveAll(dir)`을 수행했다. 바깥 workspace 정리 역시
실패 자원을 보존하지 않았다. 이 경로에서는 디렉터리가 삭제되고 Git admin 등록만
남을 수 있다. `worktreeRemoveForce`는 nonzero 결과의 stderr만 표시해 종료 코드·실행
시간·underlying error를 잃었다. 실제 소비 실행의 빈 stderr가 timeout 때문이었는지는
이 기록만으로 확정하지 않는다.

## 변경과 수용 경계

- readiness 정리 예산을 named constant 5분으로 둔다. 기존 Git executor 예산에 맞춘
  초기 상한이며 측정된 p95나 영구적인 완료 시간 보장이 아니다.
- 취소된 runner와 독립적으로 정리하되 bounded context를 유지한다.
- Git 제거 실패 시 해당 worktree와 바깥 workspace를 보존하고 실패를 반환한다.
  성공으로 바꾸거나 자동 재시도·전역 prune·파일시스템 강제 삭제로 우회하지 않는다.
- Git 제거 오류에 종료 코드·실행 시간·underlying/context 원인을 보존한다.
- 이 수정은 이후 실행의 실패 보존을 개선한다. 이미 사라진 소비 worktree나 stale
  registration의 복구, 과거 gate의 소급 PASS, 소비 문서의 source 통합을 의미하지 않는다.

## 검증·전달

구현 기준은 standard, 독립 리뷰는 strong 등급이다. 최종 helper 추출 뒤 독립 리뷰는
PASS이며, 별도 실행한 다음 다섯 회귀 검사가 `-count=1`에서 PASS(14.481초)였다.

```sh
go test ./pkg/integrate -run 'Test(RunContractTimeoutCleansDetachedWorktrees|ExecuteReadinessParentDeadlineDoesNotInventRunnerBudgetTimeout|MergeCleanupErrorDoesNotPruneUnrelatedWorktrees|RunContractRetainsWorkspaceWhenGitRemovalFails|WorktreeRemoveForceReportsTimeoutDiagnostics)$' -count=1
```

`make format-check lint-check`는 최종 구현에서 PASS(린트 0 issues)였다. 별도
`make security-code security-deps`도 exit0이며 gosec 이슈0, govulncheck 호출 경로의
취약점0이다. 가져온 패키지3·요구 모듈1의 비호출 취약점 관측은 원본 로그에 남으며
전체 의존성에 취약점이 없다고 일반화하지 않는다. 최초 린트 실패5건은 수정했고 로그
`tmp/readiness-cleanup-precommit-security.log`를 보존했다. 보안 결과는
`tmp/readiness-cleanup-security.log`다.

`VERIFY_BASE=origin/master make verify-commit`은 새 검증 문서를 unknown input으로
분류해 selector exit3으로 전체 검사를 요구했다. 테스트 실패나 좁은 프로필 PASS로
표시하지 않는다. native 통합의 전체 게이트 결과와 실제 push·회수 receipt는 별도로
확인해야 하며 위 집중 검사만으로 source 통합 완료를 주장하지 않는다.

설치된 도구를 수정하거나 PATH를 바꾸지 않는다. 공개 릴리스·workstation 설치와
소비 저장소의 재통합은 각각 지정 경로와 실제 receipt로 확인한다.

## 실제 native 통합 시도

후보 `09565294`를 push한 뒤 CE `run-finish readiness-cleanup-preservation`을 실행했다.
provider는 `refs/heads/master` ownership 대기에서 전체 검사에 진입하지 못했다.
17분 이상 대기를 확인하고 이번 provider만 SIGINT로 취소했다. CE 결과는 BLOCKED이며
source `1db93c1b`는 그대로이고 source push·worktree 회수는 모두 false다.
다른 세션 lock은 보존했다. 시간 상한 공백은 [ISSUE-54](../../../tasks/issue/54-target-ownership-wait-unbounded.md)에
등록했다. 집중 검사 PASS와 실제 전체 게이트 미실행을 구분하며 source 전달은 미완료다.
