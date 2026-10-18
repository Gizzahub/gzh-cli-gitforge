# ISSUE: 호스트 ce 바이너리 드리프트로 CE parity 골든 검증이 전면 실패한다

## 42363c6e 추가 채택 (2026-10-18)

- 상태: 참조 정합화 검증 완료 — 최종 전달은 새 native receipt가 판정한다.
- 사용자가 승인한 clean CE source·설치본은 full revision
  `42363c6eb4213e496748f329365cb4293b97e850`이다. 공식 `make install_cli`와
  소유 Go·Python 회귀 검사, 배포 전 독립 strong 검토를 확인했다.
- 12개 기계 캡처 20.585초, 별도 전체 non-record 17.712초 성공. 직전
  `8f7c2963` 대비 decoded 차이 37개는 CE identity뿐이다. 시나리오·normalizer와
  상태·종료·권한·ref·push·cleanup 결과는 바꾸지 않았다.
- issue resolve 거절·readiness cleanup 실패는 해당 12개에서 호출하지 않는다.
  소유 회귀와 capture의 검증 범위를 구분하며 이전 전달 이력은 보존한다.
- 현재 후보의 배포·검사·전달 경계는
  [42363c6e 채택 기록](../../reports/2026-10-18-ce-parity-reference-42363c6e.md)을 따른다.

## 8f7c2963 추가 채택 (2026-10-18)

- 상태: 참조 정합화 검증 완료 — 최종 전달은 native receipt로 확인한다.
  통합 대기 중 소스·설치 CE가 clean `8f7c2963`으로 변경됐다.
  사용자가 추가 후보의 의미 검토·회귀 검사·기계 재수집 후 채택을 승인했다.
- 소유 CE의 source resolver·native completion·decisions 검증 관련 집중 검사
  통과. 재수집한 12개 시나리오의 decoded 차이는 CE identity 37개뿐이며
  normalizer·시나리오·상태·종료·권한·ref·push·cleanup은 변경하지 않았다.
- targeted record 62.939초와 별도 전체 non-record 44.028초에 성공했고,
  검사 전후 실행 파일 해시가 일치했다.
- 아래 ef33833b 결과는 역사적 관측이다. 현재 검증과 최종 native 전달 근거는
  [채택 기록](../../reports/2026-10-18-ce-parity-reference-ef33833b.md)의 현재 후보 절을 따른다.

## ef33833b 후속 참조 채택 (2026-10-18)

- 상태: resolved — 참조 정합화와 독립 strong 검토 완료. 최종 전달 판정은
  native receipt가 소유하며, 아래 기록은 검증 당시의 관측이다.
- clean CE 소스와 설치본이 모두
  `ef33833b542cbe84be842f8a77293e61bce449ea`로 일치하는 것을 확인했다.
  기존 `ad92901e` 참조와의 불일치는 짧은 사전 검증에서 재현했다.
- 12개 시나리오 기계 캡처 18.222초, 계약 문서 갱신 후 별도 full non-record
  검증 21.213초 성공. 현재 참조·toolchain 행 검사를 강화한 뒤 최종 non-record도
  24.881초에 성공했다. 최초 non-record의 계약 revision 누락 오류는 문서를
  함께 갱신해 해결했다. normalizer와 시나리오는 변경하지 않았다.
- decoded diff의 39개 leaf 중 37개는 CE identity이며, 나머지 둘은 recover
  거절 reason·stderr다. CE `e5fc498c`의 bound native completion 복구와
  `3b7df148`의 dangling-link 방어를 계약 §6에 반영했다. 단순 stamp 변경으로
  분류하지 않는다. 기존 12개 시나리오의 상태·종료·ref·정리 결과는 불변이다.
- 새 복구 성공 경로는 기존 12개 캡처 범위 밖이다. 소유 CE의 positive·위조
  증거·자원 잔존·비조상·dangling-link 회귀 검사는 0.493초에 성공했다.
  이 결과는 consumer capture나 전체 게이트 성공을 대신하지 않는다.
