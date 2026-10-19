# Readiness cleanup 안전성 수정 재전달

2026-10-19 사용자 권장안 실행 요청에 따른 새 delivery 실행이다.
실행 ID는 `readiness-cleanup-safety-delivery`, 기준은 원격 master `e1a4bb5a`다.
ABORTED `readiness-cleanup-safety`의 `d99faea9`에서 검토된 제품 수정·테스트·원본 증거만
선별 재사용한다. 종료된 worktree·원시 영수증은 수정하지 않는다.

별도 ABORTED `readiness-cleanup-preservation`의 `e5a78b7a`도 비교했다.
Git 실패 후 filesystem 삭제 차단과 진단 보강은 중복된다. 이번 후보는 그보다 엄격한
생성 SHA·detached 등록·physical identity 검증과 root 비재귀 정리, bounded 진단을 유지한다.
그 별도 실행의 5분 cleanup 예산은 측정된 해결 근거가 아니므로 가져오지 않는다.
원래 30초 독립 cleanup 예산과 자동 재시도 없음 정책을 유지한다.

구현 standard, 독립 삭제 경계 리뷰 strong이다. 변경 범위는 기존 10개 경로와 이 재개 기록이다.
원격 master에는 두 후보의 cleanup 수정이 아직 없음을 확인했다.
로컬 master는 원격보다 7커밋 뒤처져 있으며 CE가 반환한 새 worktree는 원격 tip에서 시작했다.
원본 [검증 기록](validation.md)과 [실패 영수증](native-finish-blocked.json)은 과거 결과이며,
새 실행의 full gate·통합·push·회수 성공을 뜻하지 않는다.

설치·공개 릴리스·CalcGlide 재통합은 이번 도구 소스 전달 범위에 포함하지 않는다.

## 새 실행 검증

재사용 10개 경로를 Git blob과 직접 비교해 byte-identical을 확인했다.
기존 집중 회귀 명령은 exit 0(15.561s)이다. `git diff --check`는 통과했다.
`VERIFY_BASE=origin/master make verify-commit`은 profile exit3/Make exit2로 full을
요구했다. 이는 좁은 검사 PASS가 아니며 native full gate 결과를 기다려야 한다.

독립 strong 리뷰는 PASS, blocker 없음이다. 10/10 blob 일치와 실패 보존 경계를 확인했다.

## 실제 전달 결과와 mst 설치 차이

후보 `846b31ed1ab9c6aa0c015c44c70f2fd2dfe838e8`의 CE finish는
2026-10-19T02:13:11Z에 BLOCKED 종료했다. 원본은
[delivery-blocked.json](delivery-blocked.json)에 보존한다. resource wait는
22m15.878s였으며 make check/lint 모두 `no gate measured`다. 전체 검사 실패나
통과로 바꾸지 않는다. source push와 worktree/로컬/원격 branch 회수는 모두 false다.

실행된 mst `~/go/bin/gz-git`는 go build info상 `abee75f2`(modified=false)다.
원격 source에 포함된 [ISSUE-54](../../../tasks/issue/54-resource-probe-deadline.md)의
원격 관측 예산 수정 `a1c51c9f` 이전이다. 이 이슈의 지정 installer 적용은 mbp만
입증하며 mst에는 미적용이다. 이번 timeout의 직접 원인이 그 5초 probe인지는
원본 영수증만으로 확정하지 않는다. ISSUE-55의 별도 release-window 문제도
이번 실패 원인으로 단정하지 않는다.

이 실행은 깨끗한 pushed checkpoint와 BLOCKED 증거를 유지한다. 같은 조건의
native 재시도는 하지 않는다. 다음 전달 조건은 mst의 소스·관리 선언·실행 파일
정합성을 지정 installer 절차로 해결한 뒤 현재 source와 후보를 재검증하는 것이다.
이 문서는 tool 설치나 임시 PATH override를 승인·실행한 기록이 아니다.

## 승인된 mst 설치 반영 (2026-10-19)

사용자 “승인. 진행시켜” 지시로 이미 source 통합된 `e1a4bb5a`의 지정
`make install`을 별도 깨끗한 CE experiment에서 실행했다. exit 0이며
`install-path-audit: OK`다. 셸 선택 경로 `~/go/bin/gz-git`의 go build info는
해당 전체 SHA와 `vcs.modified=false`를 확인했다. 표시 version은 `0.10.4`로 동일하므로
버전 문자열만으로 채택을 판정하지 않는다. [원본 설치 증거](mst-adoption.json)에
전후 SHA-256, 실제 설치 로그, 빌드 설정, 승인 범위를 보존했다.

