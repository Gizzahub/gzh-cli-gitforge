<!-- size-limit: 1000 -->

<!-- A changelog is an append-only record, not a guide read front to back, so the
     default 500-line document budget models the wrong genre.

     1000 is a ceiling, not an exemption, and it is sized from measurement: the
     2026-08 backlog was 460 lines across 78 entries, so one release's worth of
     changes fits in the headroom above the current length. Hitting it therefore
     means a release is overdue, not that this file needs trimming — cut the
     release and move that line into docs/changelog/.

     Do not shrink the budget to a value a single batch can exhaust mid-write.
     Exceeding it is a hard error that blocks edits to this file, and the remedy
     is itself an edit to this file — that is how the 2026-08-07 to 2026-08-24
     gap happened, when 71 commits went unrecorded behind a 45KB limit. -->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Entries explain *why* a change was made — that is the part a generator cannot produce,
and the reason this file is written by hand. Keep each entry to roughly 3-5 lines and
group related commits into one entry; the exhaustive per-commit list is generated into
the GitHub Release notes by `.goreleaser.yaml` (`changelog.use: github`) and does not
belong here. When `[Unreleased]` outgrows the repository's document size gate, the fix
is to cut a release and move that line into `docs/changelog/`, not to write less.

## [Unreleased]

### Fixed

- Pinned govulncheck v1.1.4 bundles x/tools v0.29.0, whose SSA builder panics on
  the Go 1.27 linux standard library, so `security-deps` failed from the
  2026-10-13 toolchain bump onward; v1.9.0 scans it. `quality-check-validate`
  dry-runs graphs with `make -n`, which still executes lines naming `$(MAKE)`, so
  `test-install-atomic` now reaches `install-publish` through a variable instead.

- Host measurement lock wait notices end with `; host <name>` and the holder
  sidecar records `host`, from `GIT_WORK_HOST` or global Git `ce.workHost` with
  no hostname fallback; a malformed declaration shows `host invalid`. The lock
  path is identical on every device, so a pasted mst notice was attributed to
  another device's holder. Prepare patch 0.10.5.

- Resource-wait target observations now use the remaining acquisition budget
  instead of a separate five-second deadline. Healthy slow remote refreshes no
  longer cancel finite or explicitly unbounded admission early; observation
  errors returned by the observer and parent cancellation remain failures.

- Linker-stamped library versions now retain their explicit value even when it
  equals the source default. Unstamped `go install` builds still use the module
  version recorded in build information. Prepare patch 0.10.3.

- Bootstrap resume accepts GitLab-created numeric pipeline refs only when both
  expected branches and every pipeline ref match the initial commit. First-push
  emptiness checks and rejection of tags, notes, extra branches and changed SHAs
  remain strict. Prepare patch 0.9.4 for this published-behavior correction.

### Added

- Readiness V1 accepts target-owned `branch.readiness.timeout` for gates whose
  measured runtime exceeds 15 minutes. Positive durations up to two hours are
  contract-digest bound; omission preserves legacy digests and the 15-minute
  default. Resource waits remain separate. Prepare patch 0.10.4.

- Readiness V1 can opt into the closed `nd-stack-checked-tests-v1` preparation
  profile before its source-board gate. Both leased engine snapshots compile
  fixed test units into isolated caches; failures refuse the runner and preserve
  changed sibling evidence. Existing runner arguments and legacy digests remain unchanged.

- `gz-git forge bootstrap plan/apply` creates or reuses an empty GitLab project
  from a pinned template revision. Plans bind identity and configuration; retained
  receipts verify resume state, provenance and atomic master/develop refs. An initial
  template seed is supported without adding runtime dependencies to generated sites.

- Committed `branch.prepareHighLoad` puts confirmed heavy preparation profiles
  under FIFO host capacity independently of verification gates, allowing light
  integrations to proceed during a cold build. On cancellation, Go and Cargo
  preparation stop their process trees before returning capacity; expired target probes preserve
  the resource-wait diagnostic instead of fabricating target movement.

- `gz-git integrate check|run` take one host-wide OS file lock (`host-slots/integrate.lock` under
  `$XDG_STATE_HOME`, ce-devenv ADR-0001) before fetching and measuring, and `run` holds it through
  the push. Concurrent integrations on one host had driven the load past 190 and expired the make
  budget twice, and each landing voided the others' freshness. Waiting names the holder, is not
  charged to `makeTimeout`, and `--lock-wait` bounds it; an expired wait exits non-zero and measures nothing.

