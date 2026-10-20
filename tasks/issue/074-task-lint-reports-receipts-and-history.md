---
id: ISSUE-74
title: "ce task lint fails on tasks/receipts/ and tasks/HISTORY.md"
type: bug
priority: P3
severity: low
discovered-in: "gzh-cli-gitforge workspace defect registration"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`ce task lint` exits non-zero on master (448be9b0), so the board cannot report
CLEAN. Two paths cause it, and neither is a task card:

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

Run from the primary checkout on master 448be9b0.

## Expected vs Actual

- Expected: `ce task lint` reports CLEAN, so a lint failure points at a real
  board problem.
- Actual: lint exits with 1 stray and 2 structural issues on every run.
  Operators learn to ignore it.

## Follow-up scope

Decide where each file belongs and make `ce task lint` report CLEAN:

- Receipt: no code, Makefile or script in this repository reads
  `tasks/receipts/`; only the CE tooling that wrote it might. Confirm where
  that tool expects done-review receipts. Then move the receipt there, or, if
  a reader relies on the current path, declare `receipts` through
  `ce-tasks.yaml` (`card-dialect: { zones: [...] }`).
- History: move it out of `tasks/`, for example under `docs/`, and update the
  links from `tasks/README.md`. Alternatively, record why the board must
  tolerate it.

Registration only; no remediation is included here.