Homebrew 관리본·mise 심·PATH는 변경하지 않았고 reclaim도 하지 않았다.
실제 cleanup 안전성 후보는 아직 통합·설치되지 않았다. 새 설치본을 사용하는
후속 native full·source push·회수 영수증 전에는 그 전달 완료를 주장하지 않는다.

## 설치 후 native 전달 결과

`0b178b3f`의 native finish는 2026-10-19T03:31:38Z BLOCKED로 종료했다.
[원본 영수증](post-adoption-blocked.json)은 resource wait 29m2.284s,
validation 17m39.737s를 기록한다. 후보 make lint는 통과했다. 후보 make check는
`delivery.md`, `validation.md`, ISSUE-54의 Markdown format 오류로 종료했다.
긴 Go 테스트는 기준선 비교에서 실행됐으며 후보의 검사 통과 증거가 아니다.
기준선은 file:line 진단 2개를 반환했고, 후보 포맷 오류는 같은 형태가 아니어서
non-worsening을 판정할 수 없었다. source push·회수는 실행되지 않았다.

후보 문서만 저장소 지정 mdformat으로 보정하고 `make format-check`를 먼저 검증한다.
체커·기대값·검사 범위는 바꾸지 않는다. 보정 후 native full 전달을 다시 판정해야 한다.

## 최종 native 검사와 재기준화

`a8a17a2b`의 검사는 resource wait 21m34.493s, validation 16m52.620s 후
2026-10-19T04:11:54Z에 stale-base로 거절됐다. 원격 source가 `ed395f5d`로
바뀌어 기준선 측정과 post-gate freshness를 완료하지 못했다.
[원본 영수증](stale-base-refusal.json)을 보존한다. 후보 포맷 오류는 사라졌으나
후보 make check 자체는 `tests/parity`의 installed CE `d7c88ff6` 대 goldens
`42363c6e` 불일치로 종료했다. 단위 로그의 여러 PASS가 full PASS를 뜻하지 않는다.
원본 진단은 `~/.local/state/gz-git/integrate/diagnostics/make-check-1127702178.log`다.
기준선 비교 미완료이므로 non-worsening도 주장하지 않는다.

깨끗한 pushed 작업을 원격 `ed395f5d`로 rebase했다. CHANGELOG의 양쪽 항목을
모두 보존해 유일한 충돌을 해결했다. 다른 동작은 덮어쓰지 않았다. 원격이 이미
0.10.5를 사용하므로 이번 후보는 patch 0.10.6으로 준비한다. 이전 0.10.5 기록은
당시 후보의 역사적 버전이며 설치 승인 SHA e1a4bb5a는 변경하지 않았다.

미해결 CE toolchain 불일치는 기존 [ISSUE-41](../../../tasks/issue/41-ce-parity-golden-drift.md)에
연결한다. 기대값 재녹화·검사 약화·CE 자동 설치는 하지 않는다. CE 정합성 해결과
현재 source 기준 native full·통합·push·회수 전까지 이 전달은 미완료다.

## CE 42363c6e 채택과 재전달 준비

2026-10-19 사용자 “권장안으로 작업 진행해줘” 요청으로 CE 기준 설치를 실행했다.
canonical 소유 source의 기본 master `d07e098f`는 보존하고, 지정 build 경로에
소비 전용의 clean `42363c6e` clone을 만들었다. 원본 v0.10.0 태그 object도 대조해
정확한 describe `v0.10.0-289-g42363c6e`를 확인한 뒤 `make install_cli` exit 0을
검증했다. [CE 설치 증거](ce-adoption.json)에 승인·원본·고정 경로·실행 파일 해시·
설치 로그·설정 해시를 보존한다. 설치본 revision은 전체 42363c6e SHA이며 dirty=false다.
독립 strong 읽기 전용 검증은 PASS다. 플러그인·모델 설정·PATH는 바꾸지 않았다.

설치용 source는 CLI에 경로가 기록되므로 설치본 사용 중 보존해야 하는 자원이다.
경로는 `/Users/archmagece/mywork/ce/ce-agent-kit/build/installation-sources/ce423-mst-20261019`다.
종료된 authoring worktree나 새 개발 source로 쓰지 않으며 자동 정리하지 않는다.

동일 후보에서 non-record `go test ./tests/parity/... -count=1 -timeout=5m`을 실행해
35.277초에 exit 1을 확인했다. CE version guard는 해소됐고 남은 차이는
finish-explicit line183과 finish-noarg-single-active line260의 물리 processHost
(MacBookPro 기대값 대 mst 관측) 두 건이다. 기존 ISSUE-41에 같은 host literal
문제가 기록돼 있다. fixture/normalizer를 바꾸지 않고 최신 source의 실제 baseline과
native 비교로 판정해야 한다. 좁은 실패만으로 full non-worsening을 주장하지 않는다.
원격 source는 준비 중 `9d88a2b6`으로 이동했으므로 새 기준의 resource-plan 변경과
기존 cleanup 검증을 함께 보존해 재기준화하고 독립 검토를 받아야 한다.

