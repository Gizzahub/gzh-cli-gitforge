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

2026-10-22 기준 열린 카드만 둔다. 닫힌 카드는 각 카드의 Resolution과
[tasks-history](../docs/changelog/tasks-history.md)에 있다.

| #   | 이슈                                                                                                                | 우선순위 | 요약                                                                                                                                                           |
| --- | ------------------------------------------------------------------------------------------------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 42  | [runtask-port-follow-ce-f540f972](issue/42-runtask-port-follow-ce-f540f972.md)                                      | P2       | Worktrunk 0.80 라인은 반영됐다. 시작·종료·복구가 아직 로컬 source 브랜치를 기준으로 삼고 `refs/remotes/origin` SHA를 쓰지 않는다. port 모드 parity 재실행 필요 |
| 50  | [rewritten-legacy-native-context-delivery](issue/50-rewritten-legacy-native-context-delivery.md)                    | P2       | 다시 쓴 task head가 원래 legacy native 거부를 정리하지 못한다. 수정 위치는 ce-agent-kit `legacy_context_reconcile.go`                                          |
| 083 | [integrate-tests-stay-sequential-on-process-state](issue/083-integrate-tests-stay-sequential-on-process-state.md)   | P3       | `pkg/integrate`가 게이트 263초 중 243초를 쓴다(6bcd8d9f 재측정). 환경 주입과 분할 실행 중 방식 결정 대기                                                       |
| 098 | [integrate-argument-surface-adr-0161](issue/098-integrate-argument-surface-adr-0161.md)                             | P2       | ADR-0161이 퇴역시킨 `integrate` 인자 표면이 남아 있다. 서브커맨드별 결정과 CE `run-finish` 동시 변경 필요                                                      |
| 100 | [removed-key-scan-missed-singlepageweb](issue/100-removed-key-scan-missed-singlepageweb.md)                         | P2       | ISSUE-89 제거 키 재스캔이 singlepageweb-devbox와 자식 저장소, 그리고 mst 전역 설정을 빠뜨렸다                                                                  |
| 101 | [status-lists-nested-config-repos-sync-skips](issue/101-status-lists-nested-config-repos-sync-skips.md)             | P3       | `sync.recursive`가 없는 중첩 config workspace를 sync는 건너뛰는데 status는 그 저장소를 나열한다                                                                |
| 102 | [status-loader-pathless-config-workspace-overflows](issue/102-status-loader-pathless-config-workspace-overflows.md) | P3       | `path` 없는 `type: config` workspace가 status 로더를 자기 설정 재귀로 스택 오버플로시킨다                                                                      |
| 103 | [commit-edit-commits-before-editor](issue/103-commit-edit-commits-before-editor.md)                                 | P2       | `commit -e`를 `--yes` 없이 쓰면 편집기가 열리기 전에 자동 메시지로 커밋된다                                                                                    |
| 106 | [legacy-cleanup-identity-inode-reuse](issue/106-legacy-cleanup-identity-inode-reuse.md)                             | P2       | legacy cleanup ABA 테스트가 Linux에서 간헐적으로 다시 만든 worktree를 통과시킨다(inode 재사용 추정, 미확인)                                                    |
| 107 | [reusable-gate-worktree](issue/107-reusable-gate-worktree.md)                                                       | P2       | 고정 경로에 재사용하는 거버넌스 게이트 worktree가 없어 후보 게이트마다 workspace를 콜드 컴파일한다(sigdock-idp-rs 924s vs 632s)                               |

[ISSUE-20](issue/20-forge-event-stream-product-surface.md)은 devbox에서 기각된 카드의 포인터만 남는다.

### Recently closed (2026-08-25)

| #   | 태스크                                            | 결과                                                                                                                               |
| --- | ------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| 26  | defaults-scan-depth-is-a-dead-config-key          | 모든 bulk scan에 계층형 기본 깊이를 적용하고 명시 플래그 우선·설정 확장자·반복 실행 계약을 테스트로 고정 (`7fa61c5`·`341ce6d`)     |
| 27  | task-branch-upstream-points-at-integration-branch | integration upstream 별도 진단, 안전한 task ref 처방, audit code 추가와 slash remote 정규화로 해결 (`b8ad882`·`9a42ae4`·`f1c9881`) |
| 25  | no-declarative-exclusion-for-bulk-write-commands  | `defaults.scan.exclude` 추가(`2dcaf8e`)와 빈 항목 제거(`33cc679`)로 해결. 저장소 단위 `readOnly`는 채택하지 않음                   |

### Recently closed (2026-08-24)

| #   | 태스크                                        | 결과                                                                                                                                                                                        |
| --- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 22  | changelog-exceeds-doc-size-gate               | 과거 릴리스 6개 라인을 `docs/changelog/`로 분할(1320줄/63KB → 559줄/38KB), 71커밋 백로그를 주제별로 서술, 항목 밀도 규칙과 `<!-- size-limit: 700 -->` 예산 명시                             |
| 24  | make-changelog-overwrites-handwritten-log     | `.make/dev.mk`의 `changelog:` 타깃 제거(후보 A). 설정도 도구도 없어 실행된 적 없는 파괴 경로였고, 기계 목록은 `.goreleaser.yaml`이 담당                                                     |
| 23  | golangci-cache-reports-removed-worktree-paths | `e7f631f`/`052325e`가 per-run `GOLANGCI_LINT_CACHE`와 저장소 밖 진단 차단을 이미 구현. 관측된 유령 237건은 설치 바이너리가 낡아서였다 — 게이트 수정은 `make install` 뒤에야 판정에 반영된다 |

### Recently closed (2026-08-14 quality-debt follow-up)

| #   | 태스크                                 | 결과                                                                                                                                                                                                                                                                        |
| --- | -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 07  | llm-output-nondeterministic-map-order  | drop `sortLLMSummaryBlock`; consume published sorted core in `go.mod` (`51aadf0`), including `GOWORK=off` CI                                                                                                                                                                |
| 21  | golangci-exclusion-paths-unanchored    | anchor `vendor`/`tmp`, remove unnecessary `.git` exclusion, measure 254 baseline, restore lint-zero gate; [deferred lint debt](issue/21-golangci-exclusion-paths-unanchored.md#%EC%A7%80%EC%97%B0%EB%90%9C-%EB%A6%B0%ED%8A%B8-%EB%B6%80%EC%B1%84) remains explicitly scoped |
| 08  | conflict-guard residual ExitCode audit | `status` → `runGit`; existence probes documented intentional                                                                                                                                                                                                                |
| 11  | status consumer + fixture gaps         | ParseStatus C/unknown-worktree; switch/status fixtures; testutil fatals                                                                                                                                                                                                     |
| 16  | goheader                               | COMPANY=Gizzahub; linter disabled pending header-add                                                                                                                                                                                                                        |
| 18  | golangci v1/v2                         | PATH has v2.12.2; install target skips when v2 present                                                                                                                                                                                                                      |

### Task ownership

Work items for this library are tracked at the **devbox root**:

→ `gzh-cli-devbox/tasks/` (e.g. todo/40-gz-git-forge-event-stream-product.md)

Local `tasks/issue/20-…` is a **pointer only**.
