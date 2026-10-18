# Readiness 승인 모드 구현·검증 기록

## 최종 전달 완료 (2026-10-18)

아래 재개·최초 기록은 당시 관측이다. 이후 native 전체 게이트를 통과해
수동·자동 승인 모드를 통합하고 설치했다. 자동 정책은 활성화하지 않았으며
설치본의 유효 정책은 기본값 `manual`이다.

- gitforge: `master`의 `1db93c1b6b1cef49586fe53645a60b7ef5e377e2`까지
  원격 푸시·설치 완료. native 전체 검증은 945,153ms에 통과했다.
- devenv: `master`의 `1ecf454ae8737fcb94a311afa7abb02eafd147b0`까지
  원격 푸시·설치 완료. 최종 native 검증은 218,485ms에 통과했다.
  기존 iTerm2 CurrentDir 검사 실패는 기준선 대비 비악화로 판정됐으며,
  모든 개별 검사가 성공했다는 뜻은 아니다. 소유 이슈는 devenv `ISSUE-029`다.
- 두 delivery의 receipt는 `DONE`, `sourcePushed: true`이며 worktree,
  로컬·원격 task branch 제거를 모두 확인했다. 이전 유지 안내는 종료됐다.
- 설치된 CLI로 수동 거절·승인, 자동의 문장 출력만으로는 거절,
  유효한 ready JSON과 성공 종료 시 적용의 네 합성 시나리오를 확인했다.
  설치된 launcher도 `manual` 정책에서 자동 요청을 exit 2로 거부했다.
- 설치 전후 선택된 모델·provider·reasoning 설정과 Docker 설정 바이트·0600
  권한을 비교해 보존을 확인했다.

원시 native receipt는 각 primary checkout의
`tmp/readiness-approval-finish-1db93c1.json`(gitforge)과
`tmp/readiness-feature-finish-1ecf454a.json`(devenv)에 있다. 설치 로그는
각각 `tmp/readiness-gitforge-install.log`, `tmp/readiness-final-install.log`다.
이 경로는 실행 호스트의 보조 증거이며 배포 파일이나 재사용 승인서는 아니다.

## 후속 재개 (2026-10-18)

CE 참조 불일치는 clean source/executable `ad92901e`를 채택하고 열두 시나리오
기계 캡처 및 별도 full non-record 검증으로 해결했다. 독립 strong 리뷰는
task-containment 성공 진단 추가를 확인했고 normalizer는 변경하지 않았다.

아래 최초 기록의 구버전 provider 15분 제한 원인 설명은 정정한다. 실제
protocol은 `legacy-make`이고, 이 경로는 선언된 `makeTimeout: 67m`을 사용한다.
typed make timeout 증거는 없어 당시 중단의 원인을 확정할 수 없다.
자세한 정정은 ISSUE-41에 기록한다. 전체 native 게이트는 재시도 전이며,
아래 최초 전달 상태를 전체 성공으로 바꾸지 않는다.

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
