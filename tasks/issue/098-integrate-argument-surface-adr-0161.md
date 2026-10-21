---
id: ISSUE-098
title: "gz-git integrate still exposes the argument surface ADR-0161 retires"
type: bug
priority: P2
severity: medium
discovered-in: "gz-git 0.12.0 rollout (ce-agent-kit ADR-0163 follow-up)"
discovered-at: 2026-10-22
created: 2026-10-22
---

## Summary

ce-agent-kit `decisions/adr/0161-integration-judgment-owned-by-task-runtime.md`
decision 3 reduces `gz-git integrate` to execution and recording:
`integrate run <branch> --source <sha> --target <ref>@<sha> [--record <id>]
[--owner <actor>/<host>/<kind>] [--json]`, plus `integrate cleanup <id>`,
`integrate receipt <id>` and `integrate queue` (read-only). ce-agent-kit
`decisions/adr/0163-run-finish-cutover-uses-existing-provider-arguments.md`
decision 3 keeps that surface as the goal but defers it: run-finish cut over
with the arguments 0.11.1 already accepted, and the new surface arrives as a
separate gz-git change, after which CE changes its call. gz-git 0.12.0 shipped
the judgment removal without that surface, so nothing tracks it yet.

## Reproduction

1. Read the installed command reference for gz-git 0.12.0:
   `share/gz-git/markdown/gz-git-integrate-run.md` and
   `share/gz-git/markdown/gz-git-integrate.md` in the active devenv release.

## Expected vs Actual

- Expected: `integrate run` takes `--source`, `--target <ref>@<sha>`,
  `--record` and `--owner`, and the subcommand set is the four that ADR-0161
  decision 3 names.
- Actual: `integrate run` still takes the five owner arguments
  (`--execution-id`, `--generation`, `--owner-actor`, `--owner-host`,
  `--owner-kind`) and `--expect-source`/`--expect-target`, and the reference
  still lists `check`, `provision`, `reconcile` and `legacy-cleanup`.

## Scope

- Each subcommand outside the four needs an owner decision before removal.
  CE run-start calls `integrate provision siblings acquire` for sibling
  provision, so that path must have a replacement owner or stay.
- CE run-finish changes its provider call in the same cycle as the gz-git
  release, as ADR-0163 decision 3 requires. That CE change is ce-agent-kit
  work, not this repository's.
