# TASK-114 실행 증거

작업 트리 `dev/claude/mbp/feat/remove-check-reconcile-legacy-cleanup`
(base origin/master `52ef273d`). 모든 go/make 명령은 `GOWORK=off`.

## 완료 기준

| 기준                        | 검증                                                                                                                                                                                         | 실행자    | 결과                |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- | ------------------- |
| 1 하위명령 파일 부재        | `! ls cmd/gz-git/cmd/integrate_{check,reconcile,legacy_cleanup*}.go`                                                                                                                         | 리드      | rc 1 (전 경로 부재) |
| 2 레거시 플래그 잔존 없음   | `! grep -rn '"expect-source"\|...' cmd/gz-git/cmd/`                                                                                                                                          | 구현+리드 | rc 1 (매치 없음)    |
| 3 pre_mutation_outcome 부재 | `! grep -rn pre_mutation_outcome pkg/ cmd/`                                                                                                                                                  | 구현+리드 | rc 1 (매치 없음)    |
| 4 run 가드 생존             | `go test ./cmd/gz-git/cmd -run 'TestIntegrateRunPinnedTargetThatMovedExitsFourWithoutPushing\|TestIntegrateRunNonFastForwardSourceIsRefusedWithoutPushing' -count=1`                         | 리드      | PASS 1.21s / 0.97s  |
| 5 cleanup 해석 선행 거부    | `go test ./cmd/gz-git/cmd -run TestIntegrateCleanupRefusesMalformedRecordWithoutProvenance -count=1`                                                                                         | 리드      | ok 0.400s           |
| 6 핸들러 거부 3종           | `go test ./cmd/gz-git/cmd -run 'TestIntegrateRecordRefusalExitsTwoBeforeTouchingEvidence\|TestIntegrateHandlerRefusesMalformedRecord\|TestIntegrateHandlerRefusesIncompleteRecord' -count=1` | 리드      | ok 0.326s           |
| 7 커밋 게이트               | `VERIFY_BASE=origin/master make verify-commit` → widening → `make verify-full`                                                                                                               | 리드      | 아래 게이트 이력    |

기준 4의 바인딩은 리드가 정정했다: 카드 최초 기재는 `./pkg/integrate/`를
가리켰으나 해당 패키지에 패턴 매치가 없어(`[no tests to run]`도 exit 0)
공허했다. 두 테스트는 `cmd/gz-git/cmd/integrate_run_args_test.go:191,212`에
있고(origin/master에도 pkg/에는 없다), 올바른 패키지에서 통과했다.

## 게이트 이력 (`VERIFY_BASE=origin/master make verify-full`)

| 실행 | 트리                                                | 결과                                                                                                                                                                  |
| ---- | --------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1    | 구현 완료 + 기준 4-6 틱 직후                        | exit 2 — `format-md-check`: 카드 자체 틱 줄 포맷. `make fmt`로 해결                                                                                                   |
| 2    | fmt 후                                              | exit 2 — `lint-check`: golangci-lint `unused` 2건(`json_strict.go` 전체가 고립; 유일 소비자는 삭제한 `legacy_reconcile.go:233,300`). 파일 삭제 + 빌드 재검증으로 해결 |
| 3    | json_strict 삭제 후 코드 확정 트리                  | exit 0 — 전체 검증 프로파일 통과. e2e 26.888s, 임계 커버리지 플로어 4/4 (gitcmd 72.5%/59.1%, repository 59.8%/59.3%, reposync 61.0%/61.0%, workspacecli 69.4%/63.9%)  |
| 4    | 리뷰 정정 카드 + done-review.md 포함 트리           | exit 0 — 전체 검증 프로파일 통과                                                                                                                                      |
| 5    | 카드 done 확정 + 본 증거 파일 포함 커밋 트리 (권위) | exit 0 — 전체 검증 프로파일 통과. 게이트-커밋 사이 트리 변경 없음                                                                                                     |

## 기타 검증

- `GOWORK=off go build ./...` → 클린 (json_strict.go 삭제 직후 리드 재실행,
  리뷰 에이전트도 독립 실행).
- 전체 패키지 스위트(구현 단계): `ok cmd/gz-git/cmd 86.342s`,
  `ok pkg/integrate 344.507s`.
- 독립 리뷰(opus, 읽기 전용): PASS WITH MINORS — 판정·발견·확인 목록은
  done-review.md. 리뷰어는 json_strict.go 삭제를 자체 확인했고, 역방향 고립
  스윕은 run 3 lint-check가 권위라고 명시했다.

## 전제 조건 (카드 기재, 리드 확인)

- ce `v0.11.0-5-g5756e6f7c` (ce-agent-kit master `5756e6f7c`, TASK-518/519).
- 호스트 provider cask gz-git 0.14.2, 2026-11-04 zero-need 증명
  (gzh-cli-devbox TASK-349 경로 기록).
