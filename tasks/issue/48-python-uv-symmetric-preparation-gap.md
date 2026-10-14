# ISSUE: infra-ops의 Python/uv 실행환경을 통합 양측에 대칭 준비할 프로필이 없다

- status: open
- priority: P2
- category: quality/integrate-baseline
- created_at: 2026-10-14
- owner: `gzh-cli-gitforge/pkg/integrate`
- implementation-grade: strong
- review-grade: strong (독립 리뷰)
- work-card: `gzh-cli-devbox:tasks/todo/324-python-uv-symmetric-preparation.md` (TASK-324)
- related: [ISSUE-034](34-generalize-prepare-profile-baseline-probe-dies-without-bootstrap.md), infra-ops `tasks/issue/005-integrate-baseline-unmeasurable-in-pristine-worktree.md`

## 확인된 문제

infra-ops의 통합 검사는 작업 트리에 맞는 가상환경 없이 `make check`를 실행한다.
`credential-inventory-check`의 `uv run --no-project`가 `yaml`을 import하지 못해
브랜치와 pristine baseline 모두 측정에 실패할 수 있다. `branch had 0`은
file:line 진단 개수이며 검사 통과를 뜻하지 않는다.

소비자 ISSUE-005는 이 원인의 수정 주체를 통합 도구로 지정하고, 소비자 게이트의
완화나 상시 `--allow-skipped-checks` 처리를 해결로 인정하지 않는다.
이 등록은 구현·프로필 이름·네트워크 정책의 채택을 뜻하지 않는다.

## 증거와 중복 확인

- infra-ops 기준 소스: `203062d6fa01d021edb8eed555985288f0d7716c`.
  ISSUE-005의 2026-10-14 재발은 source `4901e75f`, target `c0936909`,
  baseline 로그 `make-check-2326251467.log`의 `No module named 'yaml'`를 기록한다.
  같은 카드의 `2026-10-13` 절이 원인 1과 운영자 소관 SOPS 키 문제를 분리한다.
- infra-ops `pyproject.toml`의 `[tool.uv.sources]`는 `codes/sbkube`와
  `codes/cloud-script-transformer`를 로컬 editable 입력으로 요구한다.
  `scripts/worktree-env.py`의 `bootstrap`은 고정 child revision, 격리 venv,
  `uv sync --frozen`, Node lockfile 설치와 소유권 검사를 수행한다.
  `emit_environment`는 `UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`, `UV_NO_SYNC=1`을
  내보낸다. venv 생성만으로 Make 자식이 그 환경을 사용한다고 볼 수 없다.
- gzh-cli-gitforge 등록 기준: `233eb907d80e90e0cb0f3b6bcc1b1abdfe6e2d84`.
  `pkg/integrate/prepare.go`의 `runPrepareProfileWithInputs`는 flow-taskchain,
  Cargo, pnpm, familybook Ent만 지원한다. Python/uv 프로필은 없다.
  `prepareLegacySourceWithResource`는 프로필 미선언 시 live source와 pristine
  baseline의 준비 비대칭을 명시한다. `check_make.go`는 Make에 상속 환경과
  `MAKELEVEL`, `MAKEFLAGS`, `LC_ALL=C`를 전달한다.
- ISSUE-034는 고정 프로필의 대칭 실행 메커니즘과 flow-taskchain 지원을 해결했다.
  Python/uv 및 위 editable 입력의 준비는 그 해결 범위에 포함되지 않는다.
- 등록 전 로컬 issue 01~46 및 GitHub 전체 상태 이슈 최근 100건의 title/body에서
  Python/uv/venv 준비 문제의 별도 카드를 찾지 못했다. 원격 검색은 조회 범위와
  표현 차이의 한계가 있으며, 구현 착수 시 다시 중복을 확인한다.

## 다음 작업과 설계 후보

