# ISSUE: go1.27 툴체인으로 빌드한 gosec가 export data를 읽지 못한다

- status: open (2026-10-01 기준선 기록; TASK-260과 무관)
- priority: P1
- category: quality/ci
- created_at: 2026-10-01
- owner: `gzh-cli-gitforge/.make/tools.mk`
- work-card: 없음 — 신규 발견, 후속 카드 필요

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
