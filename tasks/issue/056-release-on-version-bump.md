---
id: ISSUE-56
title: "Version bumps are not released, so device binaries drift"
type: bug
priority: P1
severity: medium
discovered-in: "gzh-cli-devbox ISSUE-350 follow-up: mst and mbp gz-git rollout"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

`VERSION` reached 0.10.5 while the last published tag is `v0.9.4`
(2026-10-14), so 0.10.0 through 0.10.5 were never released and the
Homebrew cask stayed at 0.9.4. Devices fill the gap with `make install`,
which defaults to GOBIN (`~/go/bin`) and shadows the cask outside the
DevBox. Each device then runs a different hand-installed build, fixes
(for example the 0.10.5 host label) do not reach agents working outside
the DevBox, and `install-path-audit` fails on every install.

## Reproduction

1. `git tag --sort=-v:refname | head -1` prints `v0.9.4`; `cat VERSION`
   prints `0.10.5`.
1. Observed 2026-10-19 outside the DevBox: mbp runs `~/go/bin/gz-git`
   0.10.5 with no cask installed; mst runs `~/go/bin/gz-git` 0.10.4,
   which shadows the cask's 0.9.4. Inside the DevBox both run
   `tmp/bin/gz-git` 0.10.5.

## Expected vs Actual

- Expected: a version bump that lands on master is published, every
  device upgrades with `brew upgrade gz-git`, and source builds live
  only in the DevBox `tmp/bin` (gzh-cli-devbox `mise.toml`).
- Actual: releases need a manual tag, the global channel is whatever
  each device last built, and `~/go/bin` copies shadow the cask.

## Scope

- Publish automatically when a `VERSION` bump lands on master, through
  the existing `v*.*.*` tag trigger in `.github/workflows/release.yml`,
  and only from a commit whose quality gate passed.
- `make install` without `BINDIR` must not silently shadow a
  Homebrew-managed gz-git; name the cask as the global channel and
  `BINDIR` for development builds.
- `install-path-audit` labels its findings `WARN`, not `FAIL`, when
  `INSTALL_AUDIT_WARN_ONLY=1` makes them non-fatal.
- After the first automated release, each device reclaims
  `~/go/bin/gz-git` (operator action, recorded in the resolution).

## Resolution

Resolved on 2026-10-19 in code; the operator step below is still open.

- `.github/workflows/release.yml` also runs when the `CI` workflow
  completes successfully for a push to master. The job reads `VERSION`
  at that commit. If `v$VERSION` is missing and sorts above every
  existing `v*.*.*` tag (`sort -V`), it tags the commit and runs
  GoReleaser in the same job. A tag pushed with `GITHUB_TOKEN` does not
  start another workflow, so a second tag-triggered run cannot happen.
  An existing or older tag skips the release; a malformed `VERSION`
  fails the job. Manual `v*.*.*` tag pushes publish as before. A
  `release` concurrency group serialises overlapping runs. The decision
  step was simulated locally on six cases (newer, existing, older,
  0.10.10 over 0.10.9, first tag, malformed). `workflow_run` uses the
  definition on master, so it takes effect only after this commit
  lands.
- `make install` now runs `scripts/install-channel-guard.sh` before
  building. Without `BINDIR`, it refuses when Homebrew manages gz-git
  (Caskroom or Cellar under `brew --prefix`) and the GOBIN or
  GOPATH/bin target is on `PATH`. The message names
  `brew upgrade gz-git` for global use and `BINDIR=` for development
  builds. An
  explicit `BINDIR`, a target off `PATH` (the `test-install` temp
  GOBIN), and hosts without brew are not affected. On mbp the guard
  refused `~/go/bin` over `/opt/homebrew/Caskroom/gz-git`.
- `install-path-audit.sh` labels findings `WARN` and ends with
  `install-path-audit: WARN` when `INSTALL_AUDIT_WARN_ONLY=1`.
- `scripts/test-install-path-audit.sh` covers the WARN labels and six
  guard cases.

Operator step, after the first automated release: each device runs
`brew upgrade gz-git` and removes its `~/go/bin/gz-git` (mst still has
0.10.4 there).
