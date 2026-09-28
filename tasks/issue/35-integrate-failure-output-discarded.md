# ISSUE: integrate 실패 출력이 판정 과정에서 유실된다

- status: fixed
- priority: P1
- category: product/diagnostics
- created_at: 2026-09-28
- owner: `gzh-cli-gitforge/pkg/integrate`
- work-card: `gzh-cli-devbox/tasks/doing/236-integrate-failure-diagnostics.md`

## 증상과 원인

`integrate check`의 legacy make 경로는 branch probe의 stdout/stderr를 캡처하지만
실패 판정 문자열에 결과를 포함하지 않는다. 특히 target baseline이 통과하고 branch
probe가 실패하면 `failed here (rc=...) but target tip passes`만 남는다.
target-owned readiness contract 경로도 `not_ready`·`unavailable` JSON과 함께
출력된 stderr를 버린다. 임시 worktree가 정리되면 실제 실패 원인을 찾을 수 없다.

## 해결 기준

두 경로의 실패 출력을 사용자 전용 로컬 상태 디렉터리에 보존하고 실패 항목에
파일 경로를 보여준다. 성공 출력은 저장하지 않으며 readiness `json-v1` 응답
형식은 유지한다. 코드 검증과 보드 판정은 TASK-236에서 추적한다.

## 검증

`make quality-check` 통과, `GOOS=windows GOARCH=amd64 go test -c ./pkg/integrate` 교차 컴파일 통과. 별도 독립 코드 리뷰에서 Windows 권한
보장 문제를 발견해 Windows 저장 거부로 수정한 뒤 PASS를 받았다. 소스 통합
리비전과 보드 완료 판정은 devbox TASK-236에 기록한다.