## resource-plan source 재기준화의 호환 검사

source `9d88a2b6`의 새 resource plan·Python 준비 기능과 cleanup 안전성을 결합했다.
유일한 readiness.go 충돌은 양쪽의 `addPreparedWorktree` 사용과 생성 SHA/물리 신원 캡처를
모두 보존했다. 원격 source의 슬롯 분류·준비 환경·runner binding은 덮어쓰지 않는다.

결합 직후 lint는 0 issues였으나 기존
`TestReadinessPythonUVPreparesSourceEnvironment`가 6.56초에 RED였다.
원인은 정상 준비 환경 `infra-ops-python-uv-v1/source/venv`가 남아 root 삭제가
`directory not empty`로 거절된 것이다. 실패시 자료 보존은 작동했으나 정상 정리가
새 profile과 호환되지 않았다. 전체 선택 실행은 exit 1(59.568s)이었다.

수정은 준비 성공 직후 그 profile의 정확한 소유 container를 캡처하고, root 신원과
등록 worktree 부재를 확인한 뒤 캡처된 경로/physical identity를 다시 검증해 정리하는
범위로 제한한다. symlink·상위 경로 escape·동일 경로 교체·불명확한 부분 준비는
거절하고 root를 보존한다. 전체 root 재귀 삭제나 기존 실패 경로 강제 삭제로 우회하지 않는다.
구현 standard, 독립 안전성/충돌 리뷰 strong이며 최종 native 판정은 아직 미완료다.

수정 후 cleanup·신원·profile cache·resource plan·Python UV·ND 준비를 결합한
선택 회귀 검사는 exit 0(128.124s)이다. 새 테스트 helper 세 곳의 thelper 지적은
수정했고 `golangci-lint run ./pkg/integrate/...`는 0 issues다.
`make format-check`와 `git diff --check`도 exit 0이다.
별도 strong 에이전트의 실제 diff/관련 경로 읽기 전용 검토는 blocker 없이 PASS다.
`VERIFY_BASE=origin/master make verify-commit`은 CHANGELOG.md 영향으로
full 확대(exit 3, Make exit 2)를 반환했다. 이것은 commit profile 통과가 아니며,
native integrator의 전체 검사·baseline 판정·통합·push·회수가 남아 있다.

## CE 42363c6e 실행의 실제 baseline 판정과 stale-base 거절

후보 `ff4f680a`의 native 실행은 2026-10-19T07:47:59Z에 BLOCKED로 끝났다.
[실제 CE 영수증](ce423-stale-base-refusal.json)은 source `9d88a2b6` 대비
`make check`의 baseline failure non-worsening **3 → 3**, 변경 경로 진단 없음,
`make lint` PASS를 기록한다. 진단 `make-check-2281117101.log`의 실패는 기존
CE parity finish 두 시나리오의 processHost literal 차이와 그 상위 TestCEParity다.
fixture·기대값·정규화는 변경하지 않았다. 자원 대기는 1h0m25.887s,
실제 validation은 33m51.61s다.

최종 freshness는 source가 `d0c1197bbe53af7eaee5cb3a68fb8b9b5d68d817`로 전진해
FAIL이다. source push·worktree/local/remote branch 회수는 모두 false다.
이 판정을 통합 성공으로 바꾸지 않는다. 새 source의 Go 1.27 CI 품질 검사 복구,
release gate 이슈 기록, bin/tools mdformat 고정 변경을 보존해 재기준화한다.
CE pin은 여전히 42363c6e이며 이번 source 전진은 새 CE 설치 승인이 아니다.

2026-10-19T08:26:45Z의 다음 실행도 [stale-base 영수증](ce423-second-stale-base-refusal.json)을
남기고 BLOCKED로 끝났다. 후보 `618784c7`의 검사 중 source가 `d0c1197b`에서
`c3175483`으로 전진했다. 이 source 증분은 동시 통합 비용 ISSUE-61·62 문서 두 파일이다.
candidate 검사 뒤 baseline과 lint의 자원 입장 시점에 변경을 감지해 유효 판정을 얻지 못했다.
이번 실행의 make check/lint를 PASS로 기록하지 않는다. 자원 대기 16m16.746s,
validation 15m43.538s이며 source push·회수는 모두 false다.
기존 구현·리뷰·실패 영수증을 보존해 새 기준으로 재개하며, 원인은 기존
`tasks/issue/061-concurrent-landing-reverify.md`에 이미 등록돼 있어 재발급하지 않는다.
