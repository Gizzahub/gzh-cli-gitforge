---
id: ISSUE-65
title: "CHANGELOG header says hand-written, release practice disagrees"
type: bug
priority: P3
severity: low
discovered-in: "v0.10.5 release decision"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

The `CHANGELOG.md` header states the file "is written by hand" and that
each change gets an entry explaining why. The owner stated on 2026-10-19
that the changelog is not hand-written, and v0.10.5 was tagged with
cf5c6acb (readiness `resourcePlan`) absent from `[Unreleased]`, relying on
the GitHub Release notes that `.goreleaser.yaml` (`changelog.use: github`)
generates. Agents follow the header, so they block or delay releases on an
entry the owner does not require.

## Reproduction

1. Land a feature commit without a CHANGELOG entry.
1. Read the CHANGELOG header before tagging: it requires the entry.

## Expected vs Actual

- Expected: the header states the actual contract (generated notes only,
  or hand-written entries required), and the release flow checks it.
- Actual: the header and the release practice disagree.

## Scope

- The owner chooses the contract; this card does not decide it.

## Resolution

Resolved on 2026-10-19. The owner chose generated notes as the source.

- `CHANGELOG.md` states that GoReleaser publishes release notes into the
  GitHub Releases. Agents must not add `[Unreleased]` entries and must not
  hold a release for one. The file now only points to the history.
- The former `[Unreleased]` entries moved verbatim to `docs/changelog/0.10.md`.
  That file is the last hand-written record, covering v0.9.2 to v0.10.5.
- `CONTRIBUTING.md` drops the "Update CHANGELOG.md" release step. The
  `.make/dev.mk` comment states the new contract. The stale `changelog` help
  line and the git-chglog hint are removed; no `changelog` target exists.
- The release flow gets no new check. GoReleaser generates the notes on every
  tag, so there is no hand-written entry left to verify.
- `README.md` uses a release badge that tracks the latest GitHub Release, and
  `docs/README.md` links the Releases page as the source of release notes.
- The `docs/changelog/0.x.md` intros still send readers to `CHANGELOG.md` for
  unreleased changes. They stay as written: `CHANGELOG.md` now opens with the
  contract and forbids `[Unreleased]` entries, so the link leads to the right
  rule.
- The specs that still describe a CHANGELOG release step, and the release
  archive file list, are tracked in ISSUE-67.
