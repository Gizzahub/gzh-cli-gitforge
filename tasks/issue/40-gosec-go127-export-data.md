# ISSUE: go1.27 툴체인으로 빌드한 gosec가 export data를 읽지 못한다

- status: resolved (2026-10-02, 중앙 보드 TASK-270으로 핀 v2.29.0 해결)
- priority: P1
- category: quality/ci
- created_at: 2026-10-01
- owner: `gzh-cli-gitforge/.make/tools.mk`
- work-card: 중앙 보드 카드 TASK-270 (devbox `tasks/todo/270-gitforge-gosec-go127-compatibility.md`)

## 증상과 원인

`.make/tools.mk`의 `GOSEC_VERSION ?= v2.22.10`은 gosec 버전만 고정하고 빌드
툴체인은 고정하지 않는다. mise go가 1.27.0으로 올라온 뒤 새로
`make install-security-tools`를 실행하면 gosec가 go1.27.0으로 빌드되고,
스캔 시마다 아래 오류로 죽는다(대상 패키지와 무관하게 매번 다른 표준
라이브러리 이름):

```
internal error: package "context" without types was imported from "command-line-arguments"
```

`make security-code`와 `make quality-check`가 go1.27.0 머신에서 항상
실패한다. go1.26.7로 빌드한 같은 버전 gosec는 같은 소스에서 정상 동작한다.

## 격리 근거 (2026-10-01)

- go1.27.0 빌드 gosec → 이 워크트리의 `pkg/gitsettings`(TASK-260 미관여)
  에서도 동일 오류. master 소스(primary 체크아웃)에서도 동일 오류.
- go1.26.7 빌드 gosec(primary 보유) → 동일 `pkg/gitsettings` 통과.
- golangci-lint v2.13.1, `go vet`, 전체 테스트는 go1.27.0에서 정상.

## 후속 방향(제안)

tools.mk가 gosec 빌드 툴체인을 고정(go1.26.x)하거나, gosec가 go1.27 export
data를 읽는 버전(x/tools 갱신 포함)으로 핀을 올린다. 어느 쪽이든 hosted CI
`security-code`가 실제로 통과하는 exact-SHA로 확인해야 한다.

## TASK-270 해결 기록 (2026-10-02)

실행 카드: 중앙 보드 TASK-270 (devbox `tasks/todo/270-gitforge-gosec-go127-compatibility.md`).

### 기준선 재실측 — 원인 정정

source SHA `5f05178`(origin/master)에서 v2.22.10을 두 고정 컴파일러로 각각
재빌드해 `./pkg/gitsettings` 최소 스캔을 2×2(gosec 빌드 툴체인 × 스캔 시점
go)로 측정했다.

| gosec 빌드 | 스캔 시점 go | exit | 결과 |
|---|---|---|---|
| go1.26.7 | go1.26.7 | 0 | 정상(JSON 유효, findings 0) |
| go1.26.7 | go1.27.0 | 1 | `internal error: package "context" without types ...`, stdout에 JSON 없음 |
| go1.27.0 | go1.26.7 | 0 | 정상 |
| go1.27.0 | go1.27.0 | 1 | 동일 오류 |

2026-10-01 기록의 "go1.27.0으로 빌드하면 실패"는 상관 관계일 뿐 인과가 아니다.
**실패는 gosec을 빌드한 컴파일러가 아니라 패키지를 적재하는 스캔 시점 `go`를
따른다** — mise가 1.27로 올라가면 빌드 go와 스캔 go가 함께 움직이므로 두 설명이
같은 관측을 낸다. 따라서 빌드 identity 검증(`GOSEC_VERSION_OK`)만으로는 이 실패를
잡을 수 없다(go1.27 빌드 v2.22.10도 identity는 통과했다). 실패 시 exit 1이지만
stdout에 JSON이 전혀 없으므로 finding 기반 exit 1과 구별된다. 바이너리 identity:
v2.22.10 `h1:ntbBqdWXnu46DUOXn+R2SvPo3PiJCDugTCgTW2g4tQg=`. 측정 증거(실행 머신
임시 저장): `/tmp/task-270/baseline-minimal-v2.22.10.txt`,
`/tmp/task-270/candidate-v2.29.0.txt`.

