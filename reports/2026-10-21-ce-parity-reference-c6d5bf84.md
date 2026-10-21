# CE parity 참조 c6d5bf84 채택 기록

## 범위와 후보

설치 CE가 clean `c6d5bf84`(`c6d5bf842d6adae7bd93327ee0fa0d13d49ca584`, Build
`412-gc6d5bf84`, CE 0.10.14, `vcs.modified=false`)로 바뀌어 parity golden이 CE
identity 불일치로 실패했다. `f88f4331..c6d5bf84` 구간의 `cmd`·`internal`·`pkg`
변경은 대부분 `_test.go`다. 비테스트 변경은 `access.go`, `access_index.go`,
`access_render.go`, `access_target.go`, `task_cleanup.go`, `task_lint.go`뿐이고,
병렬 테스트를 위해 작업 디렉터리와 전역 인덱스를 주입하는 리팩터다. 캡처되는
run 생명주기 경로에는 없다. `go.mod` 변경은 없다. 최신 설치본 기준 재고정
방침(2026-10-21 사용자 승인)을 따른다. 같은 브랜치의 readiness 계약 제거로
바뀐 gz-git 출력도 이 재녹화에 함께 담긴다. 시나리오, normalizer,
workstation 도구 pin은 바꾸지 않는다.

## 변경

- `tests/parity/harness_test.go`: `pinnedCECommit`을 `c6d5bf84`로 옮긴다.
- 12개 golden을 `PARITY_RECORD=1`로 재녹화했다.
- 계약 문서의 현재 참조, `ce` toolchain 행, §7 채택 문단을 갱신했다.

## 검증

- 차이 검토: CE identity(40자 SHA, `<n>-g<sha8>` build)를 정규화하면 10개
  파일은 바이트 동일하다.
- 나머지 2개(discard-no-fetch, doctor-active)는 중첩 JSON stdout까지 파싱해
  `--help` 진단 5개의 stdout만 가리면 바이트 동일하다. 바뀐 것은 doctor가
  캡처한 `integrate --help`·`integrate check --help`·`integrate run --help`
  문구다: readiness 계약·`HOST_SLOTS_HELD` 설명, `bootstrap`·`readiness` 하위
  명령, effect의 `arbitrary`, `--lock-wait` 플래그 제거. 종료 코드·상태·권한·
  ref·push·cleanup 필드는 바뀌지 않았다.
- 별도 non-record `GOWORK=off go test -count=1 ./tests/parity/...` PASS.

## 전달 경계

이 기록은 집중 검증이며 최종 전달 판정이 아니다. 설치 CE가 바뀔 때마다 기본
게이트가 실패하는 구조 자체는 별도 작업(parity를 build tag와
`make test-parity`로 분리)으로 다룬다.
