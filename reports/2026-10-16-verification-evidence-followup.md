# 검증 증거 후속: 실패 출력과 legacy JSON

- 작업: `verification-evidence-followup`
- 구현 요구: strong / 독립 리뷰 요구: strong
- 기준 source: `6edbbe19a4fca8ef27cb93cfb840b87cd653144c`
- 증거 소유: `gzh-cli-gitforge:tmp/verification-evidence-followup-20261016/`
- 실제 전달 판정: CE task runtime의 같은 이름 실행 영수증

## 기존 진단의 복구 한계

이전 owner-doc 전달은 source `6edbbe19`, target `b362607e`에서 source와
기준선의 위치 진단 수가 3→3이고 변경 경로의 진단이 없어 비악화 WARN으로
승인됐다. lint는 PASS였으며 `make check`가 깨끗하게 통과한 것은 아니다.

`saveLegacyMakeDiagnosticWithPath`가 최종 FAIL만 저장해 WARN으로 바뀐 실패
측정 원문을 버렸다. 당시 로그 파일과 수명주기 영수증을 대조했지만 세 위치의
정체는 복구할 수 없었다. 원문 소실을 인정하고 과거 TASK-325 timeout 또는
GA4 실패를 동일 원인으로 추정하지 않는다. 이번 수정은 이후 실패 측정의
원문을 보존하며 판정 규칙·timeout·검사 선택은 바꾸지 않는다.

기존 semantic FAIL은 subprocess rc=0에서도 발생할 수 있어 해당 출력 보존을
유지한다. 여기에 실패/timeout 측정의 WARN 보존을 추가했다. 성공 판정과
성공 측정의 단순 WARN은 추가 저장하지 않는다. 기준선 원문 저장은 기존
계약을 유지한다.

## ISSUE-052의 호환 수정

receipt가 없을 때 기존 네 boolean은 `RunReport.Integrated`와 structured
removal 결과에서 채운다. 실제 Run에서 회수는 push 성공 뒤에만 실행되므로
`Integrated`가 false인 불일치 report의 removal 값을 positive outcome으로 만들지 않는다.
receipt가 있으면 그 값을 우선하며 false도 덮어쓰지
않는다. 필드·타입·schema v1은 그대로이고 owner·generation·receipt phase를
만들지 않는다. legacy true는 해당 호출에서 확인한 결과이지 durable receipt가
아니다. false도 side effect 부재의 증명으로 사용할 수 없다.

실제 bare-remote 통합의 target SHA, worktree 및 local/remote ref 부재와 JSON을
대조하는 회귀가 수정 전 실패하고 수정 후 통과했다. 독립적인 부분 결과와
receipt 우선순위, prose에서 결과를 추정하지 않는 조건도 검증했다.

## 스킬 선택 실관측

설치된 `quality-verification-workflow` SHA256:
`6ff601d2d08a31fe98ac6ddb2e9e6651f305f4d07e7e09ba9b2b5390ad81d6fc`.
Codex CLI 0.162.1의 새 native 프로세스를 사용했고 모델/provider 설정을 바꾸지
않았다. trace에 실제 모델이 없어 unknown으로 기록했다.

synthetic fixture의 개발·커밋·통합 증거 재사용·릴리즈 검증·문서·공유 설정과
삭제의 여섯 양성 사례에서 스킬 파일 read와 선언된 명령 선택을 관측했다.
통합 사례는 유효한 증거 재사용으로 직전 전체 검사 중복을 피했고, 릴리즈는
check-only였다. 별도의 순수 산술·휴가 일정 두 부정 사례에서는 스킬/tool read가
없었다. 실제 프로젝트 검사·빌드·배포는 실행하지 않았다.

첫 fixture는 명령 매핑이 불완전해 결과 해석을 제한했다. v2의 부정 사례는
검증 선택 요청이 섞여 있어 제외하고 v3의 순수 부정 사례로 보완했다. 원본
실패 관측, 정정, 정확한 prompt와 fixture bytes, raw JSONL, 시간·종료 코드를
모두 보존한다. 이는 단일 관측 묶음이며 일반적인 선택 성공률의 증명이 아니다.
관측된 유효 사례에는 스킬 소스 변경이 필요하지 않았다.

실제 task worktree의 별도 native 세션도 80.51초, exit 0으로 끝났으며 스킬을
읽고 commit 8분/DVA 10분, native full 60분/outer 67분과 직전 중복 회피 계획을
선택했다. 실제 검사는 실행하지 않았다. read-only Git의 xcrun 임시 cache 권한
경고를 보존했다. 관측 중 root의 소스 수정이 있어 초기 diff hash는 시점 기록일
뿐 고정 입력이나 재사용 가능한 게이트 증거가 아니다. 선택 행위 관측으로만 쓴다.

