# gz-git 완성도 점검 리포트 — 설계·구현 갭 분석

- **작성일**: 2026-07-02
- **대상**: gzh-cli-gitforge (gz-git 바이너리)
- **방법**: 설계 문서 인벤토리 / 구현 코드 맵 / 미완성 마커 스캔 3방향 병렬 조사 후 교차 검증

## 1. 총평

핵심 기능(Phase 1~7: bulk 작업, forge 동기화, workspace, 프로필 설정)은 **구현 완료 상태이며 치명적(P0) 미완성은 없음**.
23개 공개 패키지 전부에 테스트가 존재하고, 커맨드 트리에 죽은 코드(미등록 커맨드)도 없음.

다만 아래 3종류의 갭이 확인됨:

1. **계획됐으나 미구현**: Advanced TUI(Phase 8.1), watch 사운드 알림, Phase 9~10 항목
2. **문서·코드 불일치**: 버전 표기 혼선(VERSION 0.6.1 vs CHANGELOG 3.0.0), 문서가 구현을 못 따라감
3. **품질 게이트 미충족 신호**: cmd/ 계층 테스트 부족(40개 파일 중 테스트 3개)

## 2. 설계 대비 미구현 기능

### 2.1 설계 완료, 구현 미착수

| 기능 | 출처 | 상태 | 비고 |
|------|------|------|------|
| Advanced TUI (`status --tui` 인터랙티브 뷰) | `docs/design/ADVANCED_TUI.md`, Phase 8.1 | 설계만 완료 | `pkg/tui`는 테이블/프로그레스 수준. 선택형 repo 리스트·배치 실행 UI 없음 |
| watch 사운드 알림 | `cmd/gz-git/cmd/watch.go:340-363` | **placeholder** | `--notify` 플래그는 배선됐으나 `playSystemSound()`가 빈 stub |
| 성능 최적화·캐싱 레이어 | `docs/00-product/06-roadmap.md` Phase 9 | 계획 단계 | status 결과 캐싱, 대형 repo(10k+ 파일) 대응 없음 |
| Bitbucket / Azure DevOps 프로바이더 | 로드맵 Phase 10 | 계획 단계 | `pkg/config/validator_test.go:54`에서 명시적 거부 확인 |
| 서브모듈 전용 워크플로우 / LFS | 로드맵 Phase 10 | 계획 단계 | clone 시 `--recurse-submodules`만 지원 |
| 플러그인 아키텍처 | 로드맵 Phase 10 | 계획 단계 | 커스텀 커밋 템플릿/충돌 전략 확장점 없음 |
| gzh-cli 완전 통합 | `docs/specs/70-gzh-cli-integration.md` Phase 7.2 | ⏳ 보류 | gzh-cli 측 작업 대기 |

### 2.2 라이브러리는 있으나 CLI 미노출

| 기능 | 라이브러리 | CLI |
|------|-----------|-----|
| **Worktree 관리** | `pkg/branch/worktree.go` (add/remove/list, 테스트 포함) | ❌ `gz-git worktree` 커맨드 없음 — 병렬 워크플로우가 핵심 가치인 도구에서 아까운 공백 |
| **Git 훅 실행** | `pkg/hooks` (셸 없는 안전 실행) | ❌ 훅 설치/관리 커맨드 없음 |

### 2.3 GitHub Enterprise 미지원

GitLab·Gitea는 custom BaseURL을 지원하지만 **GitHub 프로바이더는 github.com 고정**
(`pkg/github/`). GHE 사용 조직은 forge 동기화 불가. 로드맵에도 명시돼 있지 않음.

## 3. 문서·코드 불일치 (문서 부채)

| 항목 | 내용 |
|------|------|
| **버전 표기 혼선** | `VERSION`/`version.go` = **0.6.1**, `CHANGELOG.md` 최신 릴리스 = **3.0.0** (2026-01-21). 어느 쪽이 정본인지 정리 필요 |
| **미릴리스 breaking change** | CHANGELOG Unreleased에 `merge` 커맨드 제거, `sync ...` → `forge ...` 리네임이 쌓여 있음. 릴리스 태깅 필요 |
| **Phase 8.3 상태 역전** | `docs/design/INTERACTIVE_MODE.md`는 "설계 단계"라 하지만 `pkg/wizard`(charmbracelet/huh 기반 sync setup·branch cleanup·profile create 위저드)가 이미 구현됨 → 문서를 "구현 완료"로 갱신해야 함 |
| **구 커맨드 표기 잔존** | `docs/specs/00-overview.md` 등 일부 문서가 `sync from-forge` 구조로 기술. 실제는 `forge from` / 루트 `sync`(quicksync). 마이그레이션 후 문서 일괄 갱신 누락 |
| **경로 표기 불일치 의심** | 문서는 `~/.config/gz-git/`, 코드 일부는 `~/.config/gzh-git/` — 실제 경로 확인 후 통일 권장 |
| **잔여 파일** | 루트 `test.yaml` (샘플 설정) — .gitignore 처리 또는 삭제 (세션 리뷰에서도 지적됨) |

