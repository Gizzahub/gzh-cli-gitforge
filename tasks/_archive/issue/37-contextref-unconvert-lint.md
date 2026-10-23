# ISSUE: contextref의 Unix device ID 변환이 Linux lint에서 unconvert로 보고된다

- status: fixed
- priority: P2
- category: quality/ci
- created_at: 2026-09-28
- owner: `gzh-cli-gitforge/pkg/contextref`
- work-card: `gzh-cli-devbox/tasks/doing/238-contextref-unconvert-lint.md`

## 증상과 원인

`pkg/contextref/open_unix.go`의 `identFromStat`은 `fileIdent.dev`에 device ID를
`uint64`로 보관한다. Linux의 `unix.Stat_t.Dev`도 `uint64`이므로 Linux
`unconvert` lint는 `uint64(st.Dev)`를 중복 변환으로 보고 CI를 실패시킨다.

그러나 같은 Unix 소스는 Darwin에서도 빌드되며, Darwin의 `unix.Stat_t.Dev`는
`int32`이다. 변환을 Linux 경고만 보고 삭제하면 Darwin 빌드가 타입 불일치로
실패한다.

## 해결

변환을 유지하고 해당 줄에 `unconvert` 억제를 둔다. 억제 설명은 Darwin에서
필요한 변환임을 명시하며, 기존 G115 억제도 device ID가 불투명한 kernel
identifier라는 근거와 함께 유지한다.

## 검증

Darwin에서 `GOWORK=off go test ./pkg/contextref`, Linux 대상에서
`GOOS=linux GOARCH=amd64 GOWORK=off go build ./pkg/contextref`, 그리고 Linux
대상 `golangci-lint`의 `unconvert` 검사를 통과해야 한다. 구현과 보드 완료
판정은 devbox TASK-238에서 추적한다.
