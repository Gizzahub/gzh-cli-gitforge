# ISSUE: target ownership 대기에 독립적인 시간 상한이 없다

- status: open
- priority: P2
- category: product/resource-lifecycle
- created_at: 2026-10-18
- owner: `gzh-cli-gitforge/pkg/integrate`
- discovered-in: CE delivery `readiness-cleanup-preservation`

## 확인한 동작

`pkg/integrate/targetlock.go`의 `acquireTargetLock`은 200ms polling과 parent context
취소만 사용한다. host resource의 lock-wait 예산은 이 경로의 시간 상한이 아니다.
따라서 parent에 deadline이 없는 호출은 target ownership을 무기한 기다릴 수 있다.

이번 provider PID95124는 17분 이상 하위 검사 프로세스 없이 대기했고, 같은 source
common-dir의 `49c8f7d5c7458baeb01c297edec96d2886bf083c68c65a3df9c2d60de8879582.lock`을
다른 provider들과 함께 열고 있었다. 열린 FD만으로 lock 소유자를 확정하지 않는다.
이번 provider만 SIGINT로 취소한 뒤 CE가 반환한 실제 오류는 다음과 같다.

```text
integrate: waiting for target ownership ... destination=refs/heads/master
post-operation evidence failed: verify task head is contained in source: exit 1
```

CE 결과는 BLOCKED, task `095652947a8d20f1a0b422389caa0347a6946614`, source
`1db93c1b6b1cef49586fe53645a60b7ef5e377e2`, sourcePushed/worktreeRemoved는 false다.
다른 세션의 프로세스·lock은 변경하지 않았다. 이 실행은 전체 검사 실패 사례가 아니라
전체 검사 진입 전 ownership 대기 중 취소된 사례다.

## 권장 후속과 실행 경계

target lock에도 명시적인 대기 예산과 경과·대상 진단을 적용할지 소유 작업에서 정한다.
취소·상한 도달 때 source 쓰기가 없다는 구분을 유지하고, holder 강제 종료나 lock
삭제로 우회하지 않는다. 기존 lock 재진입·순서 문제 여부는 추가 증거가 필요하다.
이번 카드는 정리 실패 보존 수정과 별개이며 lock 설계 변경까지 착수하지 않는다.
