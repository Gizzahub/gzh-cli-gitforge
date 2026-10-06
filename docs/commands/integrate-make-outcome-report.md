# Make 검사별 결과 비교

기존 Make 게이트가 파일·행 정보 없이 실패하면 출력 문구나 전체 exit code만으로
같은 실패인지 새 실패인지 판단할 수 없다. 저장소가 검사별 결과를 제공할 수
있을 때 루트 `.gz-git.yaml`에 다음 계약을 선언한다.

```yaml
branch:
  makeOutcomeReport:
    version: 1
    targets: [check]
```

`check`, `lint`만 지원하며 하나씩 도입할 수 있다. 부모·workspace·global·profile
설정에서는 허용하지 않는다. 선언된 target이 없거나 보고서를 만들지 않으면
통합 검사는 실패한다. 미선언 저장소의 기존 비교 동작은 유지한다.

## 게이트의 보고서

gz-git은 target 실행마다 새 비공개 임시 경로를
`GZ_GIT_MAKE_OUTCOME_REPORT_V1` 환경변수로 전달한다. 기존 `make -w check` 또는
`make -w lint` 명령을 그대로 실행한다. 게이트는 실제 검사 결과를 수집하고
모든 검사가 끝났을 때 그 경로에 UTF-8 JSON 객체 하나를 작성한다.

```json
{
  "version": 1,
  "target": "check",
  "complete": true,
  "checks": [
    {"id": "format", "outcome": "pass"},
    {"id": "unit/shell-permissions", "outcome": "fail"}
  ]
}
```

각 ID는 실제 독립 검사에 대응하는 안정 식별자여야 한다. 출력 문구·시간·임시
경로를 ID로 사용하지 않는다. ID는 `[a-z0-9][a-z0-9._/-]{0,127}`이고 target 안에서
유일하다. `pass`, `fail` 외의 상태와 빈 검사 목록은 허용하지 않는다.
미실행·중단·skip을 pass로 쓰거나 완전한 보고서로 표시하면 안 된다.

모두 pass이면 실제 Make exit는 0, 하나라도 fail이면 실제 exit는 non-zero여야
한다. 불일치·누락·중복 필드/ID·알 수 없는 필드·잘못된 version/target·추가 JSON·
잘못된 UTF-8·1 MiB 초과·symlink/비정규 파일은 실패다. timeout·start failure·
tool crash·skip은 보고서로 구제하지 않는다. 선언된 계약의 불완전한 증거는
`--allow-skipped-checks`로도 완화하지 않는다. 임시 보고서는 읽은 뒤 회수한다.

## 비교와 도입

비교는 source와 target의 **커밋에 기록된 선언**으로 결정한다.

| 변화                        | 판정          |
| --------------------------- | ------------- |
| 기존 fail 유지              | non-worsening |
| fail → pass                 | 개선          |
| pass → fail 또는 새 fail ID | 차단          |
| 새 pass ID                  | 허용          |
| 기존 ID 삭제·이름 변경      | 차단          |

기존 changed-path 진단과 file:line 개수 증가도 계속 차단한다. 완전한 보고서는
그 증거를 덮어쓰지 않고, file:line이 없는 실패를 추가로 비교하는 근거다.
예를 들어 baseline에 format·shell 실패가 있고 source에서 format만 해결했다면,
같은 shell ID의 실패가 남아도 개선으로 판단한다. 남아 있는 baseline 실패를
통과한 검사로 표현하지 않으며 기존 WARN non-worsening 상태로 표시한다.

target이 아직 선언하지 않은 첫 도입은 보고서 형식과 exit를 검증하되 그 통합의
허용 여부는 **기존 legacy 비교**로 판단한다. 새 계약이 자기 자신을 통과시키지
않도록 깨끗한 bootstrap 커밋으로 먼저 도입한다. target이 도입한 뒤 source가
선언이나 target 목록을 제거하면 차단한다.

V1의 ID와 target 목록은 append-only다. 삭제·이름 변경을 통한 은밀한 검사 축소를
허용하지 않는다. 검사 구조를 바꿔야 하면 별도로 검토된 target-owned readiness
계약으로 전환한다. [readiness bootstrap](integrate-bootstrap.md)과
[계약 갱신](integrate-readiness-update.md)의 사람 확인 절차를 따른다.

보고서는 저장소 소유 게이트가 제공하는 결과 계약이며, branch-owned Makefile이
실제로 모든 검사를 수행했다는 독립 실행 증명은 아니다. 그 소유권 경계가 필요하면
source와 target을 함께 검사하는 target-owned readiness runner를 사용한다.
