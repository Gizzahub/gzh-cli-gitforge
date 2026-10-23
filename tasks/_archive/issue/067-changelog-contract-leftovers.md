---
id: ISSUE-67
title: "Specs and release archives still assume a hand-written CHANGELOG"
type: bug
priority: P3
severity: low
discovered-in: "independent review of the ISSUE-65 generated-notes change"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

ISSUE-65 made the GoReleaser-generated GitHub Release notes the source of
truth. `CHANGELOG.md` now only points to them and to `docs/changelog/`. Three
places still assume the old contract:

- `docs/specs/70-gzh-cli-integration.md:610` is a pending spec. Its v1.0.0
  checklist still includes "CHANGELOG.md updated".
- `docs/specs/60-library-publication.md:61` is a completed spec. Its release
  workflow still lists "Changelog Update" as a step, and line 158 still says
  to add deprecations to `CHANGELOG.md` under "Deprecated".
- `.goreleaser.yaml:79-81` ships `CHANGELOG*` and only
  `docs/changelog/0.8.md` in each release archive. The shipped `CHANGELOG.md`
  is a pointer whose relative `docs/changelog/` links are dead inside the
  tarball. The Releases link still works.

## Reproduction

1. Follow the v1.0.0 checklist in `docs/specs/70-gzh-cli-integration.md`.
1. It asks for a CHANGELOG update that `CHANGELOG.md` now forbids.

## Expected vs Actual

- Expected: the specs describe generated notes, deprecations are announced
  where the new contract puts them, and the archive file list matches what
  the pointer file needs.
- Actual: the specs and the archive list predate the ISSUE-65 contract.

## Scope

- Deciding where deprecation notices go now, such as commit bodies, release
  notes or a docs page, is part of this card.

## Resolution

Resolved on 2026-10-20.

- Deprecations are announced with a `Deprecated:` trailer in the commit
  message, next to the other trailers. GoReleaser's changelog groups read
  commit subjects only, so `scripts/release-deprecations.sh <tag>`
  collects the trailers from the previous `v*.*.*` release tag (or the
  whole history for a first release) up to `<tag>` into a
  `## Deprecated` list. A pre-release tag such as `v0.2.0-rc.1` does not
  start the range, as in the workflow's newest-release check.
  A `Deprecated:` line in the middle of a body is not a trailer and is
  not collected. Indented continuation lines are joined (`unfold`). An
  unindented continuation line usually makes Git reject the whole
  trailer block, so the script warns (`::warning::`) when the range has
  more body lines starting with the key than collected notices; like
  Git, the count ignores the key's case and spaces before the colon.
  Known limit: next to a Git-generated trailer such as `Signed-off-by:`,
  Git keeps the block and the notice is cut at its first line without a
  warning. `CHANGELOG.md` states the rule, the one-line or indented
  form, and that limit.
- `.github/workflows/release.yml` gains a "Collect deprecation notices"
  step on both publish paths (tag push and VERSION bump). It runs the
  script for the release tag and passes `--release-footer=<file>` to
  GoReleaser only when the list is not empty. A script failure stops the
  job before GoReleaser publishes. On the VERSION path the tag is
  already pushed by then; re-running the job publishes it, as the
  existing re-run recovery in that step allows.
- Merge commits stay in the range: a merge message does not copy the
  trailers of the commits it merges, so they do not duplicate notices,
  and a notice written in a merge message is still collected.
- `docs/specs/60-library-publication.md`: the release workflow drops the
  "Changelog Update" step, the deprecation step names the trailer, and
  sections 3.1 and 3.2 carry a "Superseded" note, keeping the original
  plan as history. `docs/specs/70-gzh-cli-integration.md`: the gitforge
  v1.0.0 checklist asks for commit bodies and trailers instead of a
  CHANGELOG update. The gzh-cli v1.0.0 checklist in the same spec is
  left alone, because gzh-cli owns its own changelog contract.
- `.goreleaser.yaml` ships `docs/changelog/*.md`, so every link in the
  shipped `CHANGELOG.md` resolves inside the archive.
- Verified by `make release-notes-test` (`scripts/test-release-deprecations.sh`,
  now part of `quality-check`): a release range with three trailers in
  two commits, a prose `Deprecated:` that must not be collected, a tag
  with no trailers (empty output), a first release, and a missing tag
  (exit 1). Review follow-up: a `snapshot` tag inside the range must not
  start it, a folded trailer is one notice, and an unindented wrap is
  not collected and warns. A second review added an rc tag that must not
  start the final release's range, and a collected lowercase
  `deprecated:` key that must not cancel a dropped notice's warning.
  Mutations, each reverted: dropping `unfold`, `--match` or
  `--exclude`, counting only the exact `Deprecated:` form, and always or
  never warning each fail a case. The workflow passes actionlint
  1.7.12, GoReleaser v2.18.2 `check` passes, and its `release --help`
  lists `--release-footer`.
