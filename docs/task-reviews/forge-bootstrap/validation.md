# Forge bootstrap 구현·독립 리뷰 증거

## 범위와 실제 실행

GitLab namespace·프로젝트·원격·checkout·master/develop·template URL/SHA·사이트 설정을 digest에 묶는 plan/apply를 구현했다. 최초 Template의 검증된 seed 복사와 실제 사이트 설정 없는 중립 baseline도 지원한다. 기존 내용·경로 충돌·상태 변경을 거절하고, 단계 영수증과 실제 상태가 일치할 때만 재개한다. Git 이력은 복사하지 않으며 출처·설정 digest와 외부 최초 SHA를 남긴다.

요구 등급은 구현 strong·별도 독립 리뷰 strong이다. 최초 /root/bootstrap_impl의 실제 런타임은 gpt-5.6-luna였으며 이전 prototype 기록의 GPT-6 표기는 잘못됐다. 해당 prototype을 strong 완료 증거로 사용하지 않는다. 이후 /root/strong_bootstrap과 /root(주 세션)가 substantive 구현·교정을 수행했고 별도 /root/strong_review가 실제 diff·경계·회귀 검사를 검토했다. 두 strong 세션의 실제 런타임 gpt-6.1-sol은 session_meta.agent_path와 turn_context.model을 대조해 확인했다. 사용자 라우팅 설정은 변경하지 않았다. 이전 작업 이력은 보존한다.

## 독립 리뷰와 회귀 검사

최종 source freeze 후보는 독립 strong PASS다. 최초 리뷰의 거절 원인인 API 키 이름 변형, materialized 재개 전 변경·symlink 쓰기, tags/notes 누락, 프로젝트 ID 교체와 prefix 설정 치환을 수정했다. per-step digest checkpoint와 root-scoped atomic 쓰기로 사용자 수정·symlink·hardlink 외부 대상을 보존한다. 전체 refs를 확인하고 계획 ID·provision 반환 ID·실제 ID를 비교한다. 설정 치환은 정확한 CE 속성과 Worker JSON 속성을 검사한다. 동시 외부 filesystem mutation은 지원하지 않으며 불확실한 단계는 영수증을 조작하거나 자동 삭제해 숨기지 않는다.

실제 bare Git 실패·재실행, 프로젝트 생성/빈 재사용/비어있지 않음/권한·API 실패, 경로·SHA·설정·비밀·원격 refs·전역 ignore·초기 seed 무브랜드 조건을 검사했다. 최종 구현자 package 테스트 exit0·coverage86.2%, unlimited lint exit0(0 issues), mdformat·diffcheck exit0. 독립 리뷰 package 전체 테스트 exit0·coverage85.0%, CLI Bootstrap 테스트 exit0, diffcheck exit0. 실행 사이 root strict adapter 회귀 추가 등에 따른 수치는 각 실행 관측으로 유지하며 서로 같은 실행으로 합치지 않는다.

## 필수 전체 검사와 기존 기준선

canonical `make quality-check`는 exit2이며 로그는 `tmp/forge-bootstrap-quality-final.log`다. 새 read-only 명령의 행동 검사 누락은 실제 TLS API 행동 검사로 고쳤다. 전체 read-only suite와 negative control -race exit0을 구현자·독립 리뷰가 각각 확인했다. 같은 실행에서 오래된 source helper의 stdout pipe block이 22분에 종료됐다. 최신 origin/master에는 async drain과 대용량 회귀 검사가 이미 있고 변경 없는 primary의 TestDisplayCloneResultsAndIcons -cover exit0으로 확인했다. 이 upstream 수정을 rebase로 보존하며 새 helper를 복제하거나 timeout을 늘리지 않는다. 전체 PASS를 선언하지 않는다. 기존 `tmp/forge-bootstrap-quality.log`는 format·unlimited lint·security·build 후 parity 단계에서 nonzero였다.

변경 없는 primary source 0ff0b2213153d2a8a7448ee2a1e3305d7c4a962c에서도 `go test -short -cover -count=1 -coverprofile=tmp/forge-bootstrap-baseline-current/parity.out ./tests/parity` exit1을 재현했다. 로그는 primary `tmp/forge-bootstrap-baseline-current/parity.log`다. 설치 CE 9877764d와 기존 fixture b79c1a4의 toolchain drift 및 program not built with -cover가 원인이다. 소유 이슈는 [ISSUE-41](../../../tasks/issue/41-ce-parity-golden-drift.md)이다. 새 bootstrap 수정으로 golden·pin·gate를 낮추거나 다른 세션의 참조 채택을 덮어쓰지 않는다. 이 관측은 전체 gate 성공이나 baseline 수정을 의미하지 않는다.

추가 필수 `make test-integration-quality test-e2e-only coverage-critical-check` exit0. package integration·E2E·critical coverage floor를 실제 실행했으며 로그는 `tmp/forge-bootstrap-remaining-quality.log`다. 최종 source rebase 뒤 affected 검사를 다시 확인한다.

CE controller는 기존 primary 선언을 사용한다. primary doctor ACTIVE와 run-status는 이 작업 actor·host·branch·worktree를 식별한다. task checkout의 로컬 선언 누락은 임의 복사로 우회하지 않고 primary에서 status/finish를 수행한다. 실제 finish 거절 시 중단한다.

## 전달 경계

소스 통합·push·CE 회수, 공식 stable 태그·Homebrew cask 게시와 workstation 지정 설치의 실제 결과는 이후 영수증에서 확인한다. snapshot/source binary를 설치 바이너리로 덮어쓰거나 PATH를 바꾸지 않는다. 사이트 공개 배포·인프라 적용·GA·검색 공개·Design System 재게시는 수행하지 않는다.
