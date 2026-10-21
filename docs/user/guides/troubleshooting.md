# Troubleshooting

Symptom-first guide for `gz-git`. For what a command does, see the
[command reference](../../commands/README.md); for what this project is, see the
[FAQ](faq.md).

## 개발 품질 도구가 없거나 준비 뒤에도 검사가 실패할 때

도구 버전·빌드 Go·설치 경로는 기존 `mise.toml`과 `.make/tools.mk`가 소유합니다.
DVA는 선언된 준비·검사 명령을 호출하며 버전을 다시 선언하지 않습니다.
프로젝트의 기존 mise 환경을 활성화한 뒤 실행합니다. DVA가 자동으로 mise를
설치하거나 활성화하지는 않습니다.

```bash
dva config validate
dva manifest -f json
dva ls
dva provision tools
dva quality-contract
dva quality-check
# 전체 품질 검사 대신 단위 검사만 필요한 경우
dva test-unit-quality
```

`provision tools`는 기존 installer 중 ignored `bin/tools`에 도구를 설치하는
대상만 호출합니다. mdformat도 uv로 `bin/tools`에 고정 설치되므로 여기에 포함되며,
uv가 필요합니다. `make install-quality-tools`는 gci를 전역 GOPATH/bin에 설치할 수
있어 이 로컬 profile에서 호출하지 않습니다. 전역 PATH·pin은 바꾸지 않습니다.
준비 target은 기존 Makefile의 module 버전과 active Go 빌드 검증을 유지합니다.

`dva quality-check`와 `dva test-unit-quality`는 원래 Make target과 입력 환경을
그대로 사용합니다. CE pin이 필요한 검사는 해당 작업의 승인된 `CE_BIN` 등 입력을
실행 환경으로 전달해야 합니다. `PARITY_TARGET` 등 이전 실행의 환경을 임의로
상속해 다른 검사를 실행하지 않도록 원래 검증 계약을 확인합니다.

도구 준비 exit 0이나 helper 두 개의 exit 0이 outer quality exit 2를 성공으로
바꾸지 않습니다. repository delivery, TASK-302 제품 수용과 Intent는 별도 증거로
판정합니다. `.sb/dva/`의 provision marker는 준비 실행 기록이며 현재 도구 버전의
증거가 아닙니다. 변경된 도구 계약은 준비 target을 다시 실행해 검증합니다.

### 2026-10-14 DVA 연결 검증 기록

source `9e054cf4`에서 설정 validate·manifest·ls·quality-contract와 실제 tools
provision은 exit 0이었다. 준비한 Go 도구는 기존 pin을 따르고 Go 1.27.2로
빌드됐으며, 바이너리와 provision marker는 ignored 상태다.

`dva quality-check`는 임의 중단 없이 exit 2로 종료했다. unit의 pkg/integrate는
532.202초에 통과했지만 parity guard가 source의 cd0e1d19 reference와 설치 CE
b79c1a4a 차이를 거절했다. primary source에서도 같은 guard 실패(exit 1)가
재현돼 기존 ISSUE-41을 재발로 열고 과거 종료 기록을 보존했다.
전체 quality·unit 성공으로 판정하지 않으며 뒤의 integration/E2E/critical coverage
단계가 실행됐다고 주장하지 않는다. 이번 연결에서 CE pin·golden·Go 소스는 바꾸지
않았다. 미매핑 Make 명령에 대한 DVA suggestion은 최소 부분 채택 범위 밖이며
경고를 숨기지 않았다.

원시 로그는 devbox `gzh-cli-devbox`의
`tmp/dva-project-tooling-evidence-20261014/gitforge/`에 보존한다.
실제 repository delivery는 CE receipt로 별도 확인한다.

### 후속 실행 전에 확인할 소유 계약

