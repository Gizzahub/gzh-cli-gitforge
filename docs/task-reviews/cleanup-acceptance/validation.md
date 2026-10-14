# TASK-302 cleanup retry safety

## 2026-10-14 승인된 bounded successor 계약

실행은 `task302-cleanup-contract-successor`, 기준 source는
`614e47de0c64de15cc1150fc6a90634675c6d148`이다. 사용자는 계획 구현을 승인했다.
구현과 독립 리뷰의 요구 등급은 strong이며 실제 source 리뷰 FAIL은 최대 3회다.
판정 없는 실행 오류는 별도 기록한다. 기존 ISSUE-324의 7회 FAIL과 ABORTED
`e8e0366`은 그대로 보존한다. 이 successor는 native C1–C5 수용까지이며
TASK-095, 전역 설치와 릴리스는 포함하지 않는다. 아래 과거 회귀 기록은
이번 후보의 검증 증거를 대신하지 않는다.

| 조건        | 정상 회수와 cleanup retry의 공통 계약                                                                                 |
| ----------- | --------------------------------------------------------------------------------------------------------------------- |
| 실행 정체성 | 조회 키·common-dir·captured Git 정체성과 endpoint·보호 정보는 불변이다. 잠금 뒤 재조회가 달라지면 삭제 전에 거부한다. |
| 삭제 권한   | 안전한 linked worktree, task pattern, 보호 ref, captured OID와 remote containment를 먼저 증명한다.                    |
| 삭제        | local expected-old CAS와 remote exact lease를 사용한다. caller 소유·이동·미확인 자원은 보존한다.                      |
| 완료        | 마지막 변경 뒤 worktree·local ref·remote ref 부재를 새로 확인한다. 조회 오류는 완료가 아니다.                         |
| 재등장      | complete 영수증의 자원이 다시 나타나면 재삭제하지 않는다. 최신 세대 권한은 TaskManager가 소유한다.                    |
| 저장 오류   | rename 뒤 sync 실패도 가능하다. 다음 호출은 실제 영수증과 Git을 다시 읽고 target push를 반복하지 않는다.              |

| 저장 상태             | 허용 동작                                                                      |
| --------------------- | ------------------------------------------------------------------------------ |
| v1                    | 조회만 지원한다. cleanup·실행 재사용·자동 변환을 거부한다.                     |
| v2 intent             | 같은 불변 실행만 재개한다. 실제 push가 확인된 이후에는 push를 반복하지 않는다. |
| v2 pushed / partial   | 잠금 아래 실제 Git 사실을 다시 증명하고 남은 자원만 회수한다.                  |
| v2 complete           | 현재 부재를 확인하면 무변경 성공한다. 재등장한 자원은 보존하고 거부한다.       |
| 손상·미지원·정보 부족 | 원인을 표시하고 수정 전에 거부한다.                                            |

durable receipt writer는 v2를 사용한다. 기존 저장 위치, run JSON v1과
worktree 등록 marker v1은 유지하며 영수증 버전과 분리한다. 외부 Git writer를
로컬 잠금으로 동결할 수는 없으므로 파괴적 작업 직전에 재확인하고, 불확실하면
부분 결과와 잔여 자원을 보존한다. 구버전 binary는 새 영수증을 처리할 수 없으며
과거 권한 정보를 현재 설정에서 추정해 보완하지 않는다.

Cleanup-only retry의 local ref 삭제는 captured `TaskSHA`를 expected-old로 묶은
`git update-ref -d --no-deref refs/heads/<task> <TaskSHA>`를 사용한다. 사전 검증과 삭제
사이에 ref가 바뀌면 삭제를 거부하고 남은 local/remote ref를 보존한다.
이전 `git branch -d`는 이름 기준 삭제여서 검증 이후의 이동을 막지 못했다.
remote 삭제는 기존 exact `--force-with-lease=<ref>:<TaskSHA>`를 유지한다.

