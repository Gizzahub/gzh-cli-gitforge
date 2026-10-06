# TASK-012 실행 증거

Baseline source는 `master`의 `68f4c101e5ffae3765d299ac64ef9da9ab27d5b6`다.
구현 branch는 `dev/codex/mbp/feat/issue-045-outcome-report`다. 로그와 종료
코드는 task worktree의 `tmp/issue-045/`에 보존하고 회수 전에 workbook의
ignored evidence로 복사한다. primary checkout은 그대로 유지했다.

| 검증                                                                                               | 실제 결과                                      |
| -------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| 실제 Make·local bare Git fixture 비교 12개와 parser/불완전 실행/env cleanup 회귀                   | 종료 0, `focused-fourth.log`                   |
| 최종 YAML parser 전체 `go test ./pkg/config`                                                       | 종료 0, 1.807s                                 |
| FIFO·symlink 교체, report parser와 비교 회귀                                                       | 종료 0, `nonblocking-report.log`, 0.313s       |
| `GOOS=windows GOARCH=amd64 go test ./pkg/integrate -c -o tmp/issue-045/integrate-windows.test.exe` | 종료 0, compile만 확인                         |
| 최종 integrate configured lint                                                                     | 종료 0, 오류 0건                               |
| `make quality-check`                                                                               | 종료 2, 기존 parity toolchain 불일치에서 중단  |
| `make test-integration-quality test-e2e-only coverage-critical-check`                              | 종료 0, `remaining-quality.log`                |
| source baseline `go test ./tests/parity -count=1`                                                  | 종료 1, 동일 toolchain 불일치                  |
| source baseline 및 task `ce task gate --json`                                                      | 종료 1, 양쪽 모두 기존 46개 card validate 실패 |

전체 품질 검사에서 format·lint·security·build 및 parity 외 unit package는
통과했다. `quality-check-precommit.log`의 `pkg/integrate` 전체 unit은 종료 0,
121.687s, coverage 78.2%였다. 게이트 종료 2는 기존 parity 불일치다.
별도 integration·E2E의 최종 재실행도 종료 0이며 critical coverage floor 네 개
모두 충족했다. 신규 config parser는 179/219 statements, 81.7%를 측정했다.
Windows의 환경변수 대소문자 처리도 두 가지 모드의 단위 회귀에서 종료 0이다.

Parity는 설치 CE build `529-gbe3ab29d`와 committed fixture build
`498-gf540f972`의 차이를 scenario 실행 전 거부한다. 관련 source·golden·pin은
변경하지 않았으며 원래 source에서도 동일 거부를 직접 확인했다. version
spoof, fixture 재작성이나 reference 생략으로 통과시키지 않는다. 새 카드의
개별 validate는 종료 0이며 board 전체의 기존 46개 실패를 늘리지 않았다.

작업 전용 formatter는 저장소가 지정한 mdformat 0.7.22·gfm 0.4.1·tables 1.0.0에
frontmatter plugin을 추가했다. 카드의 YAML 머리말을 보존하기 위한 검증 환경이며
전역 도구나 저장소의 제품 버전 정책을 바꾸지 않았다.

새 계약은 opt-in provider 기능이다. 소비자의 실제 검사 실행·결과 수집을
대신 만들지 않으며 원래 age-keygen 사건을 재현했다고 주장하지 않는다.
미선언 plain 실패의 legacy 동작은 유지한다.
