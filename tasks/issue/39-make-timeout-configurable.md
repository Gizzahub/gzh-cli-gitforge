# ISSUE: legacy 통합 게이트의 make 예산이 기본 15분에 고정되어 설정할 수 없다

- status: fixed (2026-10-01; 실행 카드 devbox TASK-260)
- priority: P1
- category: config/integrate
- created_at: 2026-10-01
- owner: `gzh-cli-gitforge/pkg/integrate`
- work-card: `gzh-cli-devbox/tasks/todo/260-make-target-integration-budget-configurable.md`

## 증상과 원인

legacy make 통합 게이트(`integrate check`)는 저장소마다 `make check`·`make
lint`를 직렬로 돌린다. cwrapper-devbox 보드는 337개 바인딩을 직렬 실행해
측정 게이트 벽시간이 약 53분인데, 내장 `makeTargetTimeout`(15분)이 단일
make 호출을 죽여 통합 검증이 실패했다. 예산이 하드코딩되어 있어 느린
저장소가 자기 선언으로 상한을 올릴 방법이 없었다.

## 해결 및 검증

repo-root 전용 선언 `branch.makeTimeout`(Go duration 문자열)을 추가한다.
`integrationBranch`·`taskPattern`과 마찬가지로 5계층 병합에서 상속되지
않고 루트 파일에서만 읽는다. 값이 없으면 기존 기본 15분이 그대로 적용되고,
해석 불가·0 이하·공백 보유 값은 로드 시점에 실패한다(fail-closed). JSON
경로에서는 숫자형이 문자열 규칙을 우회하지 못하게 막는다.

예산은 legacy make 게이트(`checkLegacyMake`)에서 한 번 해석되어
prepare·baseline·probe 체인 전체에 전달된다. `readiness` 러너 예산은 이
변경 범위 밖이다. 검증은 `TestRepoRootMakeTimeout`(config), 
`TestMakeTargetBudget`(integrate)과 전체 `make quality-check`이다.