- 검증 중 TASK-289가 source `abee75f2`에 반영돼 재배치했다. 그 참조
  `7140701a`는 후보 `ef33833b`의 조상이다. 상류 제품 코드와 `config[0]` 진단을
  보존하고 같은 후보로 12개를 새로 기계 캡처했다(19.245초). 별도 non-record
  19.247초와 충돌 해결 strong 검토가 통과했다. 새 baseline 대비 decoded diff는
  identity 37개·literal physicalHost 2개이며 status·종료·ref·정리는 불변이다.
- 재배치 전 실제 make check·lint는 통과했으나, source 이동으로 native 통합은
  거절됐다. 재배치 이후의 전달·정리 결과는 최종 native receipt로 확인한다.
- raw 근거와 검토 범위는
  [채택 기록](../../reports/2026-10-18-ce-parity-reference-ef33833b.md)에 있다.
  과거 실패·해결 기록은 아래에 당시 관측으로 보존한다.

## 참조 정합화 (2026-10-18)

- status: resolved (참조 불일치만 해결; native 통합 게이트는 별도)
- 사용자가 승인한 후속 계획에 따라 현재 clean CE 0.10.13
  `ad92901e322e3c64f350363feecc9c8b6b3807c3`을 채택했다. 소스와 설치된
  실행 파일의 전체 revision 및 clean Go metadata가 일치한다.
- 열두 시나리오 기계 캡처 22.873초, 별도 full non-record 검증 26.745초 성공.
  normalizer는 변경하지 않았다. 아래 과거 실패 관측은 그대로 보존한다.
- 독립 strong 검토에서 provenance stamp·물리 processHost·task 포함 관계를
  검증하는 성공 진단의 추가로 분류했다. 상태·권한·ref·정리 결과는 불변이다.
  세부 계약과 재현 경로는 `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md` §7.

### 실행 제한 진단 정정

아래 과거 관측의 contract runner 15분 원인 설명은 실제 실행 경로와 맞지
않는다. 보존된 protocol은 `legacy-make`이며 readiness contract runner를
호출하지 않았다. 설치 provider `b362607`의 legacy 경로는 이미 선언된
`branch.makeTimeout: 67m`을 source와 baseline에 적용한다. CE native adapter도
별도의 전체 15분 제한을 만들지 않는다. 원시 protocol에는 typed
`readiness_failures`가 없고 resource wait 17,938ms, validation 880,627ms와
`not ready`만 남았다. 따라서 이 기록만으로 약 15분의 중단 원인을 확정할 수
없다. 추측에 따른 timeout 코드·설정 변경 없이 확인된 source 검사 실패를
수정한 뒤 동일한 검증 provider로 native 게이트를 재시도한다.

## 재발 관측 (2026-10-18, readiness approval modes)

- status: reopened — 참조 채택은 별도 소유 작업이며 이번 기능에서 변경하지 않는다.
- 후보 `67232de27985a0250b53d64393d19441e60f2837`의 native 통합 검사에서
  golden pin `9fe1ce4506629ce5fa98023d0a8469e336130488`과 설치된 clean CE
  `1734ce1f79b385b629f176afc1f51831f46beded`의 불일치로 guard가 실패했다.
- 후보와 기준선 `f96bdca495489380347e976811aacd7db8748a42`의
  `tests/parity` tree는 모두 `62d0dd511c263b9ebca48c5309b329d9cfd58b1d`다.
  전체 기준선 실행은 완료되지 않았으므로 비악화 게이트 통과로 해석하지 않는다.
- 원시 검사 로그: `/Users/archmagece/.local/state/gz-git/integrate/diagnostics/make-check-1496353676.log`.
  native 결과: primary checkout의 `tmp/readiness-approval-finish-67232de2.json`.
- 설치된 gz-git `b362607ee19079b90950ff6fa27b269da30d1f8b`의
  `pkg/integrate/readiness.go`는 contract runner 전체에 15분 제한을 건다.
  source 검사 14분 40초와 슬롯 대기 이후 기준선 비교가 deadline으로 중단됐다.
  `branch.makeTimeout`은 legacy make 경로에만 적용된다.
