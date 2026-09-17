# ISSUE: integrate의 prepare profile을 리포 선언형으로 일반화 — baseline 프로브가 부트스트랩 없는 워크트리에서 죽는 구조

- status: open
- priority: P2
- category: quality/integrate-baseline
- created_at: 2026-09-17T00:00:00+09:00
- affects: `pkg/integrate/prepare.go` (`runPrepareProfile`, `prepareLegacyTrees`, `familybookEntPrepareV1`), `pkg/integrate/check_make.go` (`baselineAgainstTarget`, `baselineCheckItem`)
- findings: `single-hardcoded-prepare-profile`,
  `baseline-probe-requires-bootstrap-the-repo-cannot-declare`
- related: [33-integrate-subcommand-retirement.md](33-integrate-subcommand-retirement.md) —
  `pkg/integrate` 는 퇴역 계획상 동결 대상이지만, 이 결함은 동결 전 상태에서 이미
  존재하는 측정 무결성 결함이며 퇴역과 무관하게 fail-by-default 강등을 강제한다

## Problem

- `pkg/integrate/prepare.go`의 `runPrepareProfile`은 단일 하드코딩 프로필
  `familybookEntPrepareV1`만 지원한다. 그 외 프로필 문자열은
  `unsupported preparation profile %q`로 거부된다(prepare.go:124-125).
  `prepareLegacyTrees`는 프로필이 없으면 브랜치 프로브를 integrate 실행
  디렉터리(라이브 working dir, `PrepareStateWorkingDir`)에서, 기준선 프로브를
  target SHA의 detached 워크트리에서 "bootstrapped by nothing"
  (`PrepareStatePristine`)으로 실행한다(`check_make.go` `baselineAgainstTarget`
  — `baseProbe.Prepared = PrepareStatePristine`).
- 그 결과 make 대상이 부트스트랩 산출물(gitignored 서브프로젝트 클론,
  node_modules 등)을 필요로 하는 리포에서는 기준선 프로브가 file:line 진단 0건으로
  죽고 → `BaseMeasurementUnknown` → `BaselineUnmeasurable` →
  fail-by-default → 매번 `--allow-skipped-checks` 강등이 강제된다. 이는 하네스의
  정직한 설계(`baselineCheckItem` 주석: 미측정 게이트=스킵 게이트)이지만, 리포
  쪽에서 대칭 부트스트랩을 선언할 방법이 없어 근본 해결이 불가능하다.
- 실측 사례(2026-09-17): flow-taskchain-devbox develop 팁(`cbc6e0df`) detached
  워크트리에서 `make check` rc=2, 진단 0건. 실패 줄:
  - `[ERROR] dangling evidence file: flow-taskchain-engine/internal/routes/task_relations_test.go`
  - `[ERROR] reference fixture source is missing: flow-taskchain-engine/docs/api/openapi.yaml`

  전부 gitignored인 엔진 서브프로젝트 클론 부재가 원인. 같은 리포의
  `gap-303-status` 검사는 같은 상황을 SKIP으로 처리하는 선례다.
- `PrepareStateProfilePrepared` 상태와 양측 대칭 실행 경로(`prepareLegacyTrees`)는
  이미 구현돼 있다 — 프로필 선언만 일반화하면 된다.

## Request

- 리포가 `.gz-git.yaml` 등에서 prepare profile을 선언하고, 브랜치·기준선 양쪽
  워크트리를 같은 프로필로 부트스트랩하게 한다.
- 기존 `familybookEntPrepareV1` 프로필의 보안 장치를 참조 설계로 유지할 것:
  고정 타임아웃(`prepareProfileTimeout`), 격리된 HOME/XDG/cache env, prepare 전후
  git ref 변경 검증, `validatePreparedStatus`, symlink 사슬 거부
  (`rejectEntSymlinkChain`). 리포 코드를 두 SHA에서 실행한다는 점을 명시적으로
  다룰 것.

## 대안 검토

- 리포 쪽에서 클론 부재 시 SKIP으로 강등하는 방법은 하네스가 "SKIPPED CHECK"도
  fail-by-default로 처리(`check_make.go:451` — `SKIPPED CHECK (not a pass);
  pass --allow-skipped-checks to downgrade`)해 운영 효과가 없다.
- 현상 유지는 영구 `--allow-skipped-checks` 강등을 의미한다.
