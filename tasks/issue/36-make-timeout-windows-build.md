# ISSUE: make timeout process-group 변경이 Windows 빌드를 깨뜨린다

- status: fixing (Windows hosted execution pending)
- priority: P1
- category: product/cross-platform
- created_at: 2026-09-28
- owner: `gzh-cli-gitforge/pkg/integrate`
- work-card: `gzh-cli-devbox/tasks/doing/237-make-timeout-windows-build.md`

## 증상과 원인

`integrate check`의 make target timeout은 Unix에서 recipe 자식 프로세스까지
종료하려고 `syscall.SysProcAttr.Setpgid`와 음수 PID `syscall.Kill`을 사용한다.
이 Unix 전용 API가 `check_make.go`에 직접 들어가 Windows 교차 컴파일이 실패한다.

## 해결 기준

make 프로세스 설정과 종료를 OS별 파일로 분리한다. Unix는 전용 process group을
만들고 SIGKILL로 그 group을 종료하는 기존 동작을 유지한다. Windows는 Job Object에
make를 연결하고 취소 또는 close 때 job 전체를 종료해 recipe 자식까지 정리한다.
Windows의 make 실행은 Job 연결 전에는 batch handshake에서 대기하므로, 연결 실패가
이미 생성된 recipe 자식을 남기지 않는다.
batch는 상대 파일명으로만 시작하며 `%`는 이스케이프하고 delayed expansion을 끈다.
따라서 TEMP 경로의 공백, `%`, `!`가 launch gate를 바꾸지 않는다.

## 검증

대상 패키지 테스트와 `GOOS=windows GOARCH=amd64 go test -c ./pkg/integrate` 교차
컴파일로 검증한다. 소스 통합 리비전과 보드 완료 판정은 devbox TASK-237에서
기록한다.
