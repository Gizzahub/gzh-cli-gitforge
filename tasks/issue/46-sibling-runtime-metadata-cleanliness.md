# ISSUE: sibling source 검사에서 로컬 agent 메타데이터를 dirty로 판정한다

- status: resolved (2026-10-14; source `b323` native lifecycle proof)
- priority: P1
- category: integration/provisioning
- created_at: 2026-10-14
- owner: `pkg/integrate/sibling_provision.go`

공식 설치한 source `96d5ecd`로 실제 EE 작업 tree에서 sibling acquire를 실행하면
`gitrump-ce checkout is dirty`로 거부한다. CE primary `51df413`은 일반 Git status가
clean이고 HEAD와 origin/master가 일치하지만, global ignore를 배제한 provider의
검사에는 root `.ce/task-runtime.yaml`, `.agent-mesh/checkpoints/*.checkpoint.json`,
nested `.ce/audit` 및 `.ce/heartbeat` 메타데이터가 untracked로 나타난다.

수선은 committed archive에 포함되지 않는 관측된 runtime 메타데이터만 허용한다.
tracked/staged 변경, 알 수 없는 untracked 파일, 유사 경로와 malformed porcelain은
계속 거부하고 기존 flow snapshot의 허용 목록은 넓히지 않는다. 실제 acquire,
archive 배제, inspect/resume/release 증거를 확인한 뒤 해결 상태를 기록한다.

## 해결 및 실제 증거

공식 설치 source `b323`의 Go build metadata는 clean revision `b323`을 보고한다.
그 source로 실제 EE 작업 tree에서 실행한 native lifecycle은 CE
`51df4133bffda8909edf2136eca4bc9f364270bf`에 대해 다음 결과를 냈다.

- `acquire`는 lease `d2da53bb51ca437dbca137e888ec40cb`를 `ACQUIRED`로 만들었다.
- `inspect`와 이어진 상태 확인은 모두 `RESUMABLE`을 보고했다. 이는 inspect가 복구
  가능 상태를 보이는 것이며 별도의 resume CLI를 뜻하지 않는다.
- `release`는 같은 lease를 `RELEASED`로 회수했다.

증거는 각각
`/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/ee-native-sibling-{acquire,inspect,resume,release}.json`에
있다. 이로써 로컬 agent runtime 메타데이터를 source dirty로 오인하던 이슈는 실제
acquire부터 release까지의 native 경로에서 해소되었다.

별도 재현에서 만들어진 lease `0f880cd8face0842b90f350f2cfd2fb6`는 선언된 CE source의
정상 archive였다. 첫 EE coherent compile에서 재사용·복구 가능 상태로 확인한 뒤,
HTTP audit private-setting retention gap을 드러낸 경계 전에 clean release했다.
`ee-native-sibling-pre-audit-release.json`은 그 lease의 `RELEASED` 결과를 기록한다.
선언되지 않은 `--source` 이름 처리 결함은 ISSUE-49에서 추적한다.
