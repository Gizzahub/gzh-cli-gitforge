# CE parity 참조 ef33833b 채택 기록

## 현재 후보: 8f7c2963 추가 채택

통합 대기 중 다른 세션에서 소스와 설치 CE가 clean
`8f7c2963e27128e7d112a42b1f6c17bf039514aa`로 갱신됐다. 사용자는 의미 검토와
재검증 후 이 후보를 채택하도록 추가 승인했다. 아래 ef33833b 관측은 당시
이력이며 현재 후보의 검증을 대신하지 않는다. 기존 파일명과 작업 이름도
착수 이력을 유지한다.

추가 세 커밋 중 두 개는 문서이며, 제품 변경은 decisions 검증을
`ce validate decisions`로 분리하고 source branch resolver를 공유한다.
선언 우선·origin/HEAD fallback·읽기/파싱 오류 거절은 유지된다. 소유 CE의
source/Change-Id/native completion 회귀 검사와 decisions·adapter·CLI 배선
집중 검사는 모두 통과했다. 이 검사는 전체 CE 게이트를 대신하지 않는다.

12개 golden을 재수집한 decoded diff는 ef33833b 대비 CE identity 37개뿐이다.
상태·종료 코드·권한·ref·push·cleanup과 `config[0]` 진단은 동일하다.
normalizer·시나리오는 변경하지 않았다. 최초 whole-package record는 이전
golden을 검사하는 pin assertion도 함께 실행해 package가 실패했으므로 채택
증거로 쓰지 않았다. 해당 실패는 `tmp/parity-8f7c2963-record.log`에 보존하고,
계약에 지정된
`-run '^TestCEParity$'` writer는 62.939초, 별도 non-record 검증은 44.028초에
성공했다. 두 검사 전후 CE·ambient gz-git executable SHA-256도 일치했다.
원시 근거는 primary의 `tmp/parity-8f7c2963-*`에 보존한다.

최종 전달은 후보 기준 native receipt가 판정한다. 성공을 검증한 receipt만
`tmp/parity-8f7c2963-final-native.json`으로 보존한다. 아래 ef33833b의
성공 receipt 예정 경로는 추가 채택으로 대체됐으며 성공 파일을 만들지 않았다.

## ISSUE54 소유 수정 채택과 설치

사용자가 별도 소유 작업 인계·통합·설치를 승인했다. 수정은 최신 source 위로
재배치해 집중 slow-probe 검사를 12.504초에 통과했고 독립 strong 검토도
통과했다. native 절차는 `a1c51c9f`를 master에 통합·푸시하고 세 작업 자원을
모두 제거해 DONE으로 종료했다. 검증 1,950,580ms, 자원 대기 460ms다.
이는 native 통합 판정이며 source·baseline의 개별 검사가 모두 green이라는
주장은 아니다. 원시 결과는 `tmp/resource-handoff-finish.json`에 보존한다.

clean committed primary의 `make install`과 selected-path audit가 성공했다.
설치본은 `a1c51c9f`, SHA-256은
`15fddbccd4ef3135f88d04d02400f2d89b4ad3a2551c86591d35b0feaa50bad5`다.
상세 소유 기록은 [ISSUE54](../tasks/_archive/issue/54-resource-probe-deadline.md)에 있다.
parity 작업도 이 source 위로 재배치했고, 변경된 checkout provider를 새로 빌드한
별도 전체 non-record parity 검사가 29.593초에 성공했다. 로그는
`tmp/parity-8f7c2963-installed-provider-verify.log`다. ISSUE55의 기존 slot 직전 freshness
간격은 별도이며 이번 수정에서 해결됐다고 주장하지 않는다.

## 범위와 후보

단일 후보는 clean source와 설치된 CE 0.10.13의
`ef33833b542cbe84be842f8a77293e61bce449ea`다. 착수 시 참조는 `ad92901e`였고,
재배치 후 source의 참조는 후보의 조상인 `7140701a`였다. 후보는 바꾸지 않았다.
parity reference pin·기계 캡처·계약 문서를 갱신한다. 설치된 CE 바이너리와
워크스테이션 도구 pin, normalizer·시나리오·제품 구현은 변경하지 않는다.
구현과 독립 검토 요구 등급은 `strong`이다.

## 검증과 의미 차이

- 사전 parity 검사는 revision 불일치를 즉시 거절했다.
- 기존 계약의 `PARITY_RECORD=1` targeted writer로 12개를 모두 기계 캡처했다.
  18.222초 성공, 캡처 전후 설치된 CE executable SHA-256 일치.
  실제 provider는 harness가 checkout에서 빌드했다. 설치된 ambient gz-git의
  해시도 유지됐지만, 이는 임시 provider build의 해시 증거가 아니다.
- JSON 내부 stdout을 재귀적으로 decode한 차이는 39개 leaf다. 37개는 CE
  identity이며 둘은 recover 거절 reason·stderr다. 수작업 fixture 편집은 없다.
- CE `e5fc498c`는 완결된 native provider 증거가 있는데 최종 CE 기록이 끊긴
  경우 provider 재실행 없이 로컬 자원 부재와 조상을 검증해 복구한다.
  `3b7df148`은 dangling worktree symlink도 거절한다. 이는 실제 계약 확장이다.
- 기존 12개 시나리오의 status·schema·종료 코드·ref·정리 결과는 불변이다.
  새 복구 성공 경로는 이 캡처 범위 밖이며 소유 CE 검사로 별도 검증했다.
