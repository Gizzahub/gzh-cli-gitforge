# Readiness cleanup 안전성 수정

## 범위와 근거

2026-10-18 사용자 승인 범위는 진단 보강·안전한 정리·회귀 테스트·독립 리뷰다.
설치·릴리스 게시·CalcGlide 통합 재시도는 제외한다. 구현 요구 등급 standard,
독립 안전성 리뷰 strong이다. 실행 도구나 모델 지정은 별도 런타임 호출 승인이 아니다.

소스 기준은 `9174d8e6`이다. Singlepageweb DevBox ISSUE-061에서 제품 검사는 통과했으나
readiness 임시 worktree 제거 실패가 통합을 차단했다. 기존 stderr가 비어 있어 그 사례의
Git 실패 원인은 미확정이다. 이번 수정으로 해당 운영 사례가 해결됐다고 주장하지 않는다.

확인된 소스 결함은 Git 제거가 실패한 뒤 `os.RemoveAll(dir)`와 바깥 root 제거가 실행돼
파일을 지우면서 Git 등록은 남길 수 있다는 점이다. 실패 상태와 재현 자료를 보존해야 한다.

## 채택한 경계

- 제거 직전에 생성 시점의 예상 SHA·detached 상태·정확 경로·같은 Git common-dir를 검증한다.
- 등록되지 않았거나 교체된 경로, symlink, 다른 저장소 자원은 삭제하지 않는다.
- Git 실패는 경로·종료 코드·제한된 stderr·취소/시간 초과 원인을 보존한다.
- 실패 시 해당 디렉터리와 전체 임시 root를 보존하고 readiness는 unavailable로 판정한다.
- 자동 재시도와 lockfile 삭제·전체 prune·새 cleanup CLI는 추가하지 않는다.
  같은 경로의 명시적 재시도도 신원 검증을 다시 통과해야 한다.
- 기존 성공 경로의 정상 정리를 유지한다. patch 0.10.5는 소스 준비 버전이며 게시본이 아니다.

## 검증 계획

성공, 영구 실패·빈 stderr, 취소·시간 초과, 다른 HEAD/등록/저장소/심볼릭 링크,
실패 뒤 재시도와 무관한 stale metadata 보존을 synthetic Git 저장소에서 검사한다.
실제 사용자 worktree에 장애를 주입하거나 정리하지 않는다.

소스의 `pkg/integrate` 변경은 commit profile이 full로 확대된다.
`VERIFY_BASE=origin/master make verify-commit`의 확대 판정을 보존하고,
정식 CE integrator가 실행하는 full gate를 최종 전달 판정으로 사용한다.
좁은 진단 통과를 full PASS로 대신하지 않는다.

## 실행 결과

좁은 진단 명령:

```sh
go test ./pkg/integrate -run 'Test(WorktreeRemoveForce.*|ReadinessCleanup.*|ReadinessWorktreeIdentity.*|RemoveEmptyReadinessRoot.*|MergeCleanupError.*|CaptureReadinessWorktree.*|RunContractTimeoutCleansDetachedWorktrees)$' -count=1 -timeout=3m
```

전체 실행 경로의 ready 러너 뒤 제거 실패 주입, 원본 디렉터리·등록·root 보존,
실제 common-dir 교체, 명시적 재시도, 취소·deadline·실행기 실패 원인을 포함해
exit 0(22.075s)을 확인했다. 실제 사용자 저장소는 사용하지 않았다.
`VERIFY_BASE=origin/master make verify-commit`은 예상대로 full 확대를 반환했다
(profile exit 3, Make exit 2). 좁은 프로파일 PASS로 기록하지 않는다.

구현자는 Codex main과 별도 ce-worker이며, 권한·삭제 경계 검토는 별도 ce-judge가 수행한다.
독립 ce-judge의 최종 안전성·아키텍처 판정은 **PASS**다. 같은 좁은 진단을 별도로
재실행해 exit 0(14.621s)과 `git diff --check`를 확인했다. 자동 재시도를 넣지 않는
결정과 실패 시 자료 보존, 실제 사례의 원인 미확정 경계를 함께 검토했다.
설치본 및 과거 실패 자원은 변경하지 않았다. 이 문서는 commit 전 검증 기록이며
최종 native full·통합·push·회수 판정은 해당 CE 실행 영수증과 실제 ref로 확인한다.


## 첫 native 전달 시도

`70cc360aef22a9b93a8780af586364a18eff095b` 전달은 2026-10-18T16:02:25Z에
BLOCKED로 종료했다. 후보 `make check`는 formatting을 통과한 뒤 오류 타입명·변수명과
두 함수의 cognitive complexity, 총 4개 lint 오류에서 종료했다. baseline 측 검사는
FIFO resource admission 시간 초과로 측정되지 않았다. 따라서 full PASS가 아니며
source push·worktree/branch 회수도 수행되지 않았다.

진단 원본은 `~/.local/state/gz-git/integrate/diagnostics/make-check-1010465594.log`이며,
CE task `readiness-cleanup-safety`의 BLOCKED 영수증이 전달 결과를 보존한다.
린트 오류는 이름 수정과 신원 검증·정리 결과 처리 함수 추출로 바로잡는다.
이름 변경과 함수 분리는 삭제 권한·검증 순서·실패 보존·자동 재시도 정책을 바꾸지 않는다.

린트 보정 후 동일 회귀 명령 exit 0(39.570s), 전체 `golangci-lint run`
exit 0(`0 issues.`), 독립 strong 재리뷰 PASS와 diff check 통과를 확인했다.
최종 보정은 `(bool, error)` 결과 순서를 쓰며 보존 여부는 기존 값과 OR한다.
commit profile은 여전히 full로 확대되며 최종 native 전달 결과를 대신하지 않는다.
