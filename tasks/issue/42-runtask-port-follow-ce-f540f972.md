# ISSUE: runtask 포트가 CE f540f972 · b09a605e 동작을 따르지 않아 parity 포트 모드가 전면 실패한다

- status: open
- priority: P2
- category: quality/parity
- created_at: 2026-10-03
- owner: `gzh-cli-gitforge/internal/runtask`
- work-card: 없음 — 포트 결정 대기 (devbox TASK-256 Stop 조건으로 분리)

## 증상

devbox TASK-256이 parity 골든을 설치된 CE(`ce-agent-kit` `f540f972`, 빌드
`498-gf540f972`, Worktrunk 0.80.0)에 맞춰 재녹화했다. 참조 모드
(`go test ./tests/parity/... -run TestCEParity -count=1`)는 11개 시나리오가
모두 통과한다. 포트 모드
(`PARITY_TARGET=gz-git go test ./tests/parity/... -run TestCEParity -count=1`)는
11개 모두 실패하며, `doctor-active`도 실패한다.

재녹화 전 master(`4419264`, 옛 골든)에서도 포트 모드는 11개 모두 실패했다.
그러므로 TASK-256이 만든 악화는 아니다. 다만 재녹화 뒤에는 실패 원인이
아래 동작 차이로 확정된다.

## 포트가 따라가야 할 CE 동작

1. **Worktrunk 버전 (CE `b09a605e`).** CE는 0.80 minor 라인을 받는다. 포트는
   `internal/runtask/service.go:17`의 `const worktrunkVersion = "0.74.0"`을
   `service_doctor.go:64`에서 부분 문자열로 비교한다. 그래서 설치된
   wt 0.80.0에서 doctor가 `Worktrunk 0.74.0 is required`로 BLOCKED된다.
2. **원격 추적 ref 기준 run source (CE `f540f972`).**
   - run-start가 `git show-ref --verify --quiet refs/remotes/origin/<integration>`로
     원격 추적 ref를 확인한다. 그 ref의 SHA로
     `wt switch --create --base <SHA>`를 실행한다. 포트는
     `service_start.go:75`에서 로컬 브랜치 이름(`--base <source>`)을 쓴다.
   - finish/recover의 pushed-source 확인은
     `refs/remotes/origin/<integration>..refs/heads/<integration>`를 센다.
     포트는 `service_lifecycle.go:411`과 `service_recover.go:114`에서
     `origin/<source>..<source>`를 센다.
   - finish는 `gz-git integrate check/run`에 `--target origin/<integration>`을
     명시해 넘긴다. 포트는 `--target` 없이 호출한다.
   - finish-explicit 시나리오는
     `git show-ref --verify --quiet refs/heads/<integration>` 진단을 하나 더
     가진다.

근거 골든은 `tests/parity/testdata/golden/*.json`(gitforge `2f6fbcf`)이다.
계약 문서 `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md` 머리말에 이동 이력이
있다.

## 결정할 것

- 포트를 위 동작에 맞춘다(권장: CE가 기준이고 골든이 그 계약이다).
  Worktrunk 확인은 정확한 버전 비교가 아니라 CE와 같은 minor 라인 판정으로
  바꾼다.
- 원격 추적 ref가 없을 때 CE가 거부하는 경로는 지금 어떤 시나리오에도 없다.
  포트를 고칠 때 그 시나리오를 추가할지 함께 정한다.

## 완료 판정

- `PARITY_TARGET=gz-git go test ./tests/parity/... -run TestCEParity -count=1`이
  exit 0이고 skip이 없다.
- 참조 모드는 계속 exit 0이다.