고정된 opt-in built-in 프로필 하나로 양쪽 detached 작업 트리의 입력을 대칭
준비하고, 각 Make 자식에 해당 환경을 전달하는 방향을 검토한다.
`branch.prepareProfile`의 명령·경로·임의 환경 입력 금지 계약을 유지한다.
infra-ops가 프로필을 선언하는 소비자 변경은 지원 구현·검증 뒤에 진행한다.

구현 전에 다음 계약을 확정한다.

1. 통합 도구의 고정 구현과 별도 검토된 좁은 adapter 중 소유 경계를 선택한다.
   저장소의 `scripts/worktree-env.py`를 임의 명령으로 직접 실행하는 것은
   현재 프로필 계약의 해법으로 채택하지 않는다.
1. Python·uv·child OID·remote·lockfile 입력, Node 준비 범위와 출처 기록을 정한다.
   venv만 준비하고 전체 게이트의 나머지 입력을 누락하지 않는다.
1. clone·registry·cache의 허용 범위, offline 실패, timeout과 heavy slot 분류를
   정한다. 운영 자격증명·SOPS 키 준비는 이 프로필의 범위 밖이다.
1. 통합 도구가 소유하는 임시자원과 환경 전달·회수 계약을 정한다.
   live checkout의 `.venv`, `codes/`, `node_modules`, `tmp/worktree-env`를
   재사용·변경하지 않고, symlink와 타 작업의 산출물을 임의 채택하지 않는다.

## 구현 완료 조건

- 알려진 고정 ID만 허용하고 source/target 선언 비교·ref·detached HEAD 보호를 유지한다.
- 준비 없는 fixture는 Python import 실패를 재현하고, 프로필을 선언하면 양쪽이
  같은 잠긴 입력으로 통과한다. `--allow-skipped-checks`는 쓰지 않는다.
- 두 Make probe가 각각 자기 `UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`,
  `UV_NO_SYNC=1`을 받는 것을 검사한다.
- 도구·고정 입력 부재, lockfile 불일치, unsafe path, 준비 timeout은 명확한
  preparation 오류로 끝나며 baseline 통과로 보고하지 않는다.
- receipt가 profile/input 출처를 보존하고 task-owned 임시자원만 회수한다.
  기존 네 프로필과 infra-ops의 standalone 환경 준비 회귀검사를 보존한다.
- infra-ops에서 프로필 채택 후 해당 Python 측정 축의 대칭 준비 증거를 확보한다.
  기존 Ansible 로케일 문제·호스트 비밀 전제·GA4 timeout의 해결까지 주장하지 않는다.

## 2026-10-14 구현 진행

- gzh 측 closed profile `infra-ops-python-uv-v1` 구현은 source와 target 각 commit의
  `scripts/worktree-env-inputs.json`을 읽어 각각의 child revision을 적용한다.
  최초 채택에서만 profile 선언이 없는 target에 source 입력을 적용하며
  `target-input-mode=source-bootstrap`으로 기록한다. 이미 선언한 target의
  manifest 누락은 오류다. manifest에는 revision 외 설정을 넣지 않고 canonical
  remote·path·명령은 executor가 소유한다.

- child clone과 venv는 linked worktree 바깥의 task-owned prepare root에 두고,
  worktree의 `codes/*`에는 symlink만 만든다. nested Git checkout이 worktree
  cleanup을 막지 않도록 한 계약이다.

- 아직 infra-ops 소비자 선언·실제 통합 검증은 이 카드의 후속 범위다. 현재 구현
  검증에는 실제 Git checkout과 legacy gate를 사용하는 회귀검사도 포함한다.
  child pin 변경으로 source만 실패하고 target은 통과하는 경우, Git 설정과
  hook 격리, receipt 재읽기, 양쪽 환경 회수를 확인했다. 전체 quality gate와
  독립 리뷰는 별도 확인하며, 다른 세션의 설치본 갱신 중에는 parity reference를
  자동 재캡처하지 않는다. ISSUE-041의 미해결 실패를 통과로 보고하지 않는다.