삭제 후 영수증 기록이 실패하면 원래 오류를 보존하면서 `ErrCleanupPartial`
(exit 3)로 보고한다. durable 영수증을 성공한 것처럼 전진시키지 않는다.
후속 cleanup-only 호출은 기존 영수증과 실제 Git 부재를 다시 증명해 남은
자원만 회수한다. provider, readiness, merge와 target push는 반복하지 않는다.

`pkg/integrate/cleanup_retry_test.go`는 실제 bare origin과 linked worktree로
검증 직후 local/remote ref 이동, 삭제 후 영수증 쓰기 실패, 실제 partial 뒤
재시도, 잘못된 ref/task pattern 거부와 동일 target의 두 cleanup 직렬화를
검증한다. 장애 주입은 테스트 executor의 상수 Bash 프로그램에만 존재하며
production callback이나 별도 엔진을 추가하지 않는다.

수정 전 local CAS 및 영수증 실패 테스트는 각각 잘못된 완료와 일반 오류를
재현했다. 수정 후 안전 회귀와 동시성 race 검사가 통과했다. 최종 수용은
동일 source commit/tree에 묶은 canonical quality exit 0, 여덟 guard의
RED/restored GREEN, 실제 독립 strong 리뷰와 native 통합·push·회수 기록을
owner devbox의 `docs/task-302-acceptance.json`에서 확인한다. 이 문서 자체는
TASK-302 완료 영수증이 아니다.

## Native Git 회수 JSON

기존 context `integrate run --json`은 선택 필드 `git_reclaim`에 Git-only
회수 증거를 반환한다. 기존 target lock 안에서 삭제 전 실제 common-dir,
task/ref OID, worktree inventory와 safety 조건을 검증하고, 한 번 조회한
live target OID에 대한 containment를 기록한다. 삭제 후에는 성공한 Git
조회와 실제 경로 부재로 결과를 다시 확인한다. caller가 이 JSON을 저장하며
별도 ledger나 owner/generation 권한 엔진을 추가하지 않는다.

이 증거 경로는 linked task worktree의 회수만 수행한다. nested Git repository나
task에 묶인 provisioned sibling lease가 있으면 삭제 전에 거부하고 caller가
해당 자원을 처리하도록 보존한다. caller 소유 parent directory도 삭제하지
않는다. 기존 legacy 경로는 유지하며, network read를 금지한 `--no-fetch`에서는
live Git proof 필드를 생략한다. 빈 객체나 stale tracking ref를 성공 증거로
내보내지 않는다. normal reclaim의 local 삭제도 captured OID의 atomic CAS다.

독립 리뷰에서 bare repository, 조회 실패, tracking ref 부재의 세 경계 오류를
확인했다. 각각 회귀 검사가 기존 코드에서 실제 assertion RED를 재현했다.
bare 저장소는 `.git` 이름 없이 `HEAD`·`objects`·`refs`를 가지므로 이 layout도
삭제 전 보존한다. local branch 존재 여부는 성공한 `for-each-ref`의 정확한
ref/OID로만 판정하며 실패한 조회를 부재로 바꾸지 않는다. live remote proof가
있는 회수는 local tracking ref 없이도 captured OID의 exact lease로 삭제한다.
수정 후 세 검사와 기존 사후 조회·nested repository 검사에 race 검증을 적용한다.
이는 source 변경 근거이며 공식 C1–C5 수용이나 전달 완료 주장이 아니다.

같은 보존 규칙은 cleanup-only retry에도 적용한다. 정상 회수가 거부한 bare
저장소와 sibling lease를 retry가 삭제하던 경로를 회귀 검사로 재현했다.
기록된 task worktree가 다른 경로로 이동했거나 잠겨 있으면 삭제를 거부한다.
등록되지 않은 물리 경로도 부재로 처리하지 않는다. local atomic CAS 직전에는
task branch를 사용하는 worktree가 없는지 다시 확인한다. moved-worktree 및
정상 거부 후 retry 검사도 실제 RED 후 수정된 race GREEN으로 확인한다.

