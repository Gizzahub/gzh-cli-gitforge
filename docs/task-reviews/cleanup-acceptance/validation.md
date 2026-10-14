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