- 같은 실행에서 발견한 신규 policy 명령의 read-only 테스트 등록 누락은
  이번 기능 범위에서 수정했으며 관련 read-only 테스트는 통과했다.
- 통합·설치는 수행되지 않았다. CE pin 변경, golden 재녹화, 구버전 강제,
  게이트 우회는 이번 기록의 범위가 아니다.

## 2026-10-16 후속 참조 정합화

`verification-evidence-followup`의 현재 후보에서 네 시나리오 차이를 재현했다.
CE pin은 clean `9fe1ce45`와 일치하며 이번 provider help 두 곳과 물리 호스트명
두 곳의 차이다. 기존 계약에 따라 12개를 전부 기계 캡처했고 실제 네 golden만
변경됐다. pin·normalizer·schema를 바꾸지 않았다. 과거 원문 유실 진단의 원인
규명이나 전체 native 통합 성공으로 표시하지 않는다. 생성·재검증·독립 리뷰와
최종 전달 상태는 `reports/2026-10-16-verification-evidence-followup.md`에 기록한다.

- status: closed (2026-10-14 — clean CE 9877764d 참조 채택·canonical 품질 통과)
- previous-status: closed (2026-10-02 — 제안 2)대로 `PARITY_RECORD=1` 재녹화로 해결; 아래 해결 기록)
- priority: P1
- category: quality/parity
- created_at: 2026-10-01
- owner: `gzh-cli-gitforge/tests/parity/testdata/golden`
- work-card: 없음 — 신규 발견, 재녹화는 메인테이너 결정

## 증상과 원인

parity 스위트는 PATH의 `ce`를 실행해 커밋된 골든(`pinnedCECommit = 771c54cf`, 빌드 `404-g771c54cf`)과 diff한다. 호스트 ce-agent-kit이
`475-g950650ef`로 올라가면서 run-status의 `nextAction` 문구가 길어지는 등
출력이 바뀌어 TestCEParity 서브테스트 11개가 실패한다:

abort-terminal-noop, discard-no-fetch, discard-refusals, doctor-active,
finish-explicit, finish-noarg-single-active, finish-refusal-unpushed,
owner-mismatch-takeover, reclaimed-run-reconciliation,
recover-requires-cleanup-failure, start-created-existing

`make quality-check`의 `test-unit-quality`가 이 스위트를 포함하므로 ce가
드리프트된 머신에서는 게이트 전체가 실패한다.

## 격리 근거 (2026-10-01)

- master(primary 체크아웃 `74a3869`)에서 동일 11개 서브테스트 실패 —
  TASK-260 변경(make 예산)과 무관. diff 내용이 전부 ce 버전 메타데이터와
  ce 출력 문구다.
- 스위트 주석의 선언된 대응은 "toolchain drift → re-recording"이며, 골든
  재녹화는 참조 ce를 새 버전으로 인정하는 의사결정이므로 메인테이너가
  한다. 본 카드는 재녹화하지 않았다.

## 해결 기록 (2026-10-02)

메인테이너 결정으로 제안 2)를 택해 골든을 `475-g950650ef` 기준으로
재녹화했다 (`PARITY_RECORD=1 go test ./tests/parity/...`, 11개 시나리오).

- 이동 성격: stamp-only가 아니었다. word-diff 검증 결과 변경은 ① 버전
  스탬프(771c54cf/404-g → 950650ef/475-g), ② run-status `nextAction`
  가이드 문구 확장, ③ 캡처된 `gz-git integrate --help` 출력의 신규
  `Effect: mutating (...)` 라인 — 전부 캡처된 출력 문구며,
  스키마/상태/allowedActions는 불변.
- 수반 변경: `pinnedCECommit` 상수 → `950650ef`,
  `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md` 핀·이력 갱신(파일 전체가
  mdformat 정규화됨 — 변경 파일은 전체 검사 대상이라 필연).
- 근거: 재녹화 후 로컬 `make quality` 전체 통과(parity 포함). hosted CI
  parity 단계는 푸시/통합 시점에 확인한다.
