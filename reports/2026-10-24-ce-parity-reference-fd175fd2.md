# CE parity 참조 fd175fd2 채택 기록

## 범위와 후보

설치 CE가 clean `fd175fd2`(`fd175fd2b73dc457de8fa284dd93fc0361af07d8`, Build
`527-gfd175fd2b`, CE 0.10.24, `vcs.modified=false`)로 바뀌어 참조 모드가 CE
identity 불일치로 실패했다. ISSUE-42 전체 parity 포트의 기준을 현재 설치본에
맞추기 위해 재고정한다(2026-10-24 사용자 승인). `c6d5bf84..fd175fd2` 구간의
비테스트 변경에는 `taskruntime`(finish judgment, settle, sibling provisioning,
run eligibility)과 quality·config·access 변경이 있다. 시나리오, normalizer,
workstation 도구 pin은 바꾸지 않는다.

## 변경

- `tests/parity/harness_test.go`: `pinnedCECommit`을 `fd175fd2`로 옮긴다.
- 12개 golden을 `PARITY_RECORD=1`로 재녹화했다.
- 계약 문서의 현재 참조, `ce` toolchain 행, §7 채택 문단을 갱신하고 이전
  `c6d5bf84` 문단을 `RUN_LIFECYCLE_PARITY_ADOPTIONS.md`로 옮겼다.

## 검증

- 차이 검토: 중첩 JSON stdout까지 파싱하고 CE identity(40자 SHA, build,
  버전)를 정규화하면 10개 파일은 동일하다.
- 나머지 2개(discard-no-fetch, doctor-active)는 doctor가 캡처한
  `integrate --help`·`integrate run --help` 문구 3개만 다르다. 퇴역한
  lock-wait 문구("waits for it until the lock is free")가 빠졌다. 이것으로
  ISSUE-93의 남은 골든 갱신이 끝난다. 종료 코드·상태·권한·ref·push·cleanup
  필드는 바뀌지 않았다.
- 별도 non-record `GOWORK=off go test -tags=parity -count=1 ./tests/parity/...`
  PASS.

## 전달 경계

이 기록은 참조 채택이며 포트 parity 판정이 아니다. 이 구간의 `taskruntime`
변경은 12개 시나리오가 닿지 않으므로 이 캡처는 그 동작을 증명하지 않는다.
포트 모드 차이는 ISSUE-42에서 다룬다.