- 소유 CE의 `TestNativeCompletion*`, `TestRecoverNativeCompletion*` 기존
  검사 실행은 0.493초에 성공했다. positive, 위조 proof, 중복 native 진단,
  자원 잔존, source probe 실패, 비조상 및 dangling-link 거절을 포함한다.
- 최초 non-record 검사는 계약 문서의 새 revision 누락을 잡았다. 계약 §6·§7을
  갱신한 뒤 별도 full non-record 검사는 21.213초에 성공했다.

CE의 docs-record admission 변경은 해당 저장소의 검증 선택 계약이며 lifecycle
sandbox에서는 실행되지 않는다. timeout 후손 정리 보강도 정상 시나리오 결과에는
차이가 없었다. 이 채택은 CE의 새 복구 기능을 gz-git에 구현했다는 주장이 아니다.

## 증거와 전달 경계

primary checkout의 `tmp/parity-ef33833b-record.log`,
`tmp/parity-ef33833b-verify.log`, `tmp/parity-ef33833b-ce-recovery-unit.log`,
`tmp/parity-ef33833b-decoded-diff.json` 및
`tmp/parity-ef33833b-executable-before.sha256`에 호스트 관측을 보존한다.
DVA의 commit/full profile은 확인했지만 focused parity·CE unit 명령은 없어
계약에 선언된 Go 명령을 사용했다. 전체 전달 판정은 native integrator가 소유한다.
독립 검토에서 상단 현재 참조·toolchain 표의 구 revision과 해시 증거 범위를
바로잡았다. 참조 검사는 문서 어디든 SHA가 있으면 통과하던 조건 대신 현재
참조 선언과 CE toolchain 행을 검사하도록 강화했다. 강화 후 별도 full non-record
검증은 24.881초에 성공했으며 `tmp/parity-ef33833b-verify-final.log`에 보존했다.
집중 검증이나 기계 캡처를
전체 게이트 성공으로 해석하지 않는다.

## 독립 검토

독립 `strong` 검토는 PASS다. 기존 12개 bounded corpus의 참조 채택이라는
범위, decoded diff, 정확한 후보 CE의 소유 회귀 검사와 소비자 성공 경로 미검증
제한을 대조했다. 새 복구 경로를 gz-git이 구현·의존하거나 호환성을 보장할 때는
별도 consumer scenario가 필요하다. 이번 참조 채택에서는 그 보장을 추가하지 않는다.

## 첫 native 전달 시도: 측정 불가

후보 `0a46c87b`의 native 전달은 FIFO 자원 대기 중 `context deadline exceeded`로
거절됐다. `resource_wait_duration_ms`는 401,461이며 전체 게이트는 하나도 측정되지
않았다. source·target 신선도와 clean/pushed 상태는 통과했고 push/reclaim은 없다.
원시 receipt와 stderr는 primary의 `tmp/parity-ef33833b-finish-0a46c87b.*`에 있다.

관측 당시 슬롯 소유 PID 89296은 sigdock-idp-rs의 실행 중인 검증이었다.
같은 원격 관측 프로브 deadline을 다루는 기존 소유 작업은 gitforge
`dev/codex/mbp/fix/resource-probe-timeout`의 ISSUE-54
(`tasks/issue/54-resource-probe-deadline.md`, 당시 commit `6c490c89`)다.
이 기록은 해당 수정의 채택·설치를 승인하거나 반복 대기를 해결했다고 주장하지 않는다.
슬롯 소유자와 잠금을 보존하고 가용성을 확인한 뒤 원래 native 전달을 재시도한다.

## 상류 변경과 충돌 해결

두 번째 native 시도는 make check·make lint 모두 PASS였다.
structured receipt의 resource wait는 175,787ms, validation은 787,370ms다.
그러나 source가 `63c384ab`에서 TASK-289의 `abee75f2`로 이동해 post-gate freshness만
실패했으며 통합·push·reclaim은 없었다. primary의
`tmp/parity-ef33833b-finish-276ef244.*`에 원시 결과를 보존했다.

`7140701a`가 `ef33833b`의 조상임을 Git으로 확인했다. 충돌 시 상류 제품 구현을
수정하지 않았으며 golden은 상류에서 복원한 뒤 새 checkout provider로 12개를
기계 캡처했다. fixture를 수작업으로 합치지 않았다. record 19.245초와 별도
non-record 19.247초가 성공했고 CE·설치 gz-git 해시도 유지됐다.
`config[0]` provider 진단은 보존됐으며 새 baseline 대비 decoded 39개 leaf는
identity 37개와 literal physicalHost 2개다. 이전 39개(거절 문구 포함) 분석은
재배치 전 source의 역사적 관측이다. 충돌 해결 독립 strong 검토는 PASS다.
새 로그는 `tmp/parity-ef33833b-rebased-{record,verify}.log`, 구조 차이는
`tmp/parity-ef33833b-rebased-decoded-diff.json`에 보존한다.

참조 불일치의 소스 수정과 검증은 완료됐다. 실제 최종 전달 여부는 native
receipt가 판정한다. 성공 receipt를 검증한 뒤 primary의
`tmp/parity-ef33833b-final-native.json`으로 보존하며, 그 파일의 `DONE`,
source push와 worktree·로컬·원격 task branch 제거 결과가 전달 완료 근거다.
이 호스트 보조 증거는 source에 포함된 재사용 승인서가 아니다.
