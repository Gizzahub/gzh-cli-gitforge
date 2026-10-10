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

### Added

- `gz-git integrate check|run` take one host-wide OS file lock (`host-slots/integrate.lock` under
  `$XDG_STATE_HOME`, ce-devenv ADR-0001) before fetching and measuring, and `run` holds it through
  the push. Concurrent integrations on one host had driven the load past 190 and expired the make
  budget twice, and each landing voided the others' freshness. Waiting names the holder, is not
  charged to `makeTimeout`, and `--lock-wait` bounds it; an expired wait exits non-zero and measures nothing.

### Changed

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
