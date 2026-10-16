# Readiness runner 예산: 검증과 전달

구현·독립 리뷰 요구 등급은 strong이다. 기능 구현 후보는
`72d5796bf9145c6f38e0cdec623341ef7b1a548e`, 기준 source는
`6245975085298ea40f3a7fb87b2a33b6d1bb8bf9`다.
선택 timeout은 target 계약과 digest가 소유하며 기본15분·상한2시간이다.
기존 모델·provider 설정이나 별도 실행 override는 변경하지 않는다.

## 최초 native 전달: BLOCKED

[원본 CE 영수증](first-finish.json)은 종료1/BLOCKED이며 통합·source push·회수가
이루어지지 않았음을 기록한다. host resource 대기25m1.308s와 validation48m19.859s는
도구가 분리 관측한 값이다. source make lint는 pass, make check는 baseline의2개보다
진단이3개로 늘어 거절됐다. 전체 검사를 PASS로 기록하지 않는다.

추가 진단은 `pre_mutation_outcome_test.go:125`다. 기존
`TestSameExecutionCannotRecordNegativeWhileAnotherRunOwnsIt`의100ms context가
Run의 저장소 정체성 확인 중 만료돼 `not a git repository`를 반환했다.
테스트는 실행 잠금의 ownership wait 오류를 기대했으므로 fail했다.
이는 검사에서 관측한 실패 단계이며 timeout 기능의 회귀로 단정하지 않는다.

기존 parity guard는 설치 CE revision과 golden의 불일치를 보고했다. source와 후보의
`tests/parity` Git tree는 모두 `62d0dd511c263b9ebca48c5309b329d9cfd58b1d`다. 이를 새 후보가 해결했다고
주장하거나 golden을 다시 녹화하지 않는다. 기존 ISSUE-41에 연결하며, 최종 non-worsening
판정은 실제 native gate가 소유한다. native는 이번 추가 진단을 거절했다.

전체 source 원본 진단은 로컬
`/Users/archmagece/.local/state/gz-git/integrate/diagnostics/make-check-3125214192.log`다.
최초 영수증은 원본 경로와 최종 판정·SHA를 보존한다.
좁은 회귀·lint/security·독립 리뷰의 기존 PASS와 이번 전체 gate 거절을 구분한다.
실행 잠금 시험의 취소 경계를 바로잡은 뒤 실제 gate를 다시 판정한다.

## 대기 경계 회귀 보정

제품 코드는 바꾸지 않고 위 테스트에서 실제 실행 잠금 frame 도달을 관측한 뒤
context를 취소한다. 기존 owner lock은 유지하며 오류·nil report·미기록 assertion도
유지했다. 안전 관측 상한2초가 준비 단계의100ms deadline을 대신한다.

worker의 대상10회와 관련 pre-mutation 묶음이 통과했고, main의 독립 대상5회는
4.788초·종료0이었다. `make format-check lint` 종료0/0 issues이며 별도 strong reviewer가
실제 diff를 PASS했다. reviewer 세부 runtime model ID는 미확인이다.
이는 좁은 회귀 통과이며 최초 전체 gate 거절을 취소하지 않는다. 새로운 전체 native
판정을 완료하기 전에는 도구 설치·source 통합·회수 완료를 주장하지 않는다.
