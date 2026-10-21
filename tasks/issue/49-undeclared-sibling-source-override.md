# ISSUE: 선언되지 않은 sibling `--source` 이름이 무시되어 선언된 source를 획득한다

- status: resolved (2026-10-22 — 아래 Resolution)
- priority: P2
- category: integration/provisioning
- created_at: 2026-10-14
- owner: `cmd/gz-git/cmd/integrate_provision.go: integrateProvisionAcquireCmd.RunE`

## 증상 및 재현

다음은 독립 실행 명령이 아니라, 실제 관측의 작업 디렉터리와 선언 전제조건을 포함한
재현 맥락이다. cwd는 retained EE worktree
`/Users/archmagece/worktrees/scripton-gitrump/gitrump-ee/codex__mst__refactor__ee-phase6-matrix-config-adapter`이고,
그 worktree의 committed source는 `ed65d4edc6dbe210737c5597d80d476be65940cf`이다. 관측 당시
working tree의 draft `branch.siblingProvision`은 `gitrump-ce`에 대해 remote
`ssh://git@gitlab.polypia.net:2224/scripton-cloud/gitrump/gitrump-ce.git`, path
`../gitrump-ce`, source branch `master`, revision `source-tip`을 선언했다.

그 상태에서 선언에 없는 source 이름을 명시한 아래 명령은 오류를 내지 않고 성공했다.

```sh
gz-git integrate provision siblings acquire \
  --source unknown=/Users/archmagece/mydevbox/scripton-gitrump-devbox/gitrump-ce \
  --json
```

실제 출력은 `ACQUIRED`였고, lease `0f880cd8face0842b90f350f2cfd2fb6`는 선언된
`gitrump-ce`와 CE `51df4133bffda8909edf2136eca4bc9f364270bf`를 기록했다. 즉 `unknown`은
source 선택으로 처리되지 않고 조용히 무시되며, 명령은 canonical declared source를
획득한다. stderr는 비어 있었다.

소유 경계는 `integrateProvisionAcquireCmd.RunE`의 source map 조회다. 이 경로는 선언된
sibling 이름만 조회하고, map에 남은 선언되지 않은 이름을 거부하지 않는다.

## 영향과 범위

사용자는 임의 source를 명시했다고 오해할 수 있지만, 실제로는 선언된 sibling archive가
만들어진다. 관측된 lease는 첫 EE coherent compile에서 재사용·복구 가능 상태로 확인한
정상 archive였고, HTTP audit 경계 전 clean release되었다. 이 결함 기록은 그 archive가
선언되지 않은 source를 선택했다는 증거가 되지 않는다.

이 카드는 결함 기록만 한다. 선언되지 않은 이름을 어떤 오류 계약으로 거부할지와 수정의
수락 기준은 별도 구현 작업에서 정한다.

## 증거

- `/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/ee-native-sibling-invalid.json`
- `/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/ee-native-sibling-invalid.log`
- `/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/ee-native-sibling-pre-audit-release.json`

## Resolution

Resolved on 2026-10-22 by `fix(integrate): reject a --source name no declared sibling matches`.

- `integrateProvisionAcquireCmd.RunE`(`cmd/gz-git/cmd/integrate_provision.go`)는 선언된
  sibling에 없는 `--source` 이름을 provisioning 전에 exit 2 usage 오류로 거부한다. 메시지는
  알 수 없는 이름과 선언된 이름을 모두 적는다.
- `--source`를 받는 서브커맨드는 `acquire` 하나다.
- `TestProvisionCLIAcquireRejectsUndeclaredSourceName`이 exit 2, 메시지, snapshot·lease 부재를
  검사하며, 수정 전 코드에서는 실패한다(독립 리뷰에서 확인).
- `docs/commands/integrate-sibling-provision.md`의 "무시된다" 제한 설명을 새 계약으로 바꿨다.
