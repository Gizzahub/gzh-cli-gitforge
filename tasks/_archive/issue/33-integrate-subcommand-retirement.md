# ISSUE: `integrate` 서브커맨드 퇴역 — 소비자가 Worktrunk 엔진 + ce 전략의 2층으로 옮긴다

- status: superseded
- priority: P2
- category: product/scope
- created_at: 2026-09-07T13:30:00+09:00
- affects: `cmd/gz-git/cmd` 의 `integrate` 진입점, `pkg/integrate` (비테스트 5,895줄, 테스트 5,217줄)
- findings: `single-repo-lifecycle-inside-a-bulk-first-cli`,
  `readiness-gate-lived-outside-the-binary-it-judged`
- related: devenv `docs/70-tech-debt/TD-100.md`, ce-agent-kit `tasks/plan/001-*.md`

## Pointer

2026-09-28 정정: devbox ADR-0026이 이 퇴역 전제를 기각했고 devbox 보드는
TASK-188–192 퇴역 체인을 취소했다. 설치된 launcher와 CE task runtime은
현재 `gz-git integrate`를 계속 사용한다. 따라서 아래 내용은 당시 제안의
기록이며 현재 구현 금지나 동결 정책이 아니다. 후속 결함은 ISSUE-35와
devbox TASK-236으로 추적한다.

작업 항목은 devbox 루트가 소유한다 →
`gzh-cli-devbox/tasks/todo/176-retire-gz-git-integrate-subcommand.md`.
이 파일은 이 저장소가 소유하는 결정 기록이다.

## Background

`integrate` 는 저장소 하나의 통합·회수를 맡는데 gz-git 의 자기 정의는 bulk-first
다중 저장소 CLI 다. 소비자(devenv, ce-agent-kit)가 2026-09-07 에 통합 경로를
Worktrunk 엔진 + ce 전략의 2층으로 옮기기로 결정해 소비자가 없어진다. 3주 운영에서
소비자 측 부채 셋(devenv TD-36·TD-53·TD-99)을 냈고, 셋 다 「판정이 판정 대상 바이너리
바깥에 있다」 는 같은 뿌리였다.

## Scope boundary

`.gz-git.yaml` 의 `branch` 절은 남는다. `cleanup branch --non-canonical`, `info --audit`,
`pkg/config` 의 integration participation 이 함께 읽는다. 퇴역 범위는 `pkg/integrate`
와 그 CLI 진입점뿐이며, 코드는 한 릴리스 동안 동결(빌드 제외, 트리 유지)한다 —
Worktrunk 철수 기준이 발동하면 자작 엔진의 출발점이다.
