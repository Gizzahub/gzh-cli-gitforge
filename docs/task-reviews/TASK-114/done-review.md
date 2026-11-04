# TASK-114 완료 리뷰

- 대상: `dev/claude/mbp/feat/remove-check-reconcile-legacy-cleanup` 작업 트리
  (base origin/master `52ef273d`, 리뷰 시점 미커밋 65파일 +416/−5129)
- 구현: `impl-114` (Claude Sonnet 5.5, high) + 리드 게이트 패스
  (`json_strict.go` 고립 삭제, 카드 정정)
- 리뷰: `review-114` (Claude Opus, 구현과 별도 에이전트, 읽기 전용)
- 판정: `2026-11-04` PASS WITH MINORS — 커밋 전 차단 결함 없음

## 확인한 것

- `push_porcelain.go` 블록은 origin/master `run.go:469-603`과 바이트 단위로
  동일 (`push_porcelain.go:12-143`). `outcomeExecution` 헬퍼도 그대로 이동.
- `run.go`는 순수 삭제다: 156줄 삭제/2줄 추가, 577줄. 행동 변화는 카드에 명시된
  것뿐 — `FormatCheck` 헤더 `integrate run: `, `ErrInvalidExpectSource`/
  `ErrInvalidExpectTarget` 오류문이 `--source`/`--target`을 지칭(Go 식별자는
  불변), 프로토콜에서 `pre_mutation_outcome` 필드 제거(schema_version 1).
- 빌드 클린(`GOWORK=off go build ./...`), 신규 명명 테스트 6종과 재작성된
  계약/owner 테스트 5종 전부 통과. `cmd/`·`*.go`에 레거시 식별자·플래그 리터럴
  잔존 없음. `ErrEvidenceReplay`·공유 헬퍼·run 가드(이동 타깃 exit 4,
  non-fast-forward 거부) 보존.
- cleanup의 provenance 재배치는 실재한다: 해석→거부(exit 2)가 provenance
  출력보다 먼저, receipt와 동일 순서.
- `troubleshooting.md`는 예상 3개 헌크만 변경, 앵커 일관.
- 라이브 문서(README, docs/commands/\*)에 제거 표면 언급 없음.
- owner-문법 테스트 재작성은 원본보다 강하다(`errors.Is` +
  `ErrInvalidExecutionContext` + device 문법 + 거부값 검증).
- provenance 테스트의 부정 주장은 공허하지 않다(장치가 출력했다면 찍혔을 줄).

## 기록만 한 낮은 등급 소견

1. (카드 정정으로 해결) `execution_run_lock.go`는 `run.go`가 아니라
   `pre_mutation_outcome.go:106-121`에서 추출했고, doc comment는 삭제된
   pre-push-refusal 기계를 언급하던 근거 때문에 다시 썼다. 본문은 바이트 동일.
1. (카드 정정으로 해결) `TestSameExecutionRefusesWhileAnotherRunOwnsIt`는
   `TestSameExecutionCannotRecordNegativeWhileAnotherRunOwnsIt`
   (`pre_mutation_outcome_test.go:135`)의 생존 반쪽을 개명한 것이다(삭제된
   outcome-read 주장만 제거).
1. `validateOwner`(device.go:51)의 프로덕션 호출자가 0이 됐다(원래 유일 호출자는
   삭제된 `legacy_cleanup_plan.go`). `receipt.go:127-139`의 인라인 검증과
   중복. 후속 카드 후보 — 헬퍼+테스트 제거 또는
   `ExecutionContext.Validate`로 통합.
1. `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md:371,435`는 check-then-run을
   현행 포팅 계약처럼 서술한다. 내부적으로 CE 0.10.24 `fd175fd2`에 핀되어
   시점 일관성은 있지만, 오늘 읽는 구현자는 제거된 하위명령 호출을 만들 수 있다.
   parity golden 캡처(finish-explicit, finish-noarg-single-active, doctor-active,
   discard-no-fetch)도 같은 순열을 심는다(재생에 핀된 CE가 필요해 이 호스트연
   불가). 후속 카드 후보 — 일자 명시 슈퍼세션 주석.
1. 레거시 철자가 미확인 cobra 플래그로 실패함을 주장하는 명시적 테스트가 없다
   (구 묵시 배제 테스트 삭제). grep 바인딩 + cobra 기본 동작이 암묵 커버.
   선택적 후속.
1. `TestIntegrateRecordRefusalExitsTwoBeforeTouchingEvidence`와
   `TestIntegrateHandlerRefusesIncompleteRecord`가 근사 중복(전자가 유닛
   리졸버까지 커버). 수용.

## 리뷰어가 확인하지 못한 것(리드가 별도 확인)

- ce-agent-kit 전제 조건(ce v0.11.0-5-g5756e6f7c, TASK-518/519 컷오버,
  zero-need 증명) — 카드 전제 기재, 리드가 ce 버전 출력으로 확인.
- `make verify-full` 최종 결과 — 리드의 3차 권위 게이트로 기록
  (execution-evidence.md).
