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
