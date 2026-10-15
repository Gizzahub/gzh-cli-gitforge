# Legacy 작업 자원의 정리 계획

`integrate legacy-cleanup`은 기존 native v2 영수증이 없는 작업에 대해 현재
Git 자원을 조사하고, 별도로 승인한 정리 계획을 처리한다. 과거 실행의
execution ID나 generation을 추정하지 않으며 v1/v2 영수증과 등록 marker를
덮어쓰지 않는다. v2 영수증이 있는 실행은 기존
[cleanup-only 복구](integrate-cleanup-retry.md)를 사용한다.

## 지원 경계

`gz-git capability integrate-legacy-cleanup-standalone-v1`로 standalone 명령 계약을
조회한다. 각 저장소의 lifecycle 검사는 apply에서 별도로 수행한다.

기본 apply는 task-runtime 선언이 없는 standalone 저장소에 한정한다. CE 저장소는
`gz-git capability integrate-legacy-cleanup-ce-v1`와 CE가 발급한
`--runtime-guard`를 함께 사용해야 한다. guard는 task·owner·Git 정체성과 CE
검증 실행 파일의 SHA-256을 계획에 결속한다. apply는 매 mutation 전과 각
자원 상태 확인 전후에 CE verifier의 public attestation을 재검증하며,
`--runtime-attempt` nonce가 없거나 attestation이 바뀌면 자원을 보존한다.

native task lifecycle, native runtime state, CE guard가 없는 CE 선언은 계속
삭제 전에 거부한다. native는 CE의 private ledger나 receipt를 읽거나 변환하지
않는다. 선언 파일이 task worktree에 없더라도 primary의 선언을 확인해야 한다.
이 명령을 lifecycle 밖에서 자원을 정리하는 우회 수단으로 사용하지 않는다.

worktree와 Git directory의 device/inode 정체성을 관찰할 수 없는 호스트는
지원하지 않는다. 경로·파일 크기·수정 시각으로 물리 정체성을 추정하지 않는다.

같은 task의 협조적 native `integrate run`, `integrate cleanup`, legacy apply는
저장소 common directory의 task 잠금을 공유한다. target 잠금은 별도로 유지한다.
이 잠금이 외부 Git writer나 CE의 worktree 생성을 통제한다고 가정하지 않는다.

## 계획과 승인

`plan`은 task worktree에서 대상 integration branch와 DONE·RELEASED 근거를
명시해 실행한다. `--output`, `--done-artifact`, `--released-artifact`는 절대
경로를 사용한다. CE 경로는 CE가 만든 절대 경로 `--runtime-guard`도 명시한다.
기존 파일을 덮어쓰지 않는 계획 파일을 생성하며 Git 자원은
변경하지 않는다. 정확한 인자와 근거 스키마는 명령 도움말과 계획 스키마를
확인한다.

두 근거 파일은 각각 `status`가 `DONE`, `RELEASED`인 다음 구조의 disposition
문서를 받는다. `task_sha`는 전체 commit OID이며 모든 정체성 필드는 실제
조사 대상 및 caller와 일치해야 한다. 이는 사람이 과거 근거를 검토해 별도로
작성하는 prospective disposition이며, 기존 CE 응답이나 native 실행 영수증을
이 스키마로 바꿔 과거 실행의 권한으로 사용하는 기능은 제공하지 않는다.

```json
{
  "schema_version": 1,
  "status": "DONE",
  "task_sha": "<full-commit-oid>",
  "task_branch": "<task-branch>",
  "task_worktree": "<absolute-task-worktree>",
  "common_dir": "<absolute-git-common-dir>",
  "owner": {"actor": "<actor>", "host": "<logical-host>", "kind": "agent"}
}
```

계획은 caller identity, 저장소·worktree·Git directory, task와 target OID,
remote endpoint, 보호 규칙, retired disposition 및 근거의 SHA-256을 결속한다.
해시는 근거가 이후 바뀌지 않았음을 검사하는 값이며 근거의 진위를 증명하는
서명이 아니다. 과거 기록과 대상의 일치 여부를 검토한 뒤 정확한
`plan_digest`를 `apply --plan <절대 경로> --approve-digest <digest>`에 전달한다.

## 정리와 재개

apply는 모든 잔여 자원의 정체성과 원격 target의 task 포함 관계를 삭제 전에
확인한다. dirty·locked·foreign worktree, 바뀐 ref나 endpoint, nested repository,
sibling lease, 알 수 없는 등록 상태는 보존한다. local ref는 expected-old CAS,
remote ref는 승인한 endpoint와 정확한 OID lease로 삭제한다.

삭제 전 durable intent를 기록하고 단계별 결과와 실제 부재를 확인한다.
중단 후 같은 계획을 재개할 때는 승인한 자원의 감소만 허용한다. 정리가
끝난 뒤 다시 나타난 자원은 재삭제하지 않는다. readiness·gate·merge·target
push를 다시 수행하지 않는다.
