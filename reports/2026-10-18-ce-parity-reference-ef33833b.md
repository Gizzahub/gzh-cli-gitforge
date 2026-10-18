# CE parity 참조 ef33833b 채택 기록

## 범위와 후보

단일 후보는 clean source와 설치된 CE 0.10.13의
`ef33833b542cbe84be842f8a77293e61bce449ea`다. 기존 참조는 `ad92901e`다.
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
