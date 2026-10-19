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
