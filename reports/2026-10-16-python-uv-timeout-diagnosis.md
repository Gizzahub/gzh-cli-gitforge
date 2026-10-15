# Python/UV 전달 timeout과 Bash fault fixture 격리

## 조사 기준

TASK-324 / ISSUE-326의 후속 진단이다. source는
`0e232f3e56e09028a74d0c5d95dea9bc94cb8089`이며 기존 Python/UV marker 후보는
`bde465abcaf5fa72a4876550967c5f7b73802adb`다. 구현과 독립 리뷰 요구는 strong이다.
검사 선택·timeout·도구 pin·제품 환경을 완화하지 않는다.

## 서로 다른 두 실패 구간

이전 성공 진단은 최상위 PASS 379개에 787.150초, package 종료에 844.996초였다.
PASS 출력 이후 57.372초의 Go 처리 시간이 있었지만 cache/pipe 결함은 미확정이다.
이 이전 실행의 원본은 source primary의
`tmp/python-uv-tracked-codes-layout/native-timeout-diagnostic/`에 있다.

이번 source `0e232f3e`의 canonical 실패는 최상위 PASS 367개의 합계가
1199.320초였고, package가 기존 20분 제한에서 1200.363초 후 종료됐다.
당시 `TestCheck_ReleaseFromRemoteRefStillRequiresCleanCheckout`는 약 1초째였고
stack은 fixture 구성의 `runGit` / `Cmd.Wait`였다. 이 개별 테스트의 20분 교착을
뜻하지 않는다. source·suite·실행 시점이 다른 관측으로 회귀나 호스트 부하의 원인을
확정하지 않는다. 이 실행은 모든 테스트 PASS 이후의 Go 처리 시간을 측정하지 못했다.

## 원인을 구분한 추가 실행

최장 세 테스트에 같은 `-short --cover -count=1 -timeout=20m -parallel=1`과
Git Trace2·Go runtime trace를 적용했다. 전체 gate를 대체하는 실행이 아니다.

| 최상위 테스트                                                 |   수정 전 |   수정 후 |
| ------------------------------------------------------------- | --------: | --------: |
| `TestCleanupOnlyRetryLeaseGeneration`                         |   23.73초 |   31.00초 |
| `TestRun_ReclaimRefusesNestedRepoThatIsNotProvablyDisposable` |   42.59초 |   54.25초 |
| `TestRunGitReclaimRelativePushEndpoint`                       |   56.19초 |   34.37초 |
| package 종료                                                  | 123.025초 | 120.182초 |

두 실행 모두 exit 0이다. 수정 전 syscall profile의 지연은 대부분 자식
프로세스 종료 대기였다. Git Trace2는 Git 실행 내부 시간이며 Bash 시작이나
모든 부모 대기 시간을 포함하지 않으므로 둘의 합계를 같은 지표로 취급하지 않는다.

## 재현된 fixture 격리 결함

`cleanupRetryFX.interceptGit`의 Bash wrapper가 호출자의 `BASH_ENV`를 상속했다.
호출자 initializer가 먼저 실행되면 작성한 fault 프로그램을 대체할 수 있었다.
synthetic initializer가 marker를 쓰고 exit 73하도록 한 회귀는 수정 전 RED였다.
helper executor에만 `BASH_ENV=`를 추가한 뒤, 의도한 fault exit 42와 실제 Git
위임 성공을 모두 확인하고 marker 부재를 검사해 GREEN을 얻었다.

별도 시작 비용 실험에서 `/bin/bash -c 'exit 0'`의 중앙값은 ambient initializer
약 0.184초, 빈 synthetic initializer 약 0.005초였다. component 비교에서는
빈 초기화 0.007초, FD 조회 두 개만 포함한 초기화 0.010초, 전체 ambient 초기화
0.194초였다. FD 조회만으로 비용을 설명할 수 없으며, 이 실험은 수용 검사나
호스트 초기화 파일 수정의 근거가 아니다.

수정 범위는 테스트 전용 fault injector다. 제품 executor, Make 실행 환경,
실제 readiness runner, 사용자 startup 파일과 PATH는 변경하지 않는다.
상대 endpoint 시험은 빨라졌지만 다른 두 시험은 느려졌으므로 전체 timeout의
완전 해소를 주장하지 않는다. 실제 canonical/native 결과는 별도로 검증해야 한다.

## 증거와 재현

원본은 source primary의 `tmp/python-uv-timing-20261016/`에 보존한다.
JSON 이벤트·Trace2·runtime trace·시작 비용 표본·RED/GREEN 로그를 구분한다.
호출자 startup을 차단하는 회귀는 다음 명령으로 재현한다.

```bash
GOWORK=off go test -count=1 ./pkg/integrate \
  -run '^TestCleanupInterceptIgnoresAmbientBashInitializer$'
```

`-run`으로 좁힌 진단 성공은 전체 품질·baseline 비악화·native 전달 성공을
대신하지 않는다. 일반적인 캐시 우회, timeout 연장, fixture 공유에 의한 안전성
단언 축소는 이 수정에 포함하지 않는다.