CE locator의 source·배포·live 경로 일치와 실행 revision 일치는 서로 다릅니다.
2026-10-14 관측은 source HEAD 9012626c, clean 실행 CE b79c1a4a, gitforge 참조
cd0e1d19입니다. 실제 fixture 의미 비교 전에는 stamp-only로 판정하지 않습니다.
[ISSUE-41의 단일 후보 후속안](../../../tasks/issue/41-ce-parity-golden-drift.md#%EC%A0%9C%EC%95%88%ED%95%9C-%EB%8B%A8%EC%9D%BC-%ED%9B%84%EB%B3%B4-%EC%98%88%EC%82%B0--%EC%95%84%EC%A7%81-%ED%99%9C%EC%84%B1%ED%99%94%ED%95%98%EC%A7%80-%EC%95%8A%EC%9D%8C)은
소진된 TASK-318/321과 별개이며 아직 활성화하지 않았습니다.

DVA의 [공식 latest stable](https://github.com/ScriptonBasestar/dva/releases/latest)은
v0.3.0(tag 629c34c1)입니다. ce-devenv의
`scripts/install-internal-go-tools.sh`도 그 정확한 태그를 요구하지만, 실제
DVA는 0.3.0/2a807f58 및 devel metadata를 보고합니다. 이는 설치기 결함의
재현이나 자동 downgrade 근거가 아닙니다. doctor 개선 source5a0570f9는
미공개이며 현재 설치본 지원으로 취급하지 않습니다.

DVA의 기존 [릴리스 로드맵](https://github.com/ScriptonBasestar/dva/blob/master/ROADMAP.md)과
[수동 릴리스 런북](https://github.com/ScriptonBasestar/dva/blob/master/docs/52-manual-release-runbook.md)이
후보·공개 절차를 소유합니다. 검증된 공개 stable 뒤 ce-devenv의 기록 버전·commit을
소유 source에서 갱신하고, 별도 승인된 설치기로 전달해야 합니다. release·global
CLI 설치·skill 설치를 소스 delivery로 대신하거나 자동 실행하지 않습니다.
현재 바이너리를 유지하고 PATH/mise 우회·구버전 강제는 하지 않습니다.

현재 internal Go 설치기는 DVA만 선택하는 공식 selector가 없고, agent-mesh·
flow-mcp 및 locator가 설정된 CE도 함께 재빌드할 수 있습니다. DVA-only 설치를
승인하기 전에 ce-devenv 소유의 기존 설치기에 명시적 대상 선택을 준비하고,
CE locator가 있어도 CE를 교체하지 않는다는 검증이 필요합니다. 임의 경로로
다른 도구를 숨기거나 locator를 지우는 테스트 격리 방식을 실제 설치에 쓰지 않습니다.
이는 개선 제안이며 현재 설치기 결함이나 새 selector 지원을 주장하지 않습니다.

CE 라우팅 안내는 다른 ACTIVE 실행의 delivery와 소유권 clear 후 기존 스킬에서
DVA 정본으로 연결합니다. 전체 후속 범위는 ISSUE-41, CE 라우팅, DVA stable
rollout의 세 가지이며, TASK-302 선행 조건 충족 전 이전 실행 묶음을 등록하지 않습니다.

## `info` shows `↑N` but `push` reports "up-to-date"

Both are right — they measure different things. `info` prints divergence in two
columns and both use the same `↑N ↓N` glyphs:

| Column   | Compares HEAD against                        | Moved by           |
| -------- | -------------------------------------------- | ------------------ |
| `BRANCH` | its upstream (`@{upstream}`)                 | `push` / `pull`    |
| `BASE`   | the **local** base branch (`master`, `main`) | `merge` / `rebase` |

`push` only ever acts on the `BRANCH` axis. A repository that is fully pushed —
`BRANCH` blank — can still show a large `BASE` count, and `push` reporting
`up-to-date` for it is accurate.

The table prints a legend under the summary line naming both comparisons:

```text
126 repositories  ·  3.06s  ·  12 need attention
BRANCH ↑↓ = vs upstream  ·  BASE ↑↓ = vs base branch
```

Confirm either axis by hand — left count is ahead, right is behind:

```bash
git -C <repo> rev-list --left-right --count HEAD...@{upstream}   # BRANCH axis
git -C <repo> rev-list --left-right --count HEAD...master        # BASE axis
```

### Why `BASE` is often large

The base is resolved to a **local** ref, and nothing moves it on its own:
`git fetch` updates `refs/remotes/*`, `git pull` updates the branch you are
standing on, and a base you never check out is neither. On a develop-based
workflow the local `master` stays wherever it was at clone time, so
`BASE ↑1275` means "develop holds 1275 commits your stale local master lacks",
not "1275 unpushed commits".

Repair it in bulk, without checking it out and without touching the branch you
are on:

```bash
gz-git update --sync-base -d 2 ~/projects
```

This fast-forwards each stale base ref and reports which ones moved. Preview
with `--dry-run` first.

### When the base holds commits the remote lacks

That is not a fast-forward, so the ref cannot simply catch up — and the two
situations it can mean are opposites:

- The base is parked on the tip of a task branch that **was** pushed. Every
  commit is reachable from some remote-tracking ref, so moving the pointer
  loses nothing. This is adopted: `base master +12 (adopted: 2 local commit(s) already pushed elsewhere; old tip at refs/gz-git/base-backup/master)`.
- The base carries commits that exist **nowhere else** — never pushed, on no
  branch the remote has. Moving the pointer would leave them reachable only
  from the reflog. This is refused: `base master blocked: 2 commit(s) exist only here`.

The count that decides is "commits on the local base reachable from no
remote-tracking ref", not "commits the remote base lacks". A blocked base is
reported and left exactly as it was; resolve it by pushing the commits
somewhere, then run again.

#### The backup ref, and why adopting needs one

Note the exact wording above: *reachable from a remote-tracking ref*, not *on
the remote*. Those are not the same claim. `refs/remotes/origin/*` is a local
cache — a tracking ref for a branch someone deleted upstream survives until the
next `fetch --prune`, and until then it stands as evidence for a commit that is
no longer anywhere. The decision can therefore be wrong in exactly one
direction, and it is the expensive one.

So an adopt parks the old tip before it moves the ref:

```bash
git -C <repo> for-each-ref refs/gz-git/base-backup/   # what was moved off
git -C <repo> branch recovered refs/gz-git/base-backup/master
```

They live under `refs/gz-git/` rather than `refs/heads/`, so they never show up
in `git branch` and can never be picked as a base on a later run. Delete one
with `git update-ref -d refs/gz-git/base-backup/<base>` once you are satisfied.

An adopt that only rewinds — the local base was strictly *ahead* of its remote
— says so rather than reporting a distance of zero:

```text
base master rewound to origin (adopted: 2 local commit(s) already pushed elsewhere; old tip at refs/gz-git/base-backup/master)
```

### A sync that failed is not a sync that was blocked

`blocked` is a verdict reached on evidence, and it comes with an instruction:
push those commits somewhere, then run again. When the sync could not reach a
verdict at all — git itself failed — the row says so instead:

```text
base sync failed: failed to read current branch: exit status 128
```

The two are separate statuses (`base-failed` and `base-blocked`) on purpose.
Nobody clears a failed row by pushing commits, and counting it among the
blocked ones makes the blocked count overstate how much work is outstanding.
Neither status changes the exit code: a base ref is a repair the command
offered, and failing at it must not report the pull you asked for as failed.

A repository with **no commits at all** — a fresh clone of an empty remote — is
neither. It has no base ref to sync and cannot have one until something is
committed, so it is skipped silently rather than listed as needing attention.
A repository that has commits but is sitting on an orphan branch (`git checkout --orphan`) is not in that class: its base ref is real and possibly stale, so it
is reported.

### The base is checked out somewhere

A base a linked worktree is standing on is reported and left alone, the same as
one checked out here: `base master is checked out in worktree celee__mbp__feat__x`.

This is not caution for its own sake. `git update-ref` is plumbing and enforces
no checkout rule — unlike `git branch -f`, it will move a branch out from under
a worktree, leaving that worktree's index disagreeing with its HEAD, so every
file the moved-off commits added reads as a staged deletion and the next commit
made there quietly reverts them. Finish or remove the worktree, then run again.

### Repositories with no local base branch at all

A clone that was switched to `develop` on day one may have no
`refs/heads/master`. The base then resolves to `develop` — the branch you are
standing on —
and the sync hands off to the normal pull path, so nothing is ever repaired and
`info`'s `BASE` column stays meaningless. Opt in to fixing it:

```bash
gz-git update --sync-base --create-missing-base -d 2 ~/projects
```

This creates the branch locally at the remote's tip. It requires `--sync-base`
and is off by default, because a repository deliberately kept without a local
trunk is a legitimate choice and creating a branch is not a repair.

Two things it deliberately will not do:

- **Retarget a base it could repair.** The create path only runs when the
  resolved base leaves nothing to fix — nothing resolved, or it resolved to the
  branch you are standing on. A repository with a stale local `master` gets that
  `master` repaired, not an unrelated `main` invented beside it. A base your
  config declares is never overridden either way.
- **Resurrect a branch deleted upstream.** Which branches exist is asked of the
  remote with `ls-remote`, not read out of `refs/remotes/`, so a tracking ref
  left behind by a deleted branch cannot become a local trunk. This is also why
  it finds `master` in a `clone --single-branch -b develop`, which has no
  `origin/master` ref to probe. It needs the network, so it does nothing under
  `--skip-fetch`.

### Doing it by hand

For a single repository:

```bash
git -C <repo> fetch origin master:master
```

The missing `+` is the safety net: that refspec refuses a non-fast-forward.

Resolution order: the `branch.defaultBranch` candidates from config, first one
that exists as a local ref; otherwise `main`, `master`, `develop`,
`development`. Sitting on the base branch prints nothing — HEAD cannot diverge
from itself.

## `push` failed but the message doesn't say why

Older builds reported only `push exited with code 128: exit status 128`,
discarding the stderr git had already produced. Current builds carry git's own
diagnosis on the same row:

```text
✗ tasuku-repo  origin: push exited with code 128: ERROR: Permission to iheanyi/tasuku.git denied to archmagece.
```

stderr is collapsed onto one line so the bulk table stays aligned — nothing is
truncated. On an older binary, reproduce the single repository by hand to see
the full text:

```bash
git -C <repo> push origin HEAD
```

## A repository pushes to a remote you don't own

A `permission denied` on push usually means `origin` points at an upstream you
only have read access to — common for a repository cloned directly rather than
from your own fork. Check where it actually points:

```bash
gz-git info -d3 | grep <repo>     # REMOTE column
git -C <repo> remote -v
```

Repoint it at your fork, or leave it read-only and expect `push` to keep
reporting it as failed.

## A bulk command printed nothing

Only `--quiet` fully silences a bulk command; `--format json` still prints. If
output is missing without `-q`, check the exit code before assuming the command
was skipped:

```bash
gz-git pull -d3; echo "EXIT=$?"
```

| Exit | Meaning                                   |
| ---- | ----------------------------------------- |
| 0    | all repositories processed successfully   |
| 1    | tool or configuration error (nothing ran) |
| 2    | one or more repositories failed           |

For `pull`, `fetch` and `update`, "failed" includes a repository left unsynced
by a conflict, an in-progress rebase or merge, or missing auth, not only a git
error (D8 in `docs/10-architecture/90-design-decisions.md`). Not-syncable
repositories (no remote, no upstream, skipped) and base-sync statuses do not.

`--dry-run` reports the same scan without touching anything, which is the
cheapest way to confirm the command is finding your repositories at all.

## `capability integrate-readiness-v1` fails

The readiness contract (`contract-v1`) was removed together with
`integrate bootstrap`, `integrate readiness review|update`, the readiness
runner, its preparation profiles, resource plans, the host measurement slot,
and its diagnostic files. The probe

```bash
gz-git capability integrate-readiness-v1
```

now exits 1 like any unknown capability, and a hook that required it must stop
requiring it rather than fall back. gz-git runs no repository code, so the
verification a readiness runner performed belongs to the task runtime (CE
`ce task gate` and the repository's own checks), which must pass before
gz-git is called. See
[`integrate check` passes without running tests or lint](#integrate-check-passes-without-running-tests-or-lint).

## Controller-backed `integrate queue` is unavailable

Hooks or wrappers that require explicit controller queue semantics must probe
the exact capability before invoking `integrate queue --controller-config`:

```bash
gz-git capability integrate-queue-controller-v1
```

Success writes exactly `integrate-queue-controller-v1` plus a newline. A
missing or older binary must fail closed rather than falling back to repository
configuration: controller mode binds the selected remote, declared integration
branch, and optional task-pattern filter explicitly.

## `integrate check` passes without running tests or lint

gz-git does not build, execute, or lint repository code itself. The gate
mode is always `none`, and the report shows

```text
PASS  readiness contract — none declared; gz-git runs no repository code, git checks decide readiness
INFO readiness provenance — mode=none source=... target=...
```

and READY means only that the git checks passed: freshness, merge-tree,
cross-merge, a clean working tree, a pushed source, and the post-gate
revalidation. Run the repository's own checks (for example `make check`, or
CE `ce task gate`) yourself before integrating.

The legacy make gate, `--allow-skipped-checks`, the readiness contract, and
their declarations were removed without a fallback. Every gz-git loader of a
`.gz-git.yaml`, workspace, controller, clone, or global config rejects these
keys wherever a `branch` or `integration` mapping holds them:

- `branch.makeTimeout`, `branch.makeOutcomeReport`
- `branch.prepareProfile`, `integration.prepareProfile`
- `branch.readiness` (any content, including an empty mapping)
- `branch.prepareHighLoad`, `branch.resourceHighLoad`
- `integration.readinessApproval`
- `branch.lockWait` (the `--lock-wait` flag is removed with it)

A config that still declares one fails to load:

```text
branch.readiness was removed: gz-git no longer builds, runs, or lints repository code; verification belongs to the task runtime (CE `ce task gate` and the repository's own checks), which must pass before gz-git is called; delete it from /repo/.gz-git.yaml
```

Delete the named key from that file. gz-git never waits for a lock: a command
that finds another gz-git process holding the same task or execution refuses
at once with `integration resource is busy`, names the holder, and exits 2
(see [integration resource scope](../../commands/integrate-resources.md)).

A manual `make lint` uses golangci-lint's shared cache, which is keyed by
absolute path and outlives a reclaimed worktree, so it can report issues under
a directory that no longer exists. Clear the cache and re-run:

```bash
golangci-lint cache clean && GOWORK=off make lint
```

The tracked `go.work` deliberately contains only `use .`, so a worktree or a
standalone clone does not depend on sibling modules existing at relative paths.
`GOWORK=off` is still useful here because it keeps this manual lint run aligned
with the canonical quality gate and isolates it from any parent workspace.

## A gate fix does not take effect until `make install`

`integrate check` and `integrate run` are executed by the `gz-git` on your
PATH, not by the source tree you are standing in. A fix to the gate itself is
therefore invisible to the gate until it is installed:

```bash
GOWORK=off make install
```

This is easy to miss precisely because the change looks landed — it is
committed, it is on the integration branch, and its tests pass. The judgment
still comes from the binary. When a gate behaves in a way the current source
cannot explain, check what is actually running before investigating the code:

```bash
gz-git --version
strings "$(command -v gz-git)" | grep -c "<a string your fix introduced>"
```

The same applies in reverse: after installing a gate change, an unexpected new
failure may be the fix working rather than a regression.

## See also

- [FAQ](faq.md) — what `gz-git` is and how it differs from `git`
- [Command reference](../../commands/README.md) — flags and workflows
- [Error handling design](../../10-architecture/50-data-flow-and-error-handling.md)
