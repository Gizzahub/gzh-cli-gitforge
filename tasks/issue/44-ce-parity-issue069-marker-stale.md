# ISSUE: ce 511+에서 no-arg run-finish 동작이 바뀌어 parity의 ISSUE-069 known-divergent 표식이 낡았다

- status: open
- priority: P2
- category: quality/parity
- created_at: 2026-10-04
- owner: `gzh-cli-gitforge/tests/parity`
- work-card: 없음 — 재녹화 범위 결정 대기(메인테이너)

## 증상

설치된 ce-agent-kit이 `v0.8.4-511-g3656558b`(재녹화 시점에는 `513-g62db34ea`)로 올라간 뒤 `GOWORK=off go test ./tests/parity/...`가 golden guard에서 실패한다. 11개 golden이 모두 `498-gf540f972` 기준이기 때문이다. 이것만이면 ISSUE-41과 같은 재녹화로 끝나지만, `PARITY_RECORD=1`로 재녹화하면 스위트가 그래도 실패한다.

```
--- FAIL: TestCEParity/finish-noarg-single-active
    scenarios_test.go:133: known-divergent marker stale: CE now answers no-arg run-finish
    with exit 1 stdout {"status": "ACTIVE", "activeCount": 0, "task": "noarg",
    "reason": "task branch has no upstream", "nextAction": "configure and push the task branch upstream", ...}; re-evaluate ISSUE-069
```

## 원인 (2026-10-04 관찰)

`finish-noarg-single-active` 시나리오(`tests/parity/scenarios_test.go`, ISSUE-069)는 CE가 run이 하나뿐일 때 no-arg finish에서 nil-execution guard에 걸려 `READY`인데 exit 1을 낸다는 차이를 `knownDivergent: true`와 `portContract{Status: "READY", ExitCode: 1}`로 고정해 두었다. 새 CE는 단일 run을 정상적으로 찾아 finish 준비 상태를 평가한다. 시나리오 샌드박스는 태스크 브랜치를 push하지 않으므로 `ACTIVE`, `task branch has no upstream`, exit 1로 거부한다. ISSUE-069가 지적한 결함은 CE 쪽에서 고쳐진 것으로 보이고, 표식과 계약은 더 이상 실제 동작과 맞지 않는다.

재녹화 diff의 나머지는 다음과 같다(word-diff 분류).

- 버전 스탬프: `498-gf540f972` → `513-g62db34ea`, 11개 파일 모두.
- 캡처된 `gz-git` 출력: `--version` 0.8.0 → 0.9.0, 그리고 `integrate check/run --help`에 새로 생긴 `--expect-source` 플래그와 exit 4 설명, `integrate-release` capability. 이 부분은 golden이 PATH에 설치된 `gz-git` 버전에도 기대고 있다는 뜻이다.
- `--allow-skipped-checks` 설명 문구가 늘어남.

## 제안

1. `finish-noarg-single-active`를 다시 설계한다. 태스크 브랜치에 upstream을 만들어(샌드박스 bare remote로 push) no-arg finish가 실제 finish 경로까지 가게 하고, `knownDivergent`과 `portContract`를 지운다. 그다음 gitforge 포트(`gz-git run finish`)가 같은 결과를 내는지 확인한다. 다르면 이슈 42처럼 포트 추종 카드를 따로 만든다.
2. 이어서 `pinnedCECommit` 상수, `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md`의 ISSUE-069 행·본문·이력, golden 11개를 한 커밋으로 재녹화한다(ISSUE-41 절차).
3. (선택) 캡처된 `gz-git` 출력이 설치본 버전에 따라 바뀌는 문제는, golden 정규화에서 `gz-git` 버전을 placeholder로 바꾸거나 테스트가 빌드한 바이너리를 쓰게 해 분리한다.

재녹화만 해서 표식 실패를 덮지 않는다. 1을 먼저 결정해야 한다.
