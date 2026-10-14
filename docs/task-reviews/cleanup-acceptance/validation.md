# TASK-302 cleanup retry safety

Cleanup-only retry의 local ref 삭제는 captured `TaskSHA`를 expected-old로 묶은
`git update-ref -d refs/heads/<task> <TaskSHA>`를 사용한다. 사전 검증과 삭제
사이에 ref가 바뀌면 삭제를 거부하고 남은 local/remote ref를 보존한다.
이전 `git branch -d`는 이름 기준 삭제여서 검증 이후의 이동을 막지 못했다.
remote 삭제는 기존 exact `--force-with-lease=<ref>:<TaskSHA>`를 유지한다.

삭제 후 영수증 기록이 실패하면 원래 오류를 보존하면서 `ErrCleanupPartial`
(exit 3)로 보고한다. durable 영수증을 성공한 것처럼 전진시키지 않는다.
후속 cleanup-only 호출은 기존 영수증과 실제 Git 부재를 다시 증명해 남은
자원만 회수한다. provider, readiness, merge와 target push는 반복하지 않는다.

`pkg/integrate/cleanup_retry_safety_test.go`는 실제 bare origin과 linked worktree로
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
