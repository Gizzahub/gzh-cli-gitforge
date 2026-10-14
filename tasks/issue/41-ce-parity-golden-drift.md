# ISSUE: 호스트 ce 바이너리 드리프트로 CE parity 골든 검증이 전면 실패한다

- status: open (2026-10-14 재발: cd0e1d19 golden / b79c1a4a 설치본 불일치)
- previous-status: closed (2026-10-02 — 제안 2)대로 `PARITY_RECORD=1` 재녹화로 해결; 아래 해결 기록)
- priority: P1
- category: quality/parity
- created_at: 2026-10-01
- owner: `gzh-cli-gitforge/tests/parity/testdata/golden`
- work-card: 없음 — 신규 발견, 재녹화는 메인테이너 결정

## 증상과 원인

parity 스위트는 PATH의 `ce`를 실행해 커밋된 골든(`pinnedCECommit = 771c54cf`, 빌드 `404-g771c54cf`)과 diff한다. 호스트 ce-agent-kit이
`475-g950650ef`로 올라가면서 run-status의 `nextAction` 문구가 길어지는 등
출력이 바뀌어 TestCEParity 서브테스트 11개가 실패한다:

abort-terminal-noop, discard-no-fetch, discard-refusals, doctor-active,
finish-explicit, finish-noarg-single-active, finish-refusal-unpushed,
owner-mismatch-takeover, reclaimed-run-reconciliation,
recover-requires-cleanup-failure, start-created-existing

`make quality-check`의 `test-unit-quality`가 이 스위트를 포함하므로 ce가
드리프트된 머신에서는 게이트 전체가 실패한다.

## 격리 근거 (2026-10-01)

- master(primary 체크아웃 `74a3869`)에서 동일 11개 서브테스트 실패 —
  TASK-260 변경(make 예산)과 무관. diff 내용이 전부 ce 버전 메타데이터와
  ce 출력 문구다.
- 스위트 주석의 선언된 대응은 "toolchain drift → re-recording"이며, 골든
  재녹화는 참조 ce를 새 버전으로 인정하는 의사결정이므로 메인테이너가
  한다. 본 카드는 재녹화하지 않았다.

## 해결 기록 (2026-10-02)

메인테이너 결정으로 제안 2)를 택해 골든을 `475-g950650ef` 기준으로
재녹화했다 (`PARITY_RECORD=1 go test ./tests/parity/...`, 11개 시나리오).

- 이동 성격: stamp-only가 아니었다. word-diff 검증 결과 변경은 ① 버전
  스탬프(771c54cf/404-g → 950650ef/475-g), ② run-status `nextAction`
  가이드 문구 확장, ③ 캡처된 `gz-git integrate --help` 출력의 신규
  `Effect: mutating (...)` 라인 — 전부 캡처된 출력 문구며,
  스키마/상태/allowedActions는 불변.
- 수반 변경: `pinnedCECommit` 상수 → `950650ef`,
  `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md` 핀·이력 갱신(파일 전체가
  mdformat 정규화됨 — 변경 파일은 전체 검사 대상이라 필연).
- 근거: 재녹화 후 로컬 `make quality` 전체 통과(parity 포함). hosted CI
  parity 단계는 푸시/통합 시점에 확인한다.
- 후속: ce 드리프트 시 스위트가 실패 전단계에서 재녹화 안내와 함께
  실패하는 가드(parity-version-guard)를 별도 과제로 진행한다.

## 재발 관측 (2026-10-14)

DVA 최소 연결의 source 기준 `9e054cf4daae3b52fb55465741fe57815ad8e9f9`에서
참조 golden은 `cd0e1d19c3211b85a514ac03383ae6166a614f07`, 실제 PATH의 clean CE는
`b79c1a4a9ab9e53af55074a816d15a3ece9c1704`였다. 과거 재녹화 해결과 리뷰는
위 기록대로 보존하며, 새 issue를 중복 등록하지 않고 이번 재발만 추가한다.

source primary에서 다음 명령은 exit 1, golden guard에서 조기 실패했다.
DVA 연결은 Makefile·Go 소스·golden·CE pin을 변경하지 않았다.

```bash
env -u CE_BIN -u PARITY_TARGET -u GZ_GIT_BIN -u PARITY_RECORD \
  go test -short -count=1 ./tests/parity
```

`source-parity-baseline.log`는 설치 build와 12개 golden의 차이를 기록한다.
원시 증거는 소유 devbox `gzh-cli-devbox`의
`tmp/dva-project-tooling-evidence-20261014/gitforge/`에 보존한다.
이 등록은 재녹화·CE 설치·구버전 강제·fixture 예산 재설정 승인이 아니다.
참조 채택과 의미 차이는 기존 소유자가 독립 strong 리뷰로 판정해야 하며
전체 포트 추종 범위는 ISSUE-42와 분리한다.

## 2026-10-14 sibling lifecycle 문서 전달 중 재확인

설치 CE가 `c4deec428559ea49a9b5a066ddfe727045b00a37`로 전진한 뒤,
`sibling-provision-evidence` 작업의 `make quality-check`는 앞선 unit 검사를
통과하고 parity TestMain에서 exit 2로 중단했다. 12개 golden은 계속
`cd0e1d19c3211b85a514ac03383ae6166a614f07`를 참조한다. 이후 integration,
E2E, coverage 단계는 실행하지 않았다. `make check`는 통과했다.

canonical source primary `b32360a`의 직접 `go test ./tests/parity`도 같은
installed-build guard에서 exit 1로 실패했다. primary, 현재 target
`bf460da`, 문서 작업 `5599474`의 `tests/parity` Git tree object는 모두
`675f90fb8c04470b3a4aa84886b2cd73d7a8ba16`이며, 해당 parity package의 Go 소스와
fixture는 같다. `bf460da..5599474`의 작업 변경은 문서 3개뿐이며 Go・test・tool
경로를 바꾸지 않았다. 실패는 scenario 실행 전에 동일한 TestMain guard에서
발생하므로, 이 문서 작업이 baseline 실패를 악화하지 않았음을 독립 strong AI
검수로 확인했다.

원시 로그는 `/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/`의
`sibling-evidence-rebase-quality.log`와 `sibling-evidence-baseline-parity.log`다.
이 기록은 기존 ISSUE-41의 재현 갱신이며, 새 CE pin 채택이나 golden 재녹화,
rollback, waiver를 집행한 기록이 아니다.
