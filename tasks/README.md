# Tasks — gzh-cli-gitforge

> Last Updated: 2026-10-17

## 구조

```
tasks/
├── issue/       # 확인된 결함 / 후속 설계 필요 항목 (이 저장소가 소유)
├── HISTORY.md   # 완료·감사 기록
└── README.md    # 열린 항목 인덱스 (이 파일)
```

착수 대기·진행 중 **작업 큐는 여기에 두지 않는다.** devbox 루트가 소유한다 —
아래 [Task ownership](#task-ownership) 참조.

규약: **1파일 = 1작업**, `NN-kebab-case-title.md`, 인덱스 파일은 `README.md`/`INDEX.md`만.
(`gzh-cli/tasks/` 규약 준용)

완료·감사 기록: [HISTORY.md](HISTORY.md)

______________________________________________________________________

## Open Issues (후속)

stdout/stderr 캡처의 선재 교착은 [ISSUE-51](issue/51-stdout-capture-pipe-deadlock.md)에 재현·수정을 보존했다. 두 도우미 수정 후 생략 없는 전체 CLI 테스트와 린트가 통과했다.

| #   | 이슈                                                                 | 요약                                                                                                                                                                                                                                                            |
| --- | -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 28  | goreleaser-brews-deprecated-and-ci-pins-latest                       | 저장소 코드·CI 수정은 완료. v0.8.0·v0.9.0 Release run이 모두 `HOMEBREW_TAP_TOKEN` 401로 Cask 게시 단계에서 실패(GitHub Release 게시는 매번 성공). 빈 `gizzahub/homebrew-tap` bootstrap, 최초 Cask 게시·macOS 설치 검증이 남았다                                 |
| 29  | nosec-suppression-not-bound-in-if-init                               | 완료. master `fc8db6c`에서 hosted `security-code` 통과를 확인했고, master `1b1298e`/run 33606203470에서 hosted `Quality gate` 전체가 초록이다                                                                                                                   |
| 30  | d6-context-reference-contract-gaps                                   | 완료. P1 4건(매니페스트 상태 매트릭스, `componentOutcome: unknown` 제거와 `faultDomain` 도입, envelope/exit 불일치 규칙, CE→gz-git exit 매핑)과 P2 4건을 D6 문서로 종결했다. P3-1·P3-2는 pilot 실측을 선점하지 않도록 의도적으로 이월                           |
| 31  | hosted-only-test-failures-shell-and-default-branch                   | 완료. 픽스처의 `printf` 이스케이프와 `git init --bare` 기본 브랜치 의존을 제거했다(`1b1298e`). hosted `Quality gate` 초록 확인                                                                                                                                  |
| 32  | escaped-pipe-latency-assertion-measures-setup                        | 완료. `TestExecuteReadinessWaitDelayBoundsEscapedPipe`의 `started`가 러너 spawn 앞에서 찍혀 setup 지연까지 재고 있었다. 약 20회 중 1회 실패하던 선재 flake이며 `started`를 `cancel()` 직전으로 옮겨 해소했다(60/60 통과)                                        |
| 33  | integrate-subcommand-retirement                                      | ADR-0026에 따라 기각. `integrate`는 CE 통합 provider로 유지하며 기존 퇴역 제안은 역사 기록으로 남긴다                                                                                                                                                           |
| 34  | generalize-prepare-profile-baseline-probe-dies-without-bootstrap     | 해결. master `8a5b708`과 hosted CI `36422601532`가 대칭 prepare profile을 검증했고, flow-taskchain-devbox develop `7211e54d`는 같은 engine/mcp OID로 `--allow-skipped-checks` 없이 READY/integrated다                                                           |
| 35  | integrate-failure-output-discarded                                   | 수정·로컬 검증 완료. legacy make와 readiness runner의 실패 출력을 로컬 진단 파일에 보존한다. 실행 카드: devbox TASK-236                                                                                                                                         |
| 36  | make-timeout-windows-build                                           | Windows 빌드와 make recipe 자손 종료 수정. exact-SHA `a917faa` Windows 빌드·테스트 통과; 실행 카드: devbox TASK-237                                                                                                                                             |
| 37  | contextref-unconvert-lint                                            | Linux CI의 unconvert 경고를 Darwin 타입과 호환되게 교정. 실행 카드: devbox TASK-238                                                                                                                                                                             |
| 38  | hosted-gosec-diagnostic-directory                                    | 진단 디렉터리 `0700` 설정의 hosted G302 수정. exact-SHA `ece25ab` Quality gate 통과; 실행 카드: devbox TASK-239                                                                                                                                                 |
| 39  | make-timeout-configurable                                            | repo-root 선언 `branch.makeTimeout`으로 legacy 게이트 make 예산을 저장소별로 상향. 기본 15분 보존, fail-closed 파싱. 실행 카드: devbox TASK-260                                                                                                                 |
| 40  | gosec-go127-export-data                                              | go1.27.0으로 빌드한 gosec v2.22.10이 `package without types` 오류로 전면 실패. go1.26.7 빌드는 정상 — tools.mk 빌드 툴체인 고정 또는 핀 상향 필요                                                                                                               |
| 42  | runtask-port-follow-ce-f540f972                                      | 포트가 CE f540f972(원격 추적 ref 기준 source · SHA base · `--target origin/<branch>`)와 b09a605e(Worktrunk 0.80 라인)를 따르지 않아 parity 포트 모드 11개 실패. 재녹화 전에도 전부 실패(악화 아님). 분리 출처: devbox TASK-256                                  |
| 41  | ce-parity-golden-drift                                               | 2026-10-18 재발: golden CE 9fe1ce45와 설치 CE 1734ce1f 불일치. 후보·기준선 parity tree 동일, 기준선 실행은 설치 gz-git의 15분 제한으로 미완료. 참조 채택·설치는 별도 소유 작업이다.                                                                             |
| 43  | integrate-release-expected-source-sha                                | `integrate check/run`에 `--expect-source <sha>`가 없어 호출자(CE `run-release`)가 승인한 develop 커밋을 프로세스 안에서 고정할 수 없다. 외부 fetch가 끼면 승인 안 된 후손이 승격될 수 있다. 요청처: ce-agent-kit `f6493563`                                     |
| 45  | human-approval-review-queue                                          | 설계 확정·착수 대기. 사람 전용 apply 앞의 수작업 명령 조립을 줄인다: 읽기 전용 `bootstrap check`·`approval list`(에이전트 허용)와 TTY 전용 `approval review`(저장소별 확인 기본, `--group-confirm` 옵션). 요청 파일 없이 스캔. 훅·launcher는 ce-devenv 이슈 013 |
| 47  | interrupted-baseline-worktree-recovery                               | 중단된 integrate의 clean detached 기준선 worktree가 남고 CE task 회수에 포함되지 않는다. 신원·소유·종료 검증을 유지하는 provider 회수 경로가 필요하다                                                                                                           |
| 48  | python-uv-symmetric-preparation-gap                                  | Python/UV 범위 해결. source 9227bf3 및 infra-ops b7ad3178 실제 native full 수용·고정 입력·환경 전달·회수 확인. baseline Make는 미측정이며 GA4 문제 해결을 주장하지 않는다                                                                                       |
| 52  | legacy-json-unbound-mutation-flags                                   | 0.10.2에서 기존 JSON 필드를 실제 호출 결과로 채운다. receipt 우선순위·schema 유지·실제 Git 회귀 검증 완료; 전달 판정은 verification-evidence-followup 영수증에 기록한다                                                                                         |
| 53  | [atomic-install-publication](issue/53-atomic-install-publication.md) | checkout과 설치 디렉터리가 다른 파일시스템이면 직접 `mv`가 복사 경로를 사용해 실행 파일의 원자적 게시를 보장하지 못한다. destination staging은 미채택 후보이며 현재는 관측상 유휴 창에서만 설치하는 운영 완화다.                                                |

`v0.8.0`은 준비된 `VERSION`과 릴리스 노트 기준으로 2026-10-01 `9d7041c`에 발행됐다.
발행 당시 보류 조건이던 탭 bootstrap·최초 Cask 게시·macOS 설치 검증은 이슈 28이 계속
보유한다. `0.9.0`은 2026-10-02에 이어서 컷했다(실행 `gitforge-release-0-9`).

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
