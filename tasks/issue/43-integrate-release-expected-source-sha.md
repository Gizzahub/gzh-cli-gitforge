# ISSUE: `integrate check/run --release`가 기대 source SHA를 받지 않아 승인 범위를 프로세스 안에서 고정할 수 없다

- status: open
- priority: P2
- category: feature/integrate
- created_at: 2026-10-04
- owner: `gzh-cli-gitforge/pkg/integrate`
- work-card: 없음 — gzh-cli-devbox 큐 발급 대기
- 요청처: ce-agent-kit `ce task run-release` (master `f6493563`, `docs/10-ce-cli/34-release-promotion.md`)

## 증상

CE `ce task run-release`는 사용자가 승인한 record의 `source-sha`(정확한 develop 커밋)만
`develop → master`로 승격해야 한다. CE는 다음 순서로 호출한다.

1. `gz-git integrate check origin/<source> --target origin/<default> --release`
1. `refs/remotes/origin/<source>`가 `source-sha`인지 CE가 확인
1. `gz-git integrate run origin/<source> --target origin/<default> --release --no-fetch`

`run`은 내부에서 `Check`를 다시 실행하고(`pkg/integrate/run.go:37`), source SHA를
`g.revParse(ctx, branch)`로 새로 읽는다(`pkg/integrate/check_target.go:51`). 2와 3 사이에
CE 밖에서 fetch(IDE 자동 fetch 등)가 일어나 `origin/<source>`가 승인되지 않은 후손으로
움직이면 문제가 생긴다. `run`은 그 새 SHA를 일관되게 검사하고 승격한다.
`revalidateCheckedRefs`(`run.go:106`)는 같은 `run` 프로세스 안의 변화만 막고,
호출자가 승인한 SHA와는 비교하지 않는다. CE는 사후에 target이 `source-sha`가 아니면
exit 3을 내지만, 그때는 이미 push된 뒤다.

## 요청

`integrate check`와 `integrate run`에 `--expect-source <full-sha>`를 추가한다.

- 비교는 fetch 뒤, readiness 전에 한다. 지금 `resolveTarget`은 fetch(`check_target.go:72`)
  전에 source를 읽어(`:51`) `plan.BranchSHA`를 한 번만 정하고(`:58`), readiness는 그 값을
  검사한다. source 해석을 `planFetchDefault` 뒤로 옮기거나 fetch 뒤 다시 읽어
  `plan.BranchSHA`를 갱신한 다음 비교해서, 비교한 SHA와 readiness가 검사한 SHA가 같게 한다.
  비교용으로만 다시 읽으면 `check`가 일치를 보고하면서 fetch 전 커밋을 검사할 수 있다.
- 불일치는 전용 sentinel error(`ErrImplicitSourceIsTarget`처럼 `errors.Is`로 판별)로 낸다.
  지금 exit 1은 메시지 문자열 매칭(`integrate_check.go:83`, `integrate_run.go:91`)이라
  새 메시지는 exit 2로 떨어진다. exit 코드는 NOT READY(1)·사용 오류(2)와 구분되는
  전용 값을 권장한다. CE가 "승인되지 않은 source"와 "gate 실패"를 stderr 없이 구분해야 한다.
  메시지 예: `source <branch> is <sha>, expected <sha>`.
- `run`의 `revalidateCheckedRefs`도 같은 기대값과 비교한다(중복 방어).
- 축약·형식 오류 SHA는 사용 오류(exit 2)로 거부한다(40 또는 64자리 hex). 대문자 입력은
  `rev-parse` 출력과 맞도록 소문자로 정규화하거나 거부한다 — 어느 쪽인지 명시한다.
  `--release` 외 모드에서도 의미가 같으므로 모드에 묶지 않는다.
- `--help`에 플래그를 선언한다. CE는 `declaresFlag`로 capability를 탐지하고, 있을 때만 넘긴다.
  `no-fetch` 도입 때와 같은 방식이라 gz-git과 CE를 동시에 배포할 필요가 없다.

## 수용 기준

- 기대값과 다른 source에서 `check`/`run`이 push 없이 전용 exit 코드로 끝나는 테스트
- fetch로 tracking ref가 기대값까지 전진하는 경우 `check`가 통과하고 `report.Plan.BranchSHA`가 기대값인 테스트
- 기대값이 같으면 기존 동작과 동일한 테스트
- 축약·잘못된 SHA가 exit 2로 거부되는 테스트
- `gz-git integrate run --help`와 `docs/commands/`(선례: `integrate-no-fetch.md`)에 플래그가 나타남

## CE 후속

플래그가 배포되면 ce-agent-kit에서 `integrate-expect-source` capability를 탐지해
`run-release`가 `--expect-source <source-sha>`를 넘기게 하고, `34-release-promotion.md`의
잔여 race 단락을 지운다.
