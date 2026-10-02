# ISSUE: 호스트 ce 바이너리 드리프트로 CE parity 골든 검증이 전면 실패한다

- status: closed (2026-10-02 — 제안 2)대로 `PARITY_RECORD=1` 재녹화로 해결; 아래 해결 기록)
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
