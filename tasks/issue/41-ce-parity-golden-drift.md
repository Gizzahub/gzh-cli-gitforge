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

## 계약 확인과 새 후속 범위 (2026-10-14)

ce-devenv의 `config/agents/agent-kit.toml`, 배포된 CE locator와 live snapshot의
동일 파일은 같은 소스를 가리킨다. 이는 소스 경로 일치이며 immutable revision
pin의 일치를 뜻하지 않는다. authoring source HEAD는 `9012626c`, clean 실행
바이너리는 `b79c1a4a`다. 실행 build metadata는 `vcs.modified=false`다.
따라서 source HEAD를 그대로 설치하거나 과거 cd0 바이너리로 강제하지 않는다.

`cd0e1d19`는 b79의 조상이며 그 사이 네 커밋의 net diff에 task-runtime endpoint
변경은 없다. b79 이후 현재 source까지도 해당 경로 변경은 관측되지 않았다.
그러나 12개 시나리오의 실제 출력 비교는 아직 없으므로 stamp-only 또는 의미
동등성을 확정하지 않는다. 원시 locator·binary·Git 관측은 devbox의
`tmp/dva-project-tooling-evidence-20261014/followup/source-runtime-context.log`에
보존한다. 이전 guard 실패·해결·리뷰 기록은 변경하지 않는다.

### 제안한 단일 후보 예산 — 아직 활성화하지 않음

TASK-318/321의 소진 예산을 재사용하거나 초기화하지 않는다. ISSUE-41에 연결된
별도 후속 실행을 선택할 때만 다음 신규 예산을 적용하도록 제안한다.

| 항목                     | 상한                                       |
| ------------------------ | ------------------------------------------ |
| 신규 참조 후보           | clean resolved b79 한 개                   |
| 기계 캡처                | 12개 CE-reference 시나리오 전체 한 번      |
| 독립 리뷰                | strong 한 번; 구현도 strong                |
| record 실행              | 10분                                       |
| focused reference 재검증 | 10분, zero skip                            |
| canonical quality        | 45분; 기존 package timeout은 변경하지 않음 |
| 독립 리뷰 시간           | 30분                                       |

실행·리뷰 합계 상한은 95분이며 기존 FIFO 자원 대기와 Git 통합 대기는 별도로
관측한다. 재시도·두 번째 후보·자동 시간 연장은 없다. 읽기 전용 preflight는
후보를 소비하지 않지만 실패하면 진행을 멈춘다. 새 참조로 record가 처음 시작해
proposed fixture를 쓰는 순간 한 후보·한 캡처를 소비한다. isolated evidence
copy도 이 횟수에 포함하며, 임시 복사본을 추가 후보나 authoring source로 쓰지 않는다.

활성화 전 source freshness·ACTIVE 소유권·resolved CE의 SHA와 clean build를 다시
확인한다. 허용 변경은 `tests/parity/harness_test.go`의 참조 pin, 기존 golden
12개, `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md`, 이 이슈의 새 증거뿐이다.
엔진·native port·설치 pin·devenv·gz-git 버전은 제외한다.

캡처 후 모든 변경을 시나리오·구조 필드별로 분류한다. status·exit·reason/action
의미·owner·source/ref·worktree·cleanup의 변화, 미분류 차이, skip, timeout,
nonzero 또는 provenance 불일치가 있으면 후보를 거절한다. 캡처와 독립 리뷰를
통과하기 전 pin/golden을 수용하지 않는다. reference 후속은 ISSUE-42의 native
port 수용을 대신하지 않으며, outer quality의 nonzero를 helper 성공으로 상쇄하지 않는다.

이번 변경은 계획·관측 기록만 전달한다. 새 예산 채택·fixture 실행·재녹화·설치·
릴리스 또는 TASK-302 수용을 실행하지 않았다.
