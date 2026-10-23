# ISSUE: hosted gosec가 진단 디렉터리 권한 설정을 G302로 보고한다

- status: fixed (2026-09-28; product `ece25ab`)
- priority: P1
- category: quality/ci
- created_at: 2026-09-28
- owner: `gzh-cli-gitforge/pkg/integrate`
- work-card: `gzh-cli-devbox/tasks/doing/239-hosted-gosec-diagnostic-directory.md`

## 증상과 원인

`33a3808`의 hosted CI `36407624658`에서 `security-code`가
`diagnostics.go:51`의 `os.Chmod(dir, 0o700)`에 G302를 보고했다. 로컬
`make quality-check`는 통과했다. 기존 `if` 초기화 절 뒤의 `#nosec`은
hosted gosec가 지적한 호출 줄에 연결되지 않는다. 이 저장소의 ISSUE-29에서도
같은 주석 결합 차이가 확인됐다.

## 해결 및 검증

`os.Chmod`를 별도 대입문으로 빼서 제한된 `G302 G703` 억제가 호출 줄에
결합되도록 한다. 디렉터리 권한 `0700`과 파일 권한 `0600`은 유지한다.
로컬 `make quality-check`와 `GOWORK=off gosec ./pkg/integrate/...`가 통과했다.
정확한 SHA `ece25ab`의 hosted CI `36408902610`에서 `Quality gate`가
성공해 `security-code` 단계도 통과했다. 전체 워크플로는 TASK-237의 선행
Windows 빌드 오류 때문에 실패했으며, TASK-239 변경은 그 오류 경로를
건드리지 않는다. devbox TASK-239에서 완료를 판정한다.
