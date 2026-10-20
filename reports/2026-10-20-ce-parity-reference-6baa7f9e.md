# CE parity 참조 6baa7f9e 채택 기록

## 범위와 후보

사용자는 ISSUE-41 해결안 A를 승인했다(2026-10-20): gz-git 버전을 마스킹하는
normalizer를 추가하고 12개 golden을 재녹화한다. 승인 시점 분석은 설치 CE를
`61d1c0dd`로 보았다. 작업 시점 설치본은 clean `6baa7f9e`
(`6baa7f9e9f89370052320b047c116587ffff76b8`, Build `345-g6baa7f9e`,
`Dirty: false`)였고, 이 기록은 그 설치본을 참조로 채택한다. 시나리오, gz-git
제품 구현, workstation 도구 pin은 바꾸지 않는다.

## 드리프트 원인

CE의 integrate run은 이제 stderr에 `integrate: run device=<host> gz-git=<version>`을
출력한다. sandbox는 호스트의 gz-git을 링크하므로, gz-git 릴리스가 바뀔 때마다
finish 계열 golden이 깨졌다. device는 harness가 `GIT_WORK_HOST=mbp`로 고정하므로
정규화하지 않는다.

## 변경

- `tests/parity/harness_test.go`: `gzGitVersionRe`(`\bgz-git=[^\s\\"]+`)를 추가하고
  `norm()`에서 `gz-git=<VER>`로 치환한다. `pinnedCECommit`을 `6baa7f9e`로 옮긴다.
- 12개 golden을 `PARITY_RECORD=1`로 재녹화했다. 차이는 CE identity(reference,
  toolchain, receipt `tool_version`)와 `finish-explicit`,
  `finish-noarg-single-active`의 새 stderr 줄(마스킹된 형태)뿐이다. 상태·종료
  코드·권한·ref·push·cleanup은 동일하다.
- 계약 문서의 현재 참조, `ce` toolchain 행, §7 채택 문단을 갱신했다.

## 검증

- 별도 non-record `GOWORK=off go test -count=1 ./tests/parity/` 성공(15.6초).
  `TestParityReferencePinsMatchCapture`가 golden·계약 문서·pin 일치를 확인한다.
- 원시 로그는 task worktree의 `tmp/i41-verify.log`다.
- 계약 문서는 이 변경 전부터 파일 크기 검사 한도(46 KiB·500 prose 줄)를 넘었다.
  분할은 이번 범위 밖이다.

## 전달 경계

이 기록은 집중 검증이며 최종 전달 판정이 아니다. native `run-finish` 영수증이
판정한다. 설치 CE가 다시 바뀌면 CE identity 차이로 재녹화가 필요하다. 그러나
gz-git 릴리스만으로는 더 이상 golden이 무효화되지 않는다.
