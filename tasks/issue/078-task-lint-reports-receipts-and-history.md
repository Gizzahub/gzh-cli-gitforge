---
id: ISSUE-78
title: "ce task lint fails on tasks/receipts/ and tasks/HISTORY.md"
type: bug
priority: P3
severity: low
discovered-in: "gzh-cli-gitforge workspace defect registration"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`ce task lint` exits non-zero on origin/master (a7d72268), so the board cannot
report CLEAN. Two paths cause it, and neither is a task card:

- `tasks/receipts/TASK-012/done-review.json`, added in 71f1d441
  (2026-10-07), is a done-review receipt (`reviewed-card-sha256`,
  `tool_version`, `tool_revision`). Lint reports `receipts` and
  `receipts/TASK-012` as non-canonical task directories.
- `tasks/HISTORY.md`, split out of the README in a5be25ca (2026-08-07), is the
  completion history. Lint reports it as a stray root card.

## Reproduction

```
$ ce task lint
STRUCTURE
  receipts                         "receipts" is not a canonical task directory; ...
  receipts/TASK-012                "receipts/TASK-012" is not a canonical task directory; ...

STRAYS
  root-card    tasks/HISTORY.md → ?
...
Error: 1 stray card(s); 2 structural issue(s); 0 stage diagnostic(s); 0 id parity issue(s); 0 duplicate id(s); 0 hidden in progress
```

Reproduced on 448be9b0 and again on a7d72268.

## Expected vs Actual

- Expected: `ce task lint` reports CLEAN, so a lint failure points at a real
  board problem.
- Actual: lint exits with 1 stray and 2 structural issues on every run.
  Operators learn to ignore it.

## Follow-up scope

Make `ce task lint` report CLEAN:

- Receipt: keep it where it is and declare `receipts` as an auxiliary
  directory in `ce-tasks.yaml` (`card-dialect: { aux-dirs: [receipts] }`).
  - The path is already the one CE uses. `ce task cleanup` owns
    `tasks/receipts/<ID>/`, and CE's task hygiene guide names tracked
    `receipts/<TASK-ID>/*.json` as the `aux-dirs` case. ce-workbook's
    `ce-tasks.yaml` declares the same.
  - Do not follow the lint hint's `zones: [receipts]`. On a scratch copy it
    adds a third issue (`declared zone "receipts" is a legacy parking zone`).
    `aux-dirs: [receipts]` clears both structural issues.
- History: move it out of `tasks/`, for example under `docs/`, and fix every
  reference. Alternatively, record why the board must tolerate it. Moving it
  breaks:
  - `tasks/README.md:10` and `:20`;
  - `docs/changelog/0.8.md:558`;
  - `tasks/issue/19-tasks-readme-exceeds-doc-size-guideline.md:40`, `:52` and
    `:64`;
  - the relative `issue/...` and `README.md` links inside HISTORY.md itself.

Registration only; no remediation is included here.

## Resolution

Resolved on 2026-10-20. `ce task lint` now reports
`TASK LINT — CLEAN (3 dir(s), 84 card(s), ...)`.

- Receipt: a root `ce-tasks.yaml` declares
  `card-dialect: { aux-dirs: [receipts] }`. Lint lists `receipts/` as
  `auxiliary` and both structural issues are gone. The receipt stays at
  `tasks/receipts/TASK-012/`.
- History: `tasks/HISTORY.md` moved to `docs/changelog/tasks-history.md`
  with `git mv`. Its 23 relative links now point at `../../tasks/...`, and
  each resolves. `tasks/README.md:10` and `:20` and `docs/changelog/0.8.md:558`
  name the new path.
- Card 19's mentions of `tasks/HISTORY.md` (`:40`, `:52`, `:64`) are left
  as written. They record that card's own past state, not a live link.
- `ce task validate --all` reports 55 failing cards here against 58 on the
  primary checkout, so this change adds none.