- 후속: ce 드리프트 시 스위트가 실패 전단계에서 재녹화 안내와 함께
  실패하는 가드(parity-version-guard)를 별도 과제로 진행한다.

## 재발 관측 (2026-10-14)

DVA 최소 연결의 source 기준 `9e054cf4daae3b52fb55465741fe57815ad8e9f9`에서
참조 golden은 `cd0e1d19c3211b85a514ac03383ae6166a614f07`, 실제 PATH의 clean CE는
`b79c1a4a9ab9e53af55074a816d15a3ece9c1704`였다. 과거 재녹화 해결과 리뷰는
위 기록대로 보존하며, 새 issue를 중복 등록하지 않고 이번 재발만 추가한다.

source primary에서 다음 명령은 exit 1, golden guard에서 조기 실패했다.
DVA 연결은 Makefile·Go 소스·golden·CE pin을 변경하지 않았다.

```bash
env -u CE_BIN -u PARITY_TARGET -u GZ_GIT_BIN -u PARITY_RECORD \
  go test -short -count=1 ./tests/parity
```

`source-parity-baseline.log`는 설치 build와 12개 golden의 차이를 기록한다.
원시 증거는 소유 devbox `gzh-cli-devbox`의
`tmp/dva-project-tooling-evidence-20261014/gitforge/`에 보존한다.
이 등록은 재녹화·CE 설치·구버전 강제·fixture 예산 재설정 승인이 아니다.
참조 채택과 의미 차이는 기존 소유자가 독립 strong 리뷰로 판정해야 하며
전체 포트 추종 범위는 ISSUE-42와 분리한다.

## 계약 확인과 새 후속 범위 (2026-10-14)

ce-devenv의 `config/agents/agent-kit.toml`, 배포된 CE locator와 live snapshot의
동일 파일은 같은 소스를 가리킨다. 이는 소스 경로 일치이며 immutable revision
pin의 일치를 뜻하지 않는다. authoring source HEAD는 `9012626c`, clean 실행
바이너리는 `b79c1a4a`다. 실행 build metadata는 `vcs.modified=false`다.
따라서 source HEAD를 그대로 설치하거나 과거 cd0 바이너리로 강제하지 않는다.

`cd0e1d19`는 b79의 조상이며 그 사이 네 커밋의 net diff에 task-runtime endpoint
변경은 없다. b79 이후 현재 source까지도 해당 경로 변경은 관측되지 않았다.
그러나 12개 시나리오의 실제 출력 비교는 아직 없으므로 stamp-only 또는 의미
동등성을 확정하지 않는다. 원시 locator·binary·Git 관측은 devbox의
`tmp/dva-project-tooling-evidence-20261014/followup/source-runtime-context.log`에
보존한다. 이전 guard 실패·해결·리뷰 기록은 변경하지 않는다.

### 제안한 단일 후보 예산 — 아직 활성화하지 않음

TASK-318/321의 소진 예산을 재사용하거나 초기화하지 않는다. ISSUE-41에 연결된
별도 후속 실행을 선택할 때만 다음 신규 예산을 적용하도록 제안한다.

| 항목                     | 상한                                       |
| ------------------------ | ------------------------------------------ |
| 신규 참조 후보           | clean resolved b79 한 개                   |
| 기계 캡처                | 12개 CE-reference 시나리오 전체 한 번      |
| 독립 리뷰                | strong 한 번; 구현도 strong                |
| record 실행              | 10분                                       |
| focused reference 재검증 | 10분, zero skip                            |
| canonical quality        | 45분; 기존 package timeout은 변경하지 않음 |
| 독립 리뷰 시간           | 30분                                       |

실행·리뷰 합계 상한은 95분이며 기존 FIFO 자원 대기와 Git 통합 대기는 별도로
관측한다. 재시도·두 번째 후보·자동 시간 연장은 없다. 읽기 전용 preflight는
후보를 소비하지 않지만 실패하면 진행을 멈춘다. 새 참조로 record가 처음 시작해
proposed fixture를 쓰는 순간 한 후보·한 캡처를 소비한다. isolated evidence
copy도 이 횟수에 포함하며, 임시 복사본을 추가 후보나 authoring source로 쓰지 않는다.

