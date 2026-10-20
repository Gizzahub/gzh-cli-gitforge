# CE parity 참조 f66d06c1 채택 기록

## 범위와 후보

설치 CE가 `6baa7f9e` 채택 직후 clean `f66d06c1`
(`f66d06c117ad279cc1df2c60da09fcf416970809`, Build `370-gf66d06c1`,
CE 0.10.14, `Dirty: false`)로 바뀌어 parity golden 12개가 모두 CE identity
불일치로 실패했다. 사용자는 이 설치본 기준 재녹화 계획을 승인했다(2026-10-20).
시나리오, normalizer, gz-git 제품 구현, workstation 도구 pin은 바꾸지 않는다.

## 변경

- `tests/parity/harness_test.go`: `pinnedCECommit`을 `f66d06c1`로 옮긴다.
- 12개 golden을 `PARITY_RECORD=1`로 재녹화했다(14.7초). 37줄 변경은 모두 CE
  identity(reference, toolchain `ce`, receipt `tool_version`)다.
- 계약 문서의 현재 참조, `ce` toolchain 행, §7 채택 문단을 갱신했다.

## 검증

- 차이 검토: HEAD와 재녹화본 양쪽에서 full revision, build stamp, CE 버전을
  같은 토큰으로 치환한 뒤 12개 파일을 비교했다. 12개 모두 바이트 동일했다.
  상태·종료 코드·권한·ref·push·cleanup 결과는 바뀌지 않았다.
- 별도 non-record `GOWORK=off go test -count=1 -v ./tests/parity/` 성공(13.2초).
  `TestParityReferencePinsMatchCapture`가 golden·계약 문서·pin 일치를 확인한다.
- 원시 로그는 task worktree의 `tmp/i41-record.log`, `tmp/i41-verify.log`다.
- 계약 문서는 이 변경 전부터 파일 크기 검사 한도를 넘었다. 분할은 범위 밖이다.

## 전달 경계

이 기록은 집중 검증이며 최종 전달 판정이 아니다. 설치 CE가 다시 바뀌면 CE
identity 차이로 재녹화가 필요하다.
