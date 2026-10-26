# Tasks — gzh-cli-gitforge

> Last Updated: 2026-10-23

## 구조

```
tasks/
├── issue/       # 확인된 결함 / 후속 설계 필요 항목 (이 저장소가 소유)
├── receipts/    # done-review 영수증 (ce-tasks.yaml aux-dirs)
└── README.md    # 열린 항목 인덱스 (이 파일)
```

착수 대기·진행 중 **작업 큐는 여기에 두지 않는다.** devbox 루트가 소유한다 —
아래 [Task ownership](#task-ownership) 참조.

규약: **1파일 = 1작업**, `NN-kebab-case-title.md`, 인덱스 파일은 `README.md`/`INDEX.md`만.
(`gzh-cli/tasks/` 규약 준용)

완료·감사 기록: [docs/changelog/tasks-history.md](../docs/changelog/tasks-history.md)

______________________________________________________________________

## Open Issues (후속)

2026-10-24 기준 열린 카드만 둔다. 닫힌 카드는 각 카드의 Resolution과
[tasks-history](../docs/changelog/tasks-history.md)에 있다.

| #   | 이슈                                                                                                                | 우선순위 | 요약                                                                                                                                    |
| --- | ------------------------------------------------------------------------------------------------------------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| 056 | [release-on-version-bump](issue/056-release-on-version-bump.md)                                                     | P1       | 버전 올림이 릴리스되지 않아 기기 바이너리가 어긋났다. 코드는 2026-10-19 해결, 각 기기의 운영자 단계가 남음                              |
| 062 | [full-gate-duration](issue/062-full-gate-duration.md)                                                               | P2       | full gate 소요 시간 때문에 stale-base 재시도마다 한 시간이 든다. 커버리지·`-race` 축소는 소유자 결정 필요                               |
| 071 | [caller-identity-resolution-diverges](issue/071-caller-identity-resolution-diverges.md)                             | P3       | run과 cleanup이 전역 Git 설정에서 호출자 identity를 서로 다르게 해석한다                                                                |
| 50  | [rewritten-legacy-native-context-delivery](issue/50-rewritten-legacy-native-context-delivery.md)                    | P2       | 다시 쓴 task head가 원래 legacy native 거부를 정리하지 못한다. 수정 위치는 ce-agent-kit `legacy_context_reconcile.go`                   |
| 083 | [integrate-tests-stay-sequential-on-process-state](issue/083-integrate-tests-stay-sequential-on-process-state.md)   | P3       | `pkg/integrate`가 게이트 263초 중 243초를 쓴다(6bcd8d9f 재측정). 환경 주입과 분할 실행 중 방식 결정 대기                                |
| 098 | [integrate-argument-surface-adr-0161](issue/098-integrate-argument-surface-adr-0161.md)                             | P2       | ADR-0161이 퇴역시킨 `integrate` 인자 표면이 남아 있다. 서브커맨드별 결정과 CE `run-finish` 동시 변경 필요                               |
| 104 | [sync-and-status-loaders-duplicate-workspace-rules](issue/104-sync-and-status-loaders-duplicate-workspace-rules.md) | P3       | sync와 status가 같은 workspaces 설정을 서로 다른 로더로 읽고, 네 규칙이 두 곳에 중복 구현되어 네 번 어긋났다                            |
| 107 | [reusable-gate-worktree](issue/107-reusable-gate-worktree.md)                                                       | P3       | 고정 경로에서 재사용하는 거버넌스 게이트 worktree가 없다. 이득은 미측정(sigdock-idp-rs 632–1065s, 통제 측정 선행)                       |
| 112 | [runtask-parity-port-followups](issue/112-runtask-parity-port-followups.md)                                         | P3       | ISSUE-42 포트 리뷰 후속 네 가지: bare finish의 ACTIVE 집계, 자식 프로세스 그룹 미격리, 원격 ref 부재 시나리오, ABORTED 영수증 귀속 확인 |

[ISSUE-20](_archive/issue/20-forge-event-stream-product-surface.md)은 devbox에서 기각된 카드의 포인터만 남는다.

### Recently closed (2026-08-25)

| #   | 태스크                                            | 결과                                                                                                                                  |
| --- | ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| 26  | defaults-scan-depth-is-a-dead-config-key          | 모든 bulk scan에 계층형 기본 깊이를 적용하고 명시 플래그 우선·설정 확장자·반복 실행 계약을 테스트로 고정 (`7fa61c5`·`341ce6d`)        |
| 27  | task-branch-upstream-points-at-integration-branch | integration upstream 별도 진단, 안전한 task ref 처방, audit code 추가와 slash remote 정규화로 해결 (`f4743020`·`64c258c8`·`91cf9dcd`) |
| 25  | no-declarative-exclusion-for-bulk-write-commands  | `defaults.scan.exclude` 추가(`2dcaf8e`)와 빈 항목 제거(`33cc679`)로 해결. 저장소 단위 `readOnly`는 채택하지 않음                      |

### Recently closed (2026-08-24)

| #   | 태스크                                        | 결과                                                                                                                                                                                        |
| --- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 22  | changelog-exceeds-doc-size-gate               | 과거 릴리스 6개 라인을 `docs/changelog/`로 분할(1320줄/63KB → 559줄/38KB), 71커밋 백로그를 주제별로 서술, 항목 밀도 규칙과 `<!-- size-limit: 700 -->` 예산 명시                             |
| 24  | make-changelog-overwrites-handwritten-log     | `.make/dev.mk`의 `changelog:` 타깃 제거(후보 A). 설정도 도구도 없어 실행된 적 없는 파괴 경로였고, 기계 목록은 `.goreleaser.yaml`이 담당                                                     |
| 23  | golangci-cache-reports-removed-worktree-paths | `e7f631f`/`052325e`가 per-run `GOLANGCI_LINT_CACHE`와 저장소 밖 진단 차단을 이미 구현. 관측된 유령 237건은 설치 바이너리가 낡아서였다 — 게이트 수정은 `make install` 뒤에야 판정에 반영된다 |

### Recently closed (2026-08-14 quality-debt follow-up)

| #   | 태스크                                 | 결과                                                                                                                                                                                                                                                                                 |
| --- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 07  | llm-output-nondeterministic-map-order  | drop `sortLLMSummaryBlock`; consume published sorted core in `go.mod` (`51aadf0`), including `GOWORK=off` CI                                                                                                                                                                         |
| 21  | golangci-exclusion-paths-unanchored    | anchor `vendor`/`tmp`, remove unnecessary `.git` exclusion, measure 254 baseline, restore lint-zero gate; [deferred lint debt](_archive/issue/21-golangci-exclusion-paths-unanchored.md#%EC%A7%80%EC%97%B0%EB%90%9C-%EB%A6%B0%ED%8A%B8-%EB%B6%80%EC%B1%84) remains explicitly scoped |
| 08  | conflict-guard residual ExitCode audit | `status` → `runGit`; existence probes documented intentional                                                                                                                                                                                                                         |
| 11  | status consumer + fixture gaps         | ParseStatus C/unknown-worktree; switch/status fixtures; testutil fatals                                                                                                                                                                                                              |
| 16  | goheader                               | COMPANY=Gizzahub; linter disabled pending header-add                                                                                                                                                                                                                                 |
| 18  | golangci v1/v2                         | PATH has v2.12.2; install target skips when v2 present                                                                                                                                                                                                                               |

### Task ownership

Work items for this library are tracked at the **devbox root**:

→ `gzh-cli-devbox/tasks/` (e.g. todo/40-gz-git-forge-event-stream-product.md)

Local `tasks/issue/20-…` is a **pointer only**.
