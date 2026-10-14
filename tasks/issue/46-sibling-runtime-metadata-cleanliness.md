# ISSUE: sibling source 검사에서 로컬 agent 메타데이터를 dirty로 판정한다

- status: in progress
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

전체 품질 검사 첫 실행에서 parity 3개가 공식 provider 설치에 따른 진단 기준선
변경으로 실패했다. CE 기준 cd0e1d19 및 상태·Git·회수 assertion은 통과했고,
provider 경로·help 및 새 freshness/baseline/timing 진단만 달랐다. 가변 Duration을
정규화한 뒤 공식 build 96d5ecd로 해당 3개만 machine recapture하여 검증한다.
이 기록은 아직 실제 acquire 재검증의 성공 증거가 아니다.
