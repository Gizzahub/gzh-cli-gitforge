# ISSUE: 사람 전용 readiness 승인이 저장소마다 수작업 명령 조립에 의존한다

- status: open (설계 결정 완료 2026-10-06 — 착수 대기)
- priority: P2
- category: ux/integrate
- created_at: 2026-10-06
- owner: `gzh-cli-gitforge/cmd/gz-git` (`integrate bootstrap`, `integrate readiness update`)
- work-card: 없음 — 착수 시 devbox 루트에 실행 카드를 만든다
- linked: ce-devenv `tasks/issue/013-gz-git-approval-launchers.md` (hook·launcher 쪽)

## 증상

`integrate bootstrap apply`와 `integrate readiness update apply`는 사람만 실행한다.
에이전트는 PreToolUse 훅에 막히고, CLI는 TTY와 `--confirm <CONFIRM_DIGEST>`를
요구한다. 이 경계는 의도된 것이다. 문제는 그 경계 앞에서 생기는 일이다.

2026-10-06 sigdock 컴포넌트 6개를 bootstrap하는 과정에서 아래 일이 실제로 일어났다
(sigdock-pass-devbox `tasks/manual/sigdock-component-readiness-bootstrap.md`).

1. 에이전트는 plan을 미리 만들어 둘 수 없다. plan TTL이 15분이므로, 사람이 읽을
   무렵이면 만료된다. 그래서 에이전트는 명령문만 출력하고, 사람은 저장소 경로·브랜치·
   plan 파일·digest를 셸에 다시 입력한다.
2. 다시 입력하다 오타(`readiness-bottstrap`)가 났다. 저장소가 여러 개라 에이전트가
   그때그때 반복문 셸 스크립트를 따로 만들어 건넸다. 이런 일회용 스크립트는 작업할
   때마다 새로 생긴다.
3. 여섯 plan은 `readiness_tree_digest`(`7f78973c…`)와 `runner_oid`가 같았다. 실제로
   검토할 대상은 runner 하나였는데도 사람은 digest 6개를 따로 확인하고 입력해야 했다.
4. bootstrap은 "1커밋 앞섬"과 "계약 파일만 변경"을 요구한다. 그런데 이 조건은 plan을
   실행해 볼 때까지 드러나지 않는다. 그래서 준비 브랜치가 이 조건을 어긴 채(3커밋)
   사람 앞에 도착했다.

## 결정 (2026-10-06)

### D1. 하위 명령 세 개를 추가하고, 요청 파일은 두지 않는다

| 하위 명령 | 하는 일 | 선언 Effect |
| --- | --- | --- |
| `integrate bootstrap check [--branch …] [--target …]` | bootstrap 사전 조건을 판정한다(1커밋 fast-forward, 변경 경로가 `.gz-git.yaml`의 `branch.readiness`와 `.gz-git/readiness/` 아래 일반 파일뿐, target 미선언, symlink·submodule 없음). plan을 만들지 않는다. | `tracking-refs` (fetch 시) |
| `integrate approval list [<workspace>]` | workspace를 스캔해 bootstrap·readiness-update 대상 브랜치를 나열하고, 같은 readiness 트리끼리 묶어 보여 준다. 대상이 아닌 후보도 이유와 함께 표시한다. | `tracking-refs` |
| `integrate approval review [<workspace>]` | `list`와 같은 스캔 → 묶음별로 plan을 그 자리에서 생성 → 확인 → apply. TTY가 없으면 거부한다. | apply와 같다(`refs`, `tracking-refs`, `remote`, `readiness-contract`) |

승인 요청 파일(에이전트가 쓰는 대기열)은 두지 않는다. 요청 파일이 맡으려던 일은
모두 저장소 상태에서 다시 계산할 수 있기 때문이다.

