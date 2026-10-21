# ISSUE: ce 511+에서 no-arg run-finish 동작이 바뀌어 parity의 ISSUE-069 known-divergent 표식이 낡았다

- status: resolved (2026-10-22)
- priority: P2
- category: quality/parity
- created_at: 2026-10-04
- owner: `gzh-cli-gitforge/tests/parity`
- work-card: quality-baseline — CE cd0e1d19 기준 정합화; 전체 포트 추종은 ISSUE-42에서 계속

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

## 해결

`finish-noarg-single-active`는 sole ACTIVE 실행을 선택한 뒤 일반 finish
사전조건을 적용하도록 다시 녹화했다. upstream이 없는 이 fixture는
task-scoped ACTIVE/no-upstream 거부를 유지한다. 두 ACTIVE 실행에서는
owner-scoped BLOCKED 거부를 검증하는 `finish-noarg-ambiguous-active` 회귀 시나리오를
추가했다. CE cd0e1d19 실행 파일의 Go build metadata를 golden provenance로
기록했으며, 오래된 known-divergent/portContract 표식은 더 이상 사용하지
않는다. 외부 회수 후 조회는 CE ADR-0073대로 BLOCKED 기록을 유지하고 기존 start receipt를 변경하지 않는다는 semantic assertion으로 검증한다. 전체 target-mode parity는 purpose・resume・provider capability・receipt 차이 때문에 완료로 선언하지 않는다. 최신 참조와 의미 계약을 고친 결과는 검증 증거와 함께 기록한다.

The full target-mode remainder is tracked in
[ISSUE-42](42-runtask-port-follow-ce-f540f972.md).