### 선택한 핀과 근거

**v2.29.0** — `.make/tools.mk` `GOSEC_VERSION ?= v2.29.0`.

- 공식 근거: gosec v2.29.0 릴리스(2026-08-26) 변경로그 `"Add support for go
  version 1.27.0 and update to version 1.25.14 and 1…"` (#1726). v2.23.0 ~
  v2.28.0 릴리스 노트에는 Go 1.27 지원 언급이 없다.
- 의존성: v2.29.0 go.mod는 `golang.org/x/tools` v0.48.0 → v0.49.0으로
  올라가고 floor는 `go 1.25.0`이라 설치된 두 고정 컴파일러(1.26.7, 1.27.0)
  모두로 빌드된다.
- 실측: v2.29.0은 위 2×2 네 셀 모두 exit 0. v2.28.0(x/tools v0.48.0)도
  스캔 go1.27.0에서 통과했으나 공식 지원 선언이 처음인 v2.29.0을 최소 지원
  버전으로 선택했다.
- 전체 무회귀: 같은 소스에서 go1.26.7로 각각 빌드한 v2.22.10과 v2.29.0으로
  `make security-code`와 `gosec -fmt=json ./...`를 실행 — 둘 다 exit 0,
  findings 0, Golang errors 0, 새 finding 0 (아래 영수증).

### 변경

- `.make/tools.mk`: `GOSEC_VERSION` v2.22.10 → v2.29.0. GOSEC identity 블록에
  원인 정정·계약 script 안내 주석 추가. 다른 로직 변경 없음.
- `scripts/test-gosec-toolchain-contract.sh` 신규: 기본 모드는 bin/tools 핀
  일치, 두 고정 컴파일러로 재빌드한 identity, `pkg/gitsettings` 최소 스캔
  4셀(빌드 × 스캔 go)을 검증하고 `--full-security-no-regression`는 기준 핀과
  전체 비교한다. 두 모드 모두 추적 파일을 변경하지 않으며 영수증은 stdout에
  출력하고 `--write-receipt`를 줄 때만 아래 마커 안에 기록한다. 고정
  컴파일러가 없으면 skip 성공 없이 실패한다.

해결 커밋: `test(tools): pin gosec for the Go 1.27 export-data contract`
(f3c7b45). 후속 수정: `fix(tools): keep gosec contract verify runs
read-only` — verify 재실행이 영수증의 HEAD SHA를 다시 써서 작업 트리를
더럽히던 결함을 고쳤다. 영수증 기록은 `--write-receipt` 옵트인으로 분리했고
기록 시 source는 스캔 시점 HEAD(영수증 이전 커밋)로 적어 영수증 커밋 후에도
참으로 남는다.

<!-- TASK-270 receipt:start -->
mode: --full-security-no-regression (scripts/test-gosec-toolchain-contract.sh)
source: f3c7b45cccdfaee5e07b412890e9027649fb0b68 (dev/claude/mst/test/task-270) — pre-receipt commit
source note: git HEAD when the scan ran; this receipt was recorded after
  that scan, so the commit carrying this receipt cannot make the SHA false
scan compiler: go1.26.7 at /Users/archmagece/.local/share/mise/installs/go/1.26.7
baseline pin: v2.22.10 (h1:ntbBqdWXnu46DUOXn+R2SvPo3PiJCDugTCgTW2g4tQg=)
current pin: v2.29.0 (h1:pF2HSLcnY5voqpxQumEe0O5YAAbvF4Syu4qeIewpLyc=)
make security-code exits: baseline=0 current=0
gosec -fmt=json ./... exits: baseline=0 current=0
findings (rule_id/file/line/code): baseline=0 current=0 new=0 resolved-by-current=0
verdict: PASS — no new findings; no scanner/runtime/export-data errors
receipt: printed to stdout on every run; recorded into the issue card only with --write-receipt
private builds/scans ran in an isolated mktemp dir, removed on exit in all outcomes
<!-- TASK-270 receipt:end -->