실제 저장소 지침에서도 root의 커밋 예산 표기(직접 8분, DVA wrapper 10분)와
`cmd/AGENTS_COMMON.md`의 오래된 매 커밋 전체 검사 요구를 현재 프로필에 맞췄다.
전체 게이트를 없애지 않고 native integrator가 실행하는 경계에 유지한다.

v2 case 02의 stderr에는 native host-service의 system skills 설치 중
`Directory not empty` 오류가 기록됐다. 프로세스는 0으로 종료했고 이후 설치된
스킬을 직접 읽어 선택을 완료했다. 원문을 보존하며 설치 디렉터리 정리나 원인
해결을 주장하지 않는다. 이 별도 runtime 오류의 수정 소유 경로는 미확정이다.

## 검증 경계와 후속

- 수정 전 동작 회귀: legacy JSON과 WARN 출력 유실을 실제로 재현한 FAIL 보존.
- 관련 회귀와 실제 synthetic 게이트: 16.428초 PASS. 독립 리뷰의 관련 회귀와
  outcome 계약 검사도 28.228초 PASS.
- format/lint/gosec/dependency 검사: 종료 코드 0. dependency scan은 호출 경로의
  취약점 0이며 미호출 package/module 발견이 없다는 뜻은 아니다.
- commit selector/recipe는 `VERSION` 변경을 unknown/shared 입력으로 보고 exit 3으로
  전체 검사 필요를 반환했고, 이를 감싼 Make 프로세스는 exit 2였다.
  이를 commit profile PASS로 표시하지 않는다.
- `pkg/integrate`만 기존 `-short --cover -timeout=30m -p=4 -parallel=1` 조건으로
  JSON 이벤트 진단했다. 종료 코드 0, 실패 이벤트 0, wall 870.874초였다.
  출력의 테스트 시간 832.579초 뒤 최종 package PASS 이벤트는 868.576초에
  도착했다. coverage는 77.1%이며 새로운 coverage 기준을 채택한 것은 아니다.
  고정 입력, diff와 파일 hash, 실제 종료·시간은 `package-diagnostic*`에 보존한다.
  skip 이벤트 5개는 subprocess helper 4개와 별도 retained artifact 입력을 요구하는
  legacy reconciliation 검사 1개다. 해당 retained-artifact 수용을 증명하지 않는다.
  현재 입력에서는 기존 세 진단을 재현하지 못했으며, 이는 과거 원문 복구나
  원인 해결 또는 전체 수용 증거가 아니다.
- 최종 전체 게이트는 native integrator가 실행하며 바로 앞에서 중복 실행하지 않는다.
  이전 Python/UV 소비자 수용 `9227bf3`/infra-ops `b7ad3178`을 새 설치 증거로
  소급 대체하지 않는다.
- 메트릭 저장·집계·자동 timeout 조정과 과거 TASK-325/GA4의 광범위한 재조사는
  별도 후속 범위다. 임시 timeout과 worker 제한은 유지한다.

스킬 관측 방법의 참고는 [OpenAI의 skill eval 문서](https://developers.openai.com/blog/eval-skills)이며,
위 결론은 일반 지침이 아니라 보존한 실제 trace에 근거한다.

## 동시 전달의 정합화와 중단 증거

다른 실행 `issue-052-legacy-run-flags`의 `6ef04783` 전달 시도는 실패했고 해당
시점의 master는 `6edbbe19`로 유지됐다. 그 실패 진단은 기존 FAIL 저장 경로의
출력이며 새 WARN 보존 수정의 수용 증거가 아니다. 미통합 후보를 source tip처럼
rebase하지 않는다. 이후 해당 task branch가 전진할 수 있으므로 위 SHA는 그
실패 시도만 식별한다.

우리 첫 native 시도는 target 잠금 대기 뒤 측정을 시작했으나, 중복 후보와
실제 Run 상태 순서에 맞는 guard를 정합화하기 위해 중단했다. 확인한 task
프로세스 트리의 provider/Make group ID에 SIGTERM을 요청했다. 이후 같은 숫자
PGID에서 관측된 별도 GoogleUpdater는 소유 증거로 취급하지 않고 재신호하지
않았다. 원격 master는 그대로이고 task worktree/branch는 다음 시도를 위해
보존했다. 원본 BLOCKED 영수증과 취소·종료 확인 증거는 primary 보존 경로의
`native-finish.*`, `cancelled-native-processes.json`에 남긴다. 이 시도를 PASS나
통합 성공으로 표시하지 않는다.

그 뒤 성공 guard를 추가하고 실제 상태에 맞는 부분 결과 회귀를 갱신했다.
최종 부분 결과 row까지 갱신한 좁은 회귀는 6.594초 PASS다. 위 870.874초 package 진단은 이 guard 정합화
이전의 고정 입력이며 최종 후보의 전체 게이트를 대체하지 않는다. 다른 세션의
자원이나 branch를 수정·회수하지 않는다.