- **무엇을 검토할지:** bootstrap 대상은 브랜치 모양으로 정의된다("target 미선언 +
  계약 파일만 바꾼 1커밋"). 그러니 스캔으로 찾을 수 있다.
- **사전 조건:** 읽기 전용 `bootstrap check`로 사람에게 넘기기 전에 판정한다.
- **결과 인계:** 다음 세션 에이전트는 "target이 이제 계약을 선언했는가"를 git에서
  직접 읽는다. 따로 기록한 결과보다 이쪽이 정확하다.

요청 파일이 더해 주는 것은 의도 기록뿐이다. 그 의도는 이미 브랜치 이름과 카드에
있다. 반면 요청 파일을 두면 새 상태 저장소가 생기고, 오래된 요청을 정리하는
관리 비용이 따라온다.

스캔 범위는 기존 bulk 명령의 workspace·`defaults.scan`(depth·exclude) 규칙을 그대로
쓴다. 새 탐색 규칙을 만들지 않는다.

### D2. 확인 방식 — 저장소별 확인이 기본, 묶음 확인은 옵션

- **기본(저장소별):** 묶어서 보여 주되, 확인은 지금처럼 저장소마다 그 plan의
  `CONFIRM_DIGEST`를 입력받는다. 기존 계약
  (`docs/commands/integrate-bootstrap.md`: digest를 읽고 다시 입력하는 것이 검토
  확인)은 그대로다. 개선되는 것은 명령 조립, 경로·plan 파일 입력, 오타다.
- **옵션 `--group-confirm`:** 아래 조건을 모두 만족할 때만 묶음 하나를 한 번에 확인한다.
  - 묶는 기준: `manifest_oid`, `runner_oid`, `readiness_tree_digest`가 **셋 다 같은**
    요청만 한 묶음이 된다.
  - 입력: tree digest 접두(12자 이상)와 **저장소 개수**를 함께 입력받는다. 개수까지
    받는 것은, 나열된 목록을 사람이 실제로 읽었다는 근거로 삼기 위해서다.
  - 실행: apply는 저장소마다 각자의 plan과 `--force-with-lease`로 한다. 하나라도
    drift·만료·실패가 나면 **남은 저장소 전부를 중단**한다. 일부만 적용된 상태를
    자동으로 계속 진행하지 않는다.
  - 기록: 결과 출력에 확인 방식(`confirmed: per-plan` / `confirmed: group(n)`)을 남긴다.
  - 문서: `integrate-bootstrap.md`, `integrate-readiness-update.md`의 확인 정의에 이
    옵션을 명시적 예외로 추가한다. `--yes`와 환경변수 우회는 여전히 없다.

### D3. 에이전트 허용 범위 — 권한 경계는 하위 명령 경계와 같다

허용 여부는 devenv 훅의 효과 표(`_guard_gz_git_effect_policy`)가 정하고, 그 근거는
gz-git이 선언하고 릴리스에 sha256과 함께 고정한 하위 명령별 Effect다. gz-git이 할
일은 **새 하위 명령의 Effect를 정확히 선언하는 것**뿐이다.

- `bootstrap check`와 `approval list`는 `tracking-refs`만 선언한다. 그 이상을 바꾸지 않는다.
- `approval review`는 apply와 같은 Effect를 선언한다. `readiness-contract` 효과는 어떤
  launcher에도 허용돼 있지 않으므로, 선언만 정확하면 훅이 자동으로 거부한다.
- 읽기 전용 동작은 플래그(`review --list-only` 같은 것)로 만들지 않고 별도 하위 명령
  (`approval list`)으로 둔다. 플래그 조합에 따라 권한이 달라지면 훅이 인자를 해석해야
  하고, 거기서 허점이 생긴다.

launcher 추가와 효과 표 갱신은 ce-devenv 이슈 013이 소유한다.

## 범위 밖

- 운영자 인증. TTY와 digest가 운영자를 식별하지 못한다는 기존 문서의 한계는 그대로다.
- 정본 runner 내용 검증. runner가 CE 정본 어댑터와 같은지는 CE(`ce task gate` 문서 22)
  쪽 관심사다. review 화면은 runner diff를 보여 줄 수만 있다.

## 수락 기준 (착수 시 실행 카드로 옮긴다)

- `bootstrap check`가 위 사례 4의 3커밋 브랜치를 이유와 함께 거부하고, 1커밋 브랜치는 통과시킨다.
- `approval list`가 동일 트리 6개를 한 묶음으로 보이고, 같은 저장소의 무관한
  계약 브랜치를 대상 아님으로 표시한다.
- `approval review`가 TTY 없이 거부되고, 기본 모드에서 저장소별 digest 입력 없이 apply하지 않는다.
- `--group-confirm`이 세 OID 중 하나라도 다른 저장소를 묶지 않고, 중간 drift에서 남은 apply를 중단한다.
- 세 하위 명령의 Effect 선언이 위 표와 같고, release Effect 표 테스트가 이를 고정한다.

## 관련

- `docs/commands/integrate-bootstrap.md`, `docs/commands/integrate-readiness-update.md`
- ce-agent-kit `docs/10-ce-cli/22-readiness-runner-adapter.md` (「새 저장소에 적용하기」)
- ce-devenv `config/claude/hooks/lib/guard-gz-git-version.sh` (`_guard_gz_git_effect_policy`)
- 사례: sigdock-pass-devbox `tasks/manual/sigdock-component-readiness-bootstrap.md`