활성화 전 source freshness·ACTIVE 소유권·resolved CE의 SHA와 clean build를 다시
확인한다. 허용 변경은 `tests/parity/harness_test.go`의 참조 pin, 기존 golden
12개, `docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md`, 이 이슈의 새 증거뿐이다.
엔진·native port·설치 pin·devenv·gz-git 버전은 제외한다.

캡처 후 모든 변경을 시나리오·구조 필드별로 분류한다. status·exit·reason/action
의미·owner·source/ref·worktree·cleanup의 변화, 미분류 차이, skip, timeout,
nonzero 또는 provenance 불일치가 있으면 후보를 거절한다. 캡처와 독립 리뷰를
통과하기 전 pin/golden을 수용하지 않는다. reference 후속은 ISSUE-42의 native
port 수용을 대신하지 않으며, outer quality의 nonzero를 helper 성공으로 상쇄하지 않는다.

이번 변경은 계획·관측 기록만 전달한다. 새 예산 채택·fixture 실행·재녹화·설치·
릴리스 또는 TASK-302 수용을 실행하지 않았다.

## Approved one-candidate execution — rejected (2026-10-14)

The user explicitly approved the proposed bounded reference execution. A fresh
delivery run `issue-41-reference-adoption` started from clean source
`7840a4a2635c53d1c7d518e042fb3d063c3210c6`; exhausted TASK-318/321 runs and
their retained resources were untouched. Implementation and separate independent
review requested strong capability (observed models gpt-6 and gpt-6-astra).

The sole capture used the resolved clean CE executable `/Users/archmagece/go/bin/ce`,
revision `b79c1a4a9ab9e53af55074a816d15a3ece9c1704`, SHA-256
`665e88b710344a7445b90cc800b3b35488846aa8d01dc1b4d3a2f87dace7c6fe`.
Capture and focused verification completed naturally in 14.942 and 15.436 seconds:
each had all twelve distinct reference scenarios PASS, zero skips/failures,
exit 0, and unchanged CE executable hash. No second capture occurred.

Independent recursive comparison found 42 changed fields: 24 CE reference/toolchain
identity fields, 9 receipt `tool_version` fields, 8 provider help strings, and
one physical `processHost` observation. Lifecycle exit/status, logical owner,
source/ref, worktree and cleanup fields were otherwise unchanged. However,
`doctor-active` and `discard-no-fetch` help removed `--expect-target` and narrowed
exit 4 from approved source/target-pair changes to source-only changes. This is
advertised public capability/approval-contract drift, not a harmless stamp fold.

The actual clean integration provider is `f012c9f8599f3d5de9ff3ab6b2dce799ad541cba`,
while the existing contract records `96d5ecd`. Both report 0.9.1, so version text
alone did not establish equivalent provenance. This is not evidence of a CE
engine regression. The adopted meaning/provenance stop condition rejects the
candidate. The proposed pin and all twelve generated golden changes were removed;
the existing `cd0e1d19` reference and contract remain unchanged.

The rejected diff SHA-256 is
`43e0cf350a09365d833765eff413fe22bc27db34c385219a7a67823b9da1c69a`.
Capture/result receipts, field comparison, CE metadata and the exact candidate
patch are preserved under the owner devbox's
`tmp/issue-41-reference-adoption-20261014/`. The original worktree evidence was
`tmp/issue-41/`. These are audit observations, not adopted fixtures or TASK-302
acceptance evidence. No engine, native-port, tool pin, global executable, PATH,
model/provider binding, or source version was changed.

No further candidate or automatic runtime installation follows this rejection.
Resolve the integration provider's official-release/adopted-runtime provenance
through its existing owner workflow, then explicitly decide a new reference
disposition. ISSUE-42 port acceptance and TASK-302 C1–C5 remain separate. The
existing ACTIVE ISSUE-304 execution was not touched.

