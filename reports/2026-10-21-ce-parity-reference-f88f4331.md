# CE parity 참조 f88f4331 채택 기록

## 범위와 후보

설치 CE가 clean `f88f4331`(`f88f4331f2c09858778a747589a776bcb477346d`, Build
`403-gf88f4331`, CE 0.10.14, `vcs.modified=false`)로 바뀌어 parity golden 12개가
CE identity 불일치로 실패했다. CE master `a213fe3b`는 이보다 3커밋 앞서지만
`cmd`·`internal`·`pkg`·`go.mod` 변경이 없어 코드 기준 최신이다. 사용자는 최신
설치본 기준 재고정을 승인했다(2026-10-21). 같은 브랜치의 legacy make 게이트
제거로 바뀐 gz-git 출력도 이 재녹화에 함께 담긴다. 시나리오, normalizer,
workstation 도구 pin은 바꾸지 않는다.

## 변경

- `tests/parity/harness_test.go`: `pinnedCECommit`을 `f88f4331`로 옮긴다.
- 12개 golden을 `PARITY_RECORD=1`로 재녹화했다(14.7초).
- 계약 문서의 현재 참조, `ce` toolchain 행, §7 채택 문단을 갱신했다.

## 검증

- 차이 검토: CE identity(reference, toolchain `ce`, receipt `tool_version`)를
  제외하면 6개 파일(abort-terminal-noop, finish-noarg-ambiguous-active,
  finish-refusal-unpushed, reclaimed-run-reconciliation,
  recover-requires-cleanup-failure, start-created-existing)은 차이가 없다.
- 나머지 6개(discard-no-fetch, discard-refusals, doctor-active, finish-explicit,
  finish-noarg-single-active, owner-mismatch-takeover)는 `stdout` 필드만 바뀐다:
  readiness 행 `legacy-make` → `none`, `make check`/`make lint`/baseline 행 제거,
  WARN 1 → 0, `integrate --help` 게이트 설명. 종료 코드·상태·권한·ref·push·cleanup
  필드는 바뀌지 않았다.
- record 실행은 같은 프로세스에서 이전 golden을 읽은 `provider_capture_test`
  때문에 exit 1로 끝났다. 재녹화본 기준 별도 non-record
  `GOWORK=off go test -count=1 -v ./tests/parity/...` 성공(13.8초).
- 원시 로그는 task worktree의 `tmp/repin-record.log`, `tmp/repin-verify.log`다.

## 전달 경계

이 기록은 집중 검증이며 최종 전달 판정이 아니다. 설치 CE가 다시 바뀌면 CE
identity 차이로 재녹화가 필요하다.
