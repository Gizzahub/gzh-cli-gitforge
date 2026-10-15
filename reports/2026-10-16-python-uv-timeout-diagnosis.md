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


## 후속 검증 시간 기록 설계

이 절은 수집 구현 전의 설계다. 기존 Make/DVA 예산 선언을 정본으로 유지하고
새 서비스·자동 예산 조정·실패 재시도를 도입하지 않는다. 과거 20분 package 제한은
당시 관측이며 현재 `.make/test.mk` 선언과 구분한다. 초기 예산을 측정 p95로
표시하지 않는다.

각 실행은 task-owned `tmp/<execution>/`에 원본 로그와 불변 JSON 결과 하나를
보존한다. 내부 기록에는 다음 항목을 포함한다. 공개 CLI schema를 추가하지 않는다.

- 실행 ID, 관측 UTC, 저장소 ID, source/base commit과 전체 dirty diff digest,
  정확한 argv·작업 디렉터리·선택 profile/stage 및 실제 선택 package 목록.
- runner·도구 revision과 해시, 결과에 영향을 주는 설정의 digest, worker·병렬 수,
  outer/stage/package 예산. 비밀 환경 값은 저장하지 않고 필요한 비밀 외 입력만 기록한다.
- 단조 시계 wall elapsed, 종료 상태와 실제 exit code(관측 불가이면 null), timeout
  발생 계층·예산, signal 요청과 관측 종료를 구분한 취소 결과, 도달/미도달 단계.
- 원본 artifact의 상대 경로·SHA256, 생성 자원의 회수 결과와 명시적 보존 경로.
  종료 뒤 manifest를 확정하고 과거 결과를 덮어쓰지 않는다.

wrapper는 성공뿐 아니라 nonzero·timeout·signal 경로에서도 상태와 elapsed를
먼저 확정하고 정리 결과를 덧붙인다. 정리 실패가 원래 실패를 덮어쓰지 않게 한다.
강제 종료로 종료 기록을 못 남긴 실행은 다음 관측자가 incomplete로 구분하고
알 수 없는 exit를 0으로 채우지 않는다. 기록 writer도 실패하면 그 실패를 노출한다.

Go JSON의 test/package 시간은 이벤트 의미 그대로 보존한다. 병렬 test 시간의
합이나 package 시간의 합을 profile wall time으로 쓰지 않는다. resource-slot
대기, gate wall time, PASS 이후 Go 처리, coverage와 후속 integration은 각 경계가
관측됐을 때만 별도로 기록하며 단순 차감으로 원인을 추정하지 않는다.

집계는 같은 profile·command·선택 범위·예산·runner/toolchain·source 입력 조건의
표본만 묶는다. dirty diff도 입력이므로 HEAD 일치만으로 합치지 않는다. 비교 가능한
표본이 20개 미만이면 n과 개별값·min/median만 제시한다. 20개 이상이면 정렬한
표본의 nearest-rank p95(ceil(0.95*n)번째)를 함께 기록한다. timeout은 완료 시간
표본에서 빼되 별도의 건수와 예산 하한으로 반드시 보인다. 서로 다른 기준·시점의
이번 전후 수치는 인과나 p95 근거가 아니다.

수동 검토는 timeout 발생, 동일 조건 3회 연속 stage wall time이 예산의 80% 이상,
또는 비교 가능한 기준보다 median이 20% 이상 증가했을 때 시작한다. 이 수치는
초기 검토 trigger 제안이며 자동 정책이 아니다. 담당자가 실패 계층과 누적 비용을
분리한 증거를 보고 판단하고, 예산 변경은 별도 소유 설정 변경과 리뷰를 거친다.

구현 수용 시 정상·nonzero·timeout·signal·정리 실패·기록 실패·불완전 종료의
각 경로를 synthetic 실행으로 검증해야 한다. 실제 선언 profile 한 회의 입력·
wall time·종료·원본 해시를 결속하고 기존 검사 선택·예산·exit 전달의 불변을
독립 검토한다. 이 설계의 문서 검토를 수집기 구현이나 전체 품질 PASS로 대신하지 않는다.
