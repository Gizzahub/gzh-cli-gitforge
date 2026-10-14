# ISSUE: 호스트 ce 바이너리 드리프트로 CE parity 골든 검증이 전면 실패한다

- status: open (2026-10-14 — provider 설치 갱신으로 최종 source 재검증 실패·갱신 완료 대기)
- previous-status: closed (2026-10-02 — 제안 2)대로 `PARITY_RECORD=1` 재녹화로 해결; 아래 해결 기록)
- priority: P1
- category: quality/parity
- created_at: 2026-10-01
- owner: `gzh-cli-gitforge/tests/parity/testdata/golden`
- work-card: 없음 — 신규 발견, 재녹화는 메인테이너 결정

## 2026-10-14: 현행 참조 정합성 후속 실행

사용자가 잔여 작업 분석의 권장 순서대로 실제 후속 작업을 승인했다.
새 delivery `ce-reference-current-adoption`은 source `eafb1c19`에서 시작했다.
앞의 거절된 후보와 소진 예산, 다른 실행의 자원·결정은 재사용하거나 변경하지 않았다.

이전 관측의 CE `c4deec42`를 대상으로 준비했지만 실행 전 설치본이 바뀌어
첫 pin 검사는 12개 시나리오에서 거부됐다. 골든은 하나도 생성되지 않았다.
설치를 실행하거나 이전 바이너리를 강제하지 않고 현행 출처를 다시 확인했다.
정상 PATH의 clean CE는 source `8381300b9bdcd105219c3075a400a0496d3b82f0`,
clean gz-git은 source `eafb1c19ff48a7cd893ebc7ec2c672f38bf8d0f1`이었다.
두 바이너리의 SHA-256은 다음과 같다.

- CE: `32b68b759a1042f29d6513862a62627ad0e0c75885c70f13c4d0d9165646f37c`
- gz-git: `3e1537714c9b0baf1ac5818655c5c8c7b7b1aabf86aeb51d5bc5d15496706911`

현행 CE의 기계 캡처와 reference 재검증은 각각 12 PASS, 0 fail, 0 skip,
종료 코드 0이었다. 전후 바이너리 해시도 동일했다. 독립 strong 리뷰는 CE/gz-git
식별자, receipt 도구 버전, physical processHost 2건, pre-mutation-outcome
도움말 4건으로 차이를 분류했다. lifecycle 상태·종료 코드·action/reason 의미,
소유권·source/target/ref·worktree·cleanup 의미 변화는 없었다.
CE ADR-0116의 결합된 push 이전 거절 처리와 provider의 공개 계약이 정합한다.
pin과 골든·계약 문서만 갱신하며 ISSUE-42 native port 수용을 주장하지 않는다.

전체 canonical `make quality-check`는 종료 코드 0으로 통과했다. 포맷·lint·보안·빌드·
단위·package integration·E2E·critical coverage를 기존 계약 그대로 실행했다.
그러나 최종 리뷰에서 전체 검사 도중 다른 실행이 CE를 clean `9877764d`로 교체했음을
확인했다. `8381300b` 후보는 통합하지 않았고 위 통과는 그 후보의 과거 결과로만
보존한다. 현재-runtime 정합성은 해결되지 않았으므로 이슈를 닫지 않는다.
새 설치본에는 ADR-0117의 task-runtime 변경이 있어 stamp-only로 취급하지 않는다.
사용자의 현행 참조 후속 해결 승인 범위에서 실제 설치 `9877764d`를 한 번 더
검토·캡처하며, 이후 runtime이 다시 바뀌면 자동 재캡처하지 않는다.

최종 `9877764d` 캡처와 reference 재검증은 각각 12 PASS, 0 fail, 0 skip,
종료 코드 0이다. canonical `make quality-check`도 종료 코드 0이다.
캡처 전·캡처 후·재검증 후·전체 품질 검사 후의 네 해시 manifest는 모두 동일하다.
CE SHA-256은 `4555b56bc1fdb883215e1dc3d03eddd9d76a188b744d06ea2946c6531c640d5f`이며,
provider SHA-256은 위와 같다. 독립 strong 리뷰에서 838→987의 12개 골든 차이는
CE identity/stamp만이었다. ADR-0117의 새 legacy reconciliation 경로는 이 시나리오가
실행하지 않으므로 그 경로의 동등성이나 native-port 채택을 주장하지 않는다.
이 증거로 현행 설치본과 참조의 불일치를 해결하며 기존 모든 거절·드리프트 관측은 보존한다.

그러나 전달 전 source가 `0ff0b221`로 이동해 명시적으로 재기준화하고 canonical
품질 검사를 다시 실행하는 동안 provider도 clean `0ff0b221` 설치본으로 바뀌었다.
재검사는 exit 2이며 `doctor-active`, `discard-no-fetch` 두 참조가 provider 도움말
차이로 실패했다. CE는 `9877764d`를 유지했다. 앞의 PASS는 당시 CE/provider 쌍에
대한 관측이며 최종 현행 정합성 해결·통합 완료의 증거가 아니다.
사용자는 다른 세션의 설치 갱신이 진행 중이라고 확인했다. 따라서 자동 재캡처나
경쟁 설치를 더 하지 않고 후보 branch와 증거를 보존한다. 설치 갱신이 끝난 뒤
소스·실행본 출처와 해당 공개 계약 차이를 다시 검토해야 하며 이 이슈는 열린 상태다.

증거 로그와 바이너리 해시는 소유 source의 `tmp/ce-reference-current-adoption/`에
보존하며, 이 경로는 authoring source나 새로운 fixture 후보가 아니다.

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