The rejected fixture bytes were restored before the separate canonical source
measurement. Existing DVA project-local provisioning completed with exit 0;
repository-adopted tool versions and the active Go 1.27.2 were preserved, with no
global installation or new dependency selection. `dva quality-check` delegated
to the existing `make quality-check` and naturally completed in 461.297 seconds,
exit 2, without timeout or interruption. Format, lint and security-code passed;
govulncheck found zero called vulnerabilities. Unit aggregation failed at the
unchanged CE-reference guard (`cd0e1d19` goldens versus clean `b79c1a4a`).
The integrate package itself passed in 385.763 seconds. Canonical stages after
the failed unit dependency were not claimed as executed. Raw `quality.log`,
`quality.stderr` and `quality-result.json` preserve this baseline failure; it is
not a PASS or a reason to weaken TASK-302's exit-zero acceptance requirement.
This delivery changes the issue record only.

## 2026-10-14: 별도 승인된 TASK-322 참조 재검증

위 거절 기록과 소진된 후보 예산은 보존한다. 사용자는 별도 TASK-322에서
최대 세 후보와 strong 독립 리뷰를 승인했고, native bare finish의 READY 유지
계약 복원을 선택했다. 또한 clean accepted source
`1095e751ab8546a04715b1393ab0d02ada3de9fb`를 Go 1.27.2로 빌드한 뒤 지정
`make install`로 설치하는 개발 source 예외를 명시적으로 승인했다.

실제 설치는 종료 코드 0이다. 정상 PATH의 `~/go/bin/gz-git`은 해당 source,
Go 1.27.2, `vcs.modified=false`와 일치하며 `--expect-source` 및
`--expect-target`을 제공한다. 중간 설치본의 bare finish는 DONE이며,
TASK-322는 READY 복원을 별도 검증한다. 병렬 바이너리나 PATH 우회는 없다.

이 설치 후 clean CE `b79c1a4a9ab9e53af55074a816d15a3ece9c1704`로
열두 참조를 실제 재캡처하여 PASS 12, skip 0을 확인했다. 참조의 provider
capability 누락을 정상화로 숨기지 않았다. 새로운 도움말의 선택적 timeout
진단 문구, 실제 receipt host 관측값, 승인된 READY 준비 시나리오를 분류했다.
같은 후보의 `mise exec -- dva quality-check`도 종료 코드 0이다.
원본 로그는 TASK-322 실행의 `tmp/task322/reference-record-after-provider-adoption.jsonl`
및 `tmp/task322/quality-after-provider-adoption.log`에 보존한다.

이는 source 통합이나 TASK-302 C1–C5 수용 완료를 의미하지 않는다. 최종
source 기준의 독립 리뷰, 변이 검증, 실제 통합·push·회수 증거가 남아 있다.
전체 native 열두 시나리오의 byte parity도 주장하지 않는다.

## 2026-10-14: 0.9.2 공식 릴리스 준비

별도 delivery 실행 `release-0-9-2-preparation`에서 버전과 릴리스 준비 문서를
정리한다. 고정 GoReleaser 2.18.2의 설정 검사와 비게시 snapshot 빌드는
exit 0이며 5개 archive의 digest를 확인했다. 상세 명령과 검증 상태는
`docs/releases/0.9.2-preparation.md`에 기록한다. TASK302 활성 실행은 보존한다.

공식 게시 전 GitHub workflow의 secret·게시 권한 확인이 남는다. 현재 토큰의
secret 이름 조회는 HTTP 403이며, 원격 secret 부재로 판단하지 않는다.
이 기록은 릴리스 게시·stable 설치·TASK302 인수 완료를 의미하지 않는다.

## 2026-10-14: Python 준비 이슈 등록 중 mst 기준선 재확인

delivery 실행 `python-uv-prepare-gap`의 기준 소스
`233eb907d80e90e0cb0f3b6bcc1b1abdfe6e2d84`에서 `make quality-check`는 exit 2였다.
포맷, lint(0 issues), gosec(0 issues), govulncheck(영향받는 취약점 0), 빌드와
install audit는 통과했다. 단위 검사에서 `pkg/integrate`는 718.978초에 통과했지만,
parity guard가 실제 설치 CE `c4deec428559ea49a9b5a066ddfe727045b00a37`와
현재 골든의 `b79c1a4a9ab9e53af55074a816d15a3ece9c1704` 불일치를 거부했다.
전체 품질 게이트 성공이 아니며, 뒤의 package integration/E2E/critical coverage
단계는 실행됐다고 주장하지 않는다.