## 4. 코드 내 미완성·품질 부채

### 4.1 기능성 미완성 (P2)
- `cmd/gz-git/cmd/watch.go:340-363` — 사운드 알림 placeholder (macOS afplay / Linux paplay / Windows Beep 계획 주석만 존재)
- `pkg/watch/watcher.go:385` — 경로 매칭 고도화 TODO

### 4.2 리팩토링 TODO (인지 복잡도, nolint 처리 중) — 7건
`pkg/repository/bulk.go:1482,1941` · `bulk_diff.go:221` · `bulk_commit.go:133` · `client.go:124` · `pkg/history/analyzer.go:123` · `contributor.go:184`

### 4.3 테스트 커버리지 갭
- **cmd/gz-git/cmd/**: Go 파일 40개 중 테스트 파일 **3개** — PRODUCT.md 품질 게이트(cmd/ ≥ 70%) 충족 의심. 커맨드 계층 검증은 사실상 tests/ 통합 테스트에 의존
- **internal/config**: 테스트 0개 (112 LOC, 프로바이더 자격증명 로더)
- pkg/ 23개 패키지는 전부 테스트 보유 ✅

## 5. 동종 도구 대비 추가 제안 기능

ghq / gita / mu-repo / myrepos / git-xargs / meta 등과 비교한 공백:

| 제안 | 근거 | 우선순위 |
|------|------|---------|
| **`gz-git exec <cmd>`** — 전체 repo에 임의 명령 병렬 실행 | gita·mu-repo·myrepos의 핵심 기능. bulk 인프라(scanner+parallel)가 이미 있어 구현 비용 낮음. 단, 보안 원칙(no `sh -c`)과의 조화 설계 필요 | **높음** |
| **`gz-git worktree`** CLI 노출 | 라이브러리 완성 상태, 병렬 워크플로우 제품 비전(PRODUCT.md G3)과 직결 | **높음** |
| **GitHub Enterprise (custom base URL)** | 기업 사용자 차단 요소. GitLab에는 이미 동일 기능 존재 | 높음 |
| **Forge 쓰기 작업** — repo 생성/아카이브/미러링 | 현 프로바이더는 read-only(list/get). org 마이그레이션·백업 시나리오 불가 | 중간 |
| **PR/MR 일괄 작업** — 다중 repo 브랜치 push 후 PR 생성 (git-xargs 스타일) | bulk commit/push 다음의 자연스러운 단계. 대규모 리팩토링 배포에 필수 | 중간 |
| **status 결과 캐싱** | Phase 9에 이미 계획됨. 50+ repo에서 반복 status 비용 절감 | 중간 |
| **토큰 보안 저장** — OS keychain/credential helper 연동 | 현재는 env var/설정파일 평문 의존 | 중간 |
| **repo 백업/아카이브 export** (gickup 스타일) | forge 동기화 도구의 인접 수요 | 낮음 |
| **self-update 커맨드** | 배포 편의. Homebrew 셋업 문서(`docs/homebrew-setup.md`)와 연계 | 낮음 |

## 6. 권장 액션 (우선순위순)

1. **릴리스 정리**: VERSION↔CHANGELOG 버전 체계 통일 + Unreleased breaking change 릴리스 태깅
2. **문서 동기화**: Phase 8.3 상태 갱신, `sync`→`forge` 리네임 문서 반영, config 경로 표기 통일
3. **worktree CLI 노출**: 라이브러리 완성돼 있어 저비용·고효과
4. **`exec` 커맨드 추가 검토**: 동종 도구 대비 가장 눈에 띄는 기능 공백
5. **cmd/ 계층 테스트 보강** + internal/config 테스트 추가
6. **watch 사운드 알림**: 구현하거나, 당분간 미구현이면 `--notify` 도움말에 명시
7. GitHub Enterprise 지원을 로드맵에 명시적으로 추가

---
*조사 범위: PRODUCT.md, REQUIREMENTS.md, ARCHITECTURE.md, CHANGELOG.md, docs/(specs·design·00-product·reviews), cmd/, pkg/ 23개 패키지, internal/, tests/. 에이전트 조사 결과 중 핵심 주장(blame 커맨드, worktree/exec 부재, 버전 불일치, 테스트 비율)은 코드에서 직접 재검증함.*
