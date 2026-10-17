# Readiness 승인 모드 구현·검증 기록

## 범위와 결정

글로벌 `integration.readinessApproval`만 수동·자동을 선택한다. 기본값은
`manual`이다. 수동은 전체 diff와 한 번의 TTY 승인, 자동은 정확한 source SHA와
CLI 요청 문장 `테스트완료` 및 실제 candidate runner의 성공 결과를 요구한다.
대화 감시는 사용하지 않는다. 문장은 증거가 아니라 실행 요청이다.
자동 모드는 candidate runner 자체를 신뢰하므로 검사 완전성을 보증하지 않는다.
구현 standard, 통합 설계·독립 검토 strong으로 수행했다.

## 확인된 결과

- 후보 `67232de27985a0250b53d64393d19441e60f2837`에서 `make build` 성공.
- 빌드한 CLI를 네 개의 격리된 로컬 bare 원격에 실행했다. 수동 거절 시 원격
  불변, 수동 승인 시 정확한 commit 적용, 자동 runner의 문장 출력만으로는
  거절, 유효한 ready JSON과 성공 종료 시 적용을 모두 확인했다.
- 재현: primary의 `tmp/readiness-approval-smoke.py`에
  `READINESS_SMOKE_BIN=<task-worktree>/gz-git`을 지정해
  `uv run --no-project python <script>` 실행. 합성 저장소는 소유 metadata와
  자식 목록 확인 후 해당 실행이 만든 경로만 정리했다.
- source/target pair·실행 cwd·정책 철회·SHA 변경·timeout 회귀를 포함한
  범위별 Go 검사는 통과했다. 독립 검토에서 candidate 실행 cwd 지적을
  수정하고 재검토를 받았다.
- native 전체 검사에서 새 policy 명령의 read-only 테스트 등록 누락을
  발견해 수정했다. 이후
  `go test ./cmd/gz-git/cmd -run 'TestReadOnly(RunsCoverDeclarations|CommandsWriteNothing)' -count=1 -timeout 2m`
  은 성공했다.
- devenv hook·launcher 검사 213개 통과, 별도 검토자의 검사 79개 통과.

## 전달 상태와 한계

전체 native gate는 통과하지 않았다. CE parity 참조 불일치와 설치된 구버전
provider의 15분 전체 제한 때문에 기준선 비교를 완료하지 못했다.
구체적 revision·진단 경로는 [ISSUE-41](../tasks/issue/41-ce-parity-golden-drift.md)에
기록했다. 관련 `pkg/integrate` 전체 단위 검사는 796.081초에 통과했지만,
이를 전체 게이트 성공으로 해석하지 않는다.

`dva ci commit`은 문서 변경 때문에 full profile로 확대하도록 거절했다.
source/master 통합·전역 설치·자동 모드 활성화는 하지 않았다.
두 저장소의 task branch와 worktree는 재개를 위해 유지한다.