- 최신 코드의 독립 strong 리뷰, 집중 검사(20.102초), 기존 Pnpm/Cargo/
  Legacy/FlowTaskchain 검사(16.364초), lint(0 issues), config 전체 검사는 통과했다.
  증거는 `tmp/python-uv-prepare-evidence/final-focused.log`,
  `existing-profiles.log`, `final-lint.log`와 실행 도구 출력에 있다.

- 전체 `make quality-check` 시도는 종료 코드 2였다. 최종 리뷰 수정 전에
  시작한 실행으로, lint·security·build 뒤 unit 단계에서 멈췄다.
  `pkg/integrate`의 기존 테스트는 임시 디렉터리/Git 파일 생성의
  `No space left on device`로 실패했고, parity는 ISSUE-041의 기존 불일치였다.
  `cmd/gz-git/cmd`는 20분 제한을 넘어 stdout 캡처 경로에 머물러, 소유한
  테스트 프로세스만 SIGQUIT으로 stack을 기록하고 중단했다. 원인은 별도 조사한다.
  로그는 `tmp/python-uv-prepare-evidence/quality-after-lint-fix.log`다.
  후속 integration/E2E/coverage 단계와 최종 코드 전체 gate 통과는 주장하지 않는다.
  설치본과 저장 공간이 안정된 뒤 source 정렬·최종 gate·통합을 이어간다.

- 후속 확인에서 stdout 캡처 문제는 master의 ISSUE-051과 일치했고 이미
  `ff54f169`에서 수정돼 있었다. 새 이슈는 등록하지 않고 해당 source로 명시적으로
  재기준화했다. `final-rebased-quality.log`의 최종 코드 전체 검사는 종료 코드 2이며,
  실패는 기존 `tests/parity`의 CE `b79c1a4a` → `9877764d` 불일치뿐이다.
  명령 패키지는 51.405초, `pkg/integrate`는 677.225초에 통과했다.
  이 실행에는 디스크 공간 부족 실패가 없다. 뒤의 integration/E2E/critical coverage
  게이트는 별도 실행해 종료 코드 0을 받았으며 `final-remaining-gates.log`에 보존한다.
  참조 파일은 변경하지 않았다. 소비자 프로필 선언과 실제 양측 준비 수용이 남아
  있으므로 이 카드 전체의 해결을 주장하지 않는다.

- `e77bb528`의 sibling-primary 변경까지 재기준화한 검사도 완료했다.
  `current-source-quality.log`는 기존 CE parity 불일치만으로 종료 코드 2였고,
  `pkg/integrate`는 597.768초에 통과했다. `current-source-remaining-gates.log`의
  integration/E2E/critical coverage 검사는 종료 코드 0이다.
  이후 `d45c4d3e`의 독립 forge-bootstrap 변경까지 충돌 없이 재기준화했다.
  해당 변경은 기존 prepare/config/integrate 코드와 의존성 선언을 변경하지 않는다.
  현재 결합 tree의 `latest-source-quality.log`도 종료 코드 2이며 실패는 같은
  기존 parity guard뿐이다. 새 forge-bootstrap(4.978초), 명령 패키지(50.980초),
  `pkg/integrate` 검사는 통과했다. `latest-source-remaining-gates.log`의
  integration/E2E/critical coverage는 종료 코드 0이다. 변경 문서는 별도
  `latest-source-format.log`로 format-check 종료 코드 0을 확인했다.

- infra-ops의 closed 입력 manifest와 기존 helper 소비 변경은 master
  `10f8cc3a329bd047a8b61a5f833d45bf1560140e`로 전달했다. 실제 frozen bootstrap과
  native `make check`(5분 28.901초)가 통과했고, 작업 branch/worktree/준비 환경은
  회수했다. 새 builtin 프로필의 소비자 선언은 아직 적용하지 않았다.
  다른 세션이 설치본을 갱신 중이므로 경쟁 설치나 parity 재캡처 없이 기다린다.

## 범위 밖

일반적인 임의 repository hook, 자동 키 배치·회전, infra-ops 게이트 완화,
기존 CASE/coverage 상한 조정, 다른 준비 프로필의 재설계는 하지 않는다.
