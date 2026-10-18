# CE parity 참조 42363c6e 채택 기록

## 범위와 후보

사용자가 별도로 승인한 후보는 clean CE source
`42363c6eb4213e496748f329365cb4293b97e850`이다. 이전 `8f7c2963` 전달은
`9174d8e6`으로 완료됐으며 그 기록은 보존한다. 이번에는 source·설치본·소비자
참조를 새 후보에 맞춘다. normalizer·시나리오·gz-git 제품 구현과 workstation
도구 pin은 바꾸지 않는다. 구현·독립 검토 요구 등급은 `strong`이다.

## 소유 변경과 배포

- `d7c88ff6`은 `ce task resolve`의 지원하지 않는 frontmatter를 쓰기와 성공
  출력 전에 거절한다. canonical scalar·정상 재해결 동작은 유지한다.
- `42363c6e`는 readiness expectation 임시 디렉터리 정리가 성공한 뒤에만
  verdict를 발행한다. 정리 실패 시 유효한 verdict를 내보내지 않는다.
- 이 두 동작은 기존 12개 lifecycle 시나리오에서 호출하지 않는다. 소유 Go
  filesystem·CLI 집중 회귀는 0.325초·0.336초에, Python 실행 검사 19개는
  46.637초, cleanup 실패 주입 검사 1개는 0.002초에 통과했다.
- 소유 CE의 `verification-ready-cleanup` native receipt는 후보의 master
  통합·push와 세 작업 자원 제거를 포함한 DONE을 확인한다. 집중 검사를 소유
  전체 게이트로 해석하거나 이번 작업에서 전체 CI를 중복 실행하지 않는다.
- 배포 전 독립 strong 검토가 통과했다. 공식 `make install_cli`는 exact
  revision과 source 경로를 바이너리에 기록한다. 변경된 Python bridge는 그
  source checkout이 소유한다. 이번 diff에 플러그인 자산·모델 설정 변경이
  없어 전체 플러그인 캐시 재설치 없이 필요한 CLI 설치 대상을 사용했다.
- clean exact source의 `env -u VIRTUAL_ENV make install_cli`가 성공했다.
  설치 CE 0.10.13은 Build `289-g42363c6e`, full revision 일치,
  `vcs.modified=false`다. SHA-256은
  `5b3b122df3763a0b2a957d9b0cf3606be82a656b8384278c17708953763e931b`다.
  CE primary `tmp/ce423-adoption-{install.log,build-metadata.log,installed.sha256}`에
  근거를 보존한다. 다른 호스트의 설치나 published release는 주장하지 않는다.

## 소비자 검증

기존 계약의 targeted writer로 12개를 기계 캡처했다. record 20.585초와 별도
전체 non-record 17.712초가 성공했다. 검사 전후 설치 CE·ambient gz-git 해시도
일치했다. 실제 provider는 harness가 이 task checkout에서 새로 빌드했으며,
ambient gz-git 해시는 그 임시 provider build의 해시 증거가 아니다.

직전 `8f7c2963` capture 대비 decoded 차이는 37개이며 reference 12개,
CE toolchain 12개, receipt tool_version 13개로 모두 CE identity다.
상태·종료 코드·권한·ref·push·cleanup과 `config[0]` 진단은 동일하다.
새 issue resolve 거절과 readiness cleanup 실패 동작을 이 12개 capture가
검증했다고 주장하지 않는다. 그 보장은 위 소유 회귀 검사와 별개다.

gitforge primary의 `tmp/ce423-parity-{record.log,verify.log,decoded-diff.json}`,
`tmp/ce423-parity-executable-before.sha256`에 원시 근거가 있다. DVA에는
commit/full profile만 있고 focused capture·설치 명령은 없어 소유 프로젝트의
선언된 명령을 사용했다. 전체 소비자 전달 판정은 native integrator가 소유한다.

devenv의 CE 지연 검사는 source·설치본이 모두 `42363c6e`인 14:51 UTC에
11 passed, 1.64초였다. 근거는 devenv primary의
`tmp/ce423-adoption-lag-check.log`다. 종료 시점 계약은 ISSUE030의 별도 최종
검사로 확인하며, 이 관측만으로 미래 소스 변경까지 일치한다고 주장하지 않는다.

## 전달 경계

이 기록의 집중 검증은 최종 native 전달 판정이 아니다. 성공 receipt의 DONE,
source push와 worktree·로컬·원격 task branch 제거를 실제로 검증한 뒤에만
primary의 `tmp/ce423-parity-final-native.json`에 보존한다. 이 호스트 보조
증거는 재사용 가능한 소스 승인서가 아니다. 이전 후보의 성공을 새 후보의
게이트 성공으로 해석하지 않는다.