symbolic task ref가 삭제 직전에 보호 branch를 가리키도록 바뀌면 `update-ref`
기본 dereference가 그 대상까지 삭제할 수 있었다. retry와 normal 경로의
실제 RED를 확인해 direct ref 검사를 추가하고 atomic 삭제에 `--no-deref`를
적용했다. fetch와 push 주소가 다른 remote도 재현했다. context 실행은 기존
push endpoint resolver로 단일 실제 주소를 통합 전에 확정하고 기존 receipt의
`PushRemote`에 기록한다. push, 삭제 전 containment, remote 삭제와 사후 부재
조회가 같은 주소를 사용하며 Git-only JSON의 observed target도 그 endpoint를
내보낸다. 여러 push 주소는 intent/push 전에 거부하고 기존 자원을 보존한다.

plain relative filesystem remote는 최초 push 실행 디렉터리의 physical 주소로
확정해 저장한다. 다른 checkout parent와 같은 task/target OID를 가진 별도
bare 저장소를 준비하자 기존 normal/retry가 그 저장소를 삭제하고 실제 push
저장소를 남기는 assertion RED가 재현됐다. 절대 주소 binding 후에는 실제
push 저장소만 회수하고 caller의 별도 저장소를 보존한다. 이미 저장된 relative
주소나 상대 `file:` URI는 원래 실행 디렉터리를 증명할 수 없으면 거부하며
현재 caller 디렉터리로 재해석하지 않는다. 기존 absolute/SSH/HTTPS 주소의
textual identity는 유지한다.

`./store@host:repo.git`처럼 colon 앞에 slash가 있는 이름은 Git에서 로컬
경로다. 단순 `@` 검사로 SSH로 분류해 relative binding을 건너뛰던 우회도
normal/retry의 실제 RED로 재현했다. sanitizer와 endpoint binding이 기존
공통 SCP 분류 helper를 함께 사용하도록 수정했고 같은 spelling의 race
GREEN 및 `internal/gitcmd` 전체 race 회귀 검사를 확인했다.

symlink 뒤의 `..`를 lexical 정리하면 실제 Git filesystem 해석과 다른
저장소를 선택할 수 있다. 이 경우도 같은 refs를 가진 decoy로 실제 잘못된
target push를 재현했다. relative 구성 요소를 physical resolution 전에
보존하도록 수정하고 configured target만 변경되는 race 회귀 검사를 추가했다.

relative binding이 만든 최종 absolute 주소에 적용되는 URL rewrite도 검사한다.
기존 검사는 raw relative spelling만 확인해 decoy 주소로 실제 push와 retry가
돌아갈 수 있었다. 두 경로의 RED를 확인해 최종 주소와 저장된 explicit 주소에
호출 context의 rewrite 검사를 적용했다. config 조회 실패도 빈 설정으로
처리하지 않으며 rewrite가 있으면 intent/push 또는 retry 삭제 전에 거부한다.

controller가 존재하면 빈 task pattern도 그대로 적용해 retry의 declaration
fallback을 막는다. 실행 당시의 resolved default·integration branch 이름은
기존 receipt provenance의 `reclaim_refs`에 기록하고, retry도 정상 회수의
동일 ref guard를 사용한다. 이 정보가 없는 과거 receipt는 삭제를 거부하고
잔여 자원을 보존한다. 별도 권한·세대 ledger를 추가하지 않는다.
provenance도 기존 immutable receipt 비교에 포함한다. target lock을 기다리는
동안 pattern이나 branch 보호 정보가 바뀌면 실제 회수 전에 replay로 거부한다.

빈 URL rewrite prefix는 Git에서 모든 주소와 일치한다. 실제 Git의 주소
변환과 기존 검사의 assertion RED를 확인했고, 빈 값과 공백을 포함한
prefix를 보존해 검사하도록 수정했다. 이 변경도 최종 수용 증거와 별개다.