변경 없는 review 실행 `python-uv-baseline`은 당시 최신 source
`ddf1276c47126d7bb004c0726dbbe1d1aae66423`에서
`GOWORK=off go test -count=1 ./tests/parity/...`를 실행해 같은 guard와 exit 1을
재현했다. 두 source 사이 변경은 README와 별도 이슈 카드뿐이며 parity 코드·골든,
Go manifest, Make 품질 계약은 같았다. 이번 등록 역시 문서 변경만 포함한다.
따라서 이 CE 설치/참조 불일치는 Python 준비 이슈 등록의 회귀가 아니다.

이 관측은 앞의 별도 승인된 참조 재캡처 결과를 무효화하거나 설치 이력을
소급 수정하지 않는다. 설치된 CE와 선언된 참조를 소유 계약에 따라 다시 맞추는
작업은 이 기존 이슈의 후속이다. 이번 등록에서는 CE 설치·PATH·골든·검사 상한을
바꾸지 않았고, 같은 문제가 이미 등록돼 있어 새 parity 이슈는 만들지 않는다.

## 2026-10-14 sibling lifecycle 문서 전달 중 재확인

설치 CE가 `c4deec428559ea49a9b5a066ddfe727045b00a37`로 전진한 뒤,
`sibling-provision-evidence` 작업의 `make quality-check`는 앞선 unit 검사를
통과하고 parity TestMain에서 exit 2로 중단했다. 12개 golden은 계속
`cd0e1d19c3211b85a514ac03383ae6166a614f07`를 참조한다. 이후 integration,
E2E, coverage 단계는 실행하지 않았다. `make check`는 통과했다.

canonical source primary `b32360a`의 직접 `go test ./tests/parity`도 같은
installed-build guard에서 exit 1로 실패했다. primary, 현재 target
`bf460da`, 문서 작업 `5599474`의 `tests/parity` Git tree object는 모두
`675f90fb8c04470b3a4aa84886b2cd73d7a8ba16`이며, 해당 parity package의 Go 소스와
fixture는 같다. `bf460da..5599474`의 작업 변경은 문서 3개뿐이며 Go・test・tool
경로를 바꾸지 않았다. 실패는 scenario 실행 전에 동일한 TestMain guard에서
발생하므로, 이 문서 작업이 baseline 실패를 악화하지 않았음을 독립 strong AI
검수로 확인했다.

원시 로그는 `/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/`의
`sibling-evidence-rebase-quality.log`와 `sibling-evidence-baseline-parity.log`다.
이 기록은 기존 ISSUE-41의 재현 갱신이며, 새 CE pin 채택이나 golden 재녹화,
rollback, waiver를 집행한 기록이 아니다.

## 2026-10-14 최신 harness 참조 채택

기존 후보 `9742f8b`는 최신 golden 12개와 rebase 충돌해 선택 없이 취소하고
CE run-abort로 보존했다. 설치·통합 프로세스와 executable hash를 조회한 뒤
clean gitforge `1503edf`를 지정 `make install`로 반영했다(경로 audit 포함 exit 0).
CE는 clean `9877764d`를 유지했다. source checkout의 추가 `3e4340e5`는 문서
3파일만 바꿨으며 CE 실행 코드는 선택한 revision과 같다.

최신 master의 좁은 provider metadata 정규화와 승인된 help를 유지한 새
`ce-reference-stable-adoption`에서 fixture를 기계 캡처했다. capture와 fresh
verify 각각 12 PASS/0 skip, canonical `make quality-check` 종료 코드 0이었다.
실행 전후 CE/provider hash가 동일했다. 원본은 source primary checkout의
`tmp/ce-reference-stable-adoption/`에 보존한다.