### Changed

- Integration receipts use schema v2 to bind the original push endpoint and
  cleanup protections. Schema v1 remains readable but cannot authorize cleanup
  or replay; this compatibility change requires minor version 0.10.0.
  Completion requires fresh Git absence proofs and preserves recreated resources.

- Prepare the 0.9.2 patch release with native task finish READY semantics and
  current CE reference validation. Provider rollout retains the expected source
  and target checks; release preparation does not publish or install a snapshot.

- Stale integration candidates return NOT READY before readiness, preparation,
  or Make gates. Previously an already-stale TASK-285 candidate spent 25 minutes
  in validation. Fresh candidates retain required gates and the separate wait /
  validation timing contract; unexecuted gates have zero measured duration.

- High-load resource waiters refresh their captured target ref and stop for a new check
  when it moves, so stale work does not spend the full wait budget before learning that
  verification must restart. Transient fetch failures warn once and preserve the wait.

- Nested high-load gate children reuse their parent’s host slot through `HOST_SLOTS_HELD`,
  preserving FIFO capacity and holder records while Git target locks remain independent.

- High-load slot acquisition now defaults to a 60 minute wait, so a stuck resource holder
  cannot stall a new integration forever. Explicit `--lock-wait` overrides repository-root
  `branch.lockWait`; zero remains unbounded. FIFO and OS waiting share the original limit,
  and expiry reports its source separately from gate execution.

- Declared high-load integration gates now enter FIFO resource admission before the host OS
  lock. Cancellation removes only the caller's ticket, positively dead waiters can be recovered,
  and queue plus OS-lock waiting share one budget. Older callers still use the same capacity lock.

- Integration now locks each local repository and destination branch separately. Only gates
  explicitly classified by committed `branch.resourceHighLoad: true` acquire the host resource
  slot, releasing it after each gate so unrelated light integrations can finish during heavy
  validation. A successful push remains recorded when local fast-forward or reclaim fails.

- `gz-git integrate check` measures a prepared target baseline only for a make target the source
  failed. A passing source never needed the baseline for its verdict, yet a prepare profile
  prepared and ran both trees every time, which doubled the slowest step of every green check.
  The two worktrees are no longer alive together, and the report now says per make target whether
  the baseline was measured or skipped. A target that declares a make outcome report is still measured.

______________________________________________________________________

## Source-version milestones

Earlier 0.x files record source-version milestones. v0.8.0 (2026-10-01) was the first
stable published tag, and 0.9.0 (2026-10-02) and 0.9.1 (2026-10-08) continue the stable line. This file
carries only unreleased changes.

| Line                           | Milestones                             |
| ------------------------------ | -------------------------------------- |
| [0.9.x](docs/changelog/0.9.md) | 0.9.1 (2026-10-08), 0.9.0 (2026-10-02) |
| [0.8.x](docs/changelog/0.8.md) | 0.8.0 (2026-10-01)                     |
| [0.7.x](docs/changelog/0.7.md) | 0.7.0 (2026-07-02)                     |
| [0.6.x](docs/changelog/0.6.md) | 0.6.1 (2026-01-25), 0.6.0 (2026-01-21) |
| [0.4.x](docs/changelog/0.4.md) | 0.4.0 (2026-01-02)                     |
| [0.3.x](docs/changelog/0.3.md) | 0.3.1 (2025-12-02), 0.3.0 (2025-12-01) |
| [0.2.x](docs/changelog/0.2.md) | 0.2.0 (2025-12-01)                     |
| [0.1.x](docs/changelog/0.1.md) | 0.1.0-alpha (2025-12-01)               |

The mechanical per-commit list is not copied here; `.goreleaser.yaml`
(`changelog.use: github`) generates it into the GitHub Release notes.

## Links

- **Repository**: https://github.com/gizzahub/gzh-cli-gitforge
- **Documentation**: https://pkg.go.dev/github.com/gizzahub/gzh-cli-gitforge
- **Issues**: https://github.com/gizzahub/gzh-cli-gitforge/issues
- **Discussions**: https://github.com/gizzahub/gzh-cli-gitforge/discussions

______________________________________________________________________

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Follows [Conventional Commits](https://www.conventionalcommits.org/) specification
- Inspired by [gzh-cli](https://github.com/gizzahub/gzh-cli)

______________________________________________________________________
