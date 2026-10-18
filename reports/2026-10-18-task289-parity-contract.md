# TASK-289 parity 계약 복구

구현·독립 리뷰 요구는 각각 strong이며 신규 독립 리뷰 예산은1회, FAIL 시 중단한다.
기준 source는63c384abd00c073e34ae044c16b340b87b20ed86이다. 이전 ABORTED
후보 f6d19f9a의 선언 재검증·프로필 수정 전체를 재사용하고 참조 계약을 보완했다.

TASK-289는 모든 target에 resolution source를 표시하므로 config[0] 출력을
되돌리지 않는다. 현재 clean 설치 CE7140701a와 owning source가 일치한다.
캡처 전 SHA-256은2ce42eb5cfff9fc93c7394fd7452e181068138814a4506db6ccdf3a02391eca4다.
고정 CE의12시나리오를 실제 실행해 참조를 생성하고 별도 non-record 전체 parity를
검증했다. normalizer·시나리오·공개 API·manifest schema는 변경하지 않았다.

## 참조 차이

embedded JSON 문자열도 구조적으로 펼친48개 leaf 차이의 분류다. 중복 직렬화
view를 포함한 수이며 서로 독립된48개 사건이라는 뜻은 아니다.

| 분류                 |  수 | 의미                                                           |
| -------------------- | --: | -------------------------------------------------------------- |
| reference/toolchain  |  24 | 현재 clean CE revision                                         |
| receipt stamp        |  13 | 실제 실행 도구 revision                                        |
| physical processHost |   2 | 실제 Mac Studio 캡처 호스트, 정규화 없음                       |
| provider stdout      |   7 | 필수 config[0] 출처 표시, nested view 포함                     |
| recover refusal      |   2 | bound native completion을 지원 입력으로 설명, 여전히 동일 거부 |

상태·exit·owner·ref·push·worktree·cleanup 결과의 차이는 없다. bound native
completion 성공 경로의 수용은 주장하지 않는다. 다른 호스트의 차이는 기존 계약대로
남으며 native가 실제 기준선과 비악화를 판정해야 한다.

## 검증 경계

선택한 개발 검증은 변경 위험별 진단이다. CE_BIN을 실제 설치 경로로 고정하고
GOWORK=off, GOMAXPROCS=4, package worker4, parallel1로 실행했다.
capture는 TestCEParity 전체를 PARITY_RECORD=1로 실행했고, fresh verify는 별도
non-record package 전체를 실행했다. 각 outer5m, Go4m을 적용했다.
기계 캡처는 생성 결과이며 full gate 성공을 의미하지 않는다.

원본은 source primary의 tmp/task289-parity-contract/ 아래 capture.log,
verify.log, focused.log, quality-components.log에 보존한다. commit 프로필의
full 확대는 PASS가 아니며 실제 native 검사가 최종 수용을 결정한다.
이 문서는 리뷰·native 이전 구현 증거다. 최종 결과는 별도 CE receipt가 소유한다.