독립 strong decoded 비교의 39개 leaf 변경은 CE 참조/build 24개, receipt
`tool_version` 13개, 실제 processHost 관측 2개다. 상태·action·exit·논리 owner·
source/target/ref·회수·provider 명령/인자/help 변경은 없다. ADR-0117 경로는
이 시나리오에서 호출하지 않았으며 native port 완료를 주장하지 않는다.

## 2026-10-16 ISSUE-332 재베이스의 물리 host 기준선

source `6b8c62ad2e72636c1d6c2c2604cec493cd92e8c1`의 깨끗한 review worktree와
별도 version 수정 worktree에서 동일한 두 parity 시나리오를 실제 실행했다.
`finish-explicit`와 `finish-noarg-single-active` 모두 `scenarios_test.go:141`에서
exit 1이며, 두 JSON 차이는 각각 line 183·260의 `processHost`뿐이다.
정본은 `CE-Mac-Studio.local`, 현재 관측은 `CEui-MacBookPro.local`이다.
두 트리의 `tests/parity` Git tree는 동일한
`62d0dd511c263b9ebca48c5309b329d9cfd58b1d`다. 변경 경로에 새 parity 진단은 없다.

`processHost`의 foreign-host claim 보존 의미를 유지한다. golden 재녹화나
정규화 변경으로 차이를 지우지 않는다. 이 관측은 ISSUE-332의 버전 회귀와
분리된 기존 reference 문제이며, native gate가 실제 source/branch 비교로
non-worsening READY를 반환해야 전달할 수 있다. unmeasurable 또는 새 실패면
통합을 보류한다. 이것은 TASK-306의 새 후보나 소진 예산 재설정이 아니다.

## TASK-289 출력 계약의 제한된 후속

TASK-289 후보 f6d19f9a의 실제 native는 literal processHost 차이에 config[0]
출력 차이가 추가돼 진단2→3으로 거부됐다. 단위 pkg/integrate954.073s와 lint는
통과했지만 baseline full도 실패했고 source도 이동해 전달하지 않았다. 원본은
source primary의 tmp/task289-current/native-finish-rebased-primary.json과
current-outcome.json이다. 실행은 ABORTED, 후보와 자원은 보존했다.

사용자가 승인한 task289-parity-contract 후속은 모든 target의 출처 표시라는
기존 수용 조건을 계약에 명시하고 현재 clean CE7140701a를 채택한다. 기계 캡처와
별도 non-record 검증은12시나리오 PASS/skip0이며 normalizer를 바꾸지 않았다.
decoded diff48개는 reference24, receipt stamp13, literal physicalHost2,
provider prose7, recover 거부 문구2다. 새로운 native completion 성공 경로를
검증한 것은 아니다. 전체 native 수용·설치는 아직 별도 미완료다.
이 기록은 TASK-306/318/321의 소진된 시도를 재개하지 않는다.

소유 source primary의 `tmp/issue332-version-delivery-20261016/`에
`issue332-parity-baseline.log`, `issue332-parity-branch.log`와 baseline metadata를
보존한다. review worktree는 증거 복사 후 정식 discard하며 수용 근거로 쓰지 않는다.
첫 `b362607e` 기반 후보의 DVA full은 exit 0이었지만, 그 결과를 최신 source의
full PASS로 이월하지 않는다. 최종 전달 판정은 native 영수증이 소유한다.

## 2026-10-17 runner 예산 후보의 기준선 대조

readiness runner 예산 후보의 최초 native 전달은 설치 CE `e8307aa4`와 golden의 guard
불일치를 관측했다. 기준 source62459750과 후보72d5796b의 `tests/parity` Git tree는
동일하다. [전달 기록](../../docs/task-reviews/readiness-runner-budget/validation.md)과
[원본 영수증](../../docs/task-reviews/readiness-runner-budget/first-finish.json)을 보존한다.
이번에는 별도의 실행 잠금 테스트 진단이 source에 추가돼 native가 BLOCKED를 반환했다.
parity 재녹화·기대값 변경·통합 우회는 하지 않으며, 이번 결과는 전달 성공이 아니다.
