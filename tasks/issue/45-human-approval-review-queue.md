# ISSUE: 사람 전용 readiness 승인이 저장소마다 수작업 명령 조립에 의존한다

- status: open (설계 제안 — 착수 전 아래 「결정할 것」을 먼저 정한다)
- priority: P2
- category: ux/integrate
- created_at: 2026-10-06
- owner: `gzh-cli-gitforge/cmd/gz-git` (`integrate bootstrap`, `integrate readiness update`)
- work-card: 없음 — 착수 시 devbox 루트에 실행 카드를 만든다

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

## 제안

에이전트는 **승인 요청**만 기록하고, 사람은 **한 명령**으로 대기 중인 요청을 검토·적용한다.

```text
에이전트                                     사람 (TTY 전용)
gz-git integrate approval request            gz-git integrate approval review
  --kind bootstrap|readiness-update            · 대기 요청을 나열한다
  --branch <source> --target origin/<b>        · 요청마다 그 자리에서 plan을 새로 만든다
  → 요청 파일만 쓴다                            · tree/runner digest가 같은 요청끼리 묶어 보여 준다
    (plan·digest는 담지 않는다)                 · 확인을 받은 뒤 apply를 실행한다
  → 사전 조건을 이 자리에서 검사한다            · 결과(digest·SHA·시각·issuer)를 요청에 기록한다
```

- **request는 승인 재료를 담지 않는다.** 저장소 URL·경로, source/target ref, kind,
  요청 주체, 생성 시각만 기록한다. plan은 review 시점에 만들므로 TTL 문제가 없다.
  에이전트가 digest를 미리 갖고 있을 수도 없다.
- **request는 bootstrap의 사전 조건을 즉시 검사한다.** 1커밋 fast-forward 여부,
  변경 경로가 `.gz-git.yaml`과 `.gz-git/readiness/` 아래뿐인지, target이 아직 계약을
  선언하지 않았는지를 확인한다. 위 증상 4처럼 사람 앞에 와서야 실패하는 일을 없앤다.
  이 검사는 원격 ref를 바꾸지 않으므로 에이전트에게 허용할 수 있다.
- **review는 지금의 apply 경계를 그대로 지킨다.** TTY가 없으면 거부하고, `--yes`나
  환경변수로 우회할 길을 두지 않는다. apply는 요청마다 각자의 plan과
  `--force-with-lease`로 실행한다.
- **결과 기록은 후속 작업이 읽는다.** 다음 세션 에이전트가 이 기록을 보고 bootstrap
  뒤의 일반 통합(예: `ce-tasks.yaml` 선언 브랜치)을 이어서 처리한다.

## 결정할 것

1. **묶음 확인을 허용할지.** `docs/commands/integrate-bootstrap.md`는 사람이 plan
   digest를 읽고 **다시 입력하는 것**을 검토 확인으로 정의하고, plan과 apply를 이어
   붙일 수 없게 만들었다. 묶음 확인(예: 같은 `readiness_tree_digest`를 공유하는 N개
   요청에 대해 tree digest 접두를 한 번 입력)은 이 확인 단위를 "plan 1개"에서 "runner
   트리 1개 + 저장소 목록"으로 바꾸는 계약 변경이다. 대안은 둘이다.
   - (a) 묶어서 **보여 주기만** 하고, 확인은 지금처럼 요청마다 digest를 입력받는다.
     안전성은 그대로이고, 개선되는 것은 명령 조립과 오타뿐이다.
   - (b) 묶음 확인을 도입한다. 문서 계약과 하류 훅 정책을 함께 개정해야 한다.
2. **요청 저장 위치.** 여러 저장소를 한 번에 review하려면 저장소 밖 상태
   디렉터리(`${XDG_STATE_HOME:-~/.local/state}/gz-git/approvals/`)가 맞다. 저장소별
   `.git/` 아래에 두면 탐색 범위를 따로 정해야 한다.
3. **에이전트 허용 범위.** `approval request`(와 목록 조회)만 허용하고 `review`는
   계속 거부하려면, 하류 훅 허용 목록을 바꿔야 한다(ce-agent-kit/devenv 소유).
   gz-git은 해당 하위 명령의 `Effect` 표기만 정확히 제공한다.

## 범위 밖

- 운영자 인증. TTY와 digest가 운영자를 식별하지 못한다는 기존 문서의 한계는 그대로다.
- 정본 runner 내용 검증. runner가 CE 정본 어댑터와 같은지는 CE(`ce task gate` 문서 22)
  쪽 관심사다. review 화면은 runner diff를 보여 줄 수만 있다.

## 관련

- `docs/commands/integrate-bootstrap.md`, `docs/commands/integrate-readiness-update.md`
- ce-agent-kit `docs/10-ce-cli/22-readiness-runner-adapter.md` (「새 저장소에 적용하기」)
- 사례: sigdock-pass-devbox `tasks/manual/sigdock-component-readiness-bootstrap.md`
