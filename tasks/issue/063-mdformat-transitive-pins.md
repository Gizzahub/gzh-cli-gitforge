---
id: ISSUE-63
title: "mdformat transitive dependencies are not pinned"
type: bug
priority: P3
severity: low
discovered-in: "release-gate-ci review of the bin/tools mdformat pin (d0c1197b)"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

`install-mdformat` pins mdformat 1.0.0, mdformat-gfm 1.0.0 and
mdformat-frontmatter 2.1.2, but `uv tool install` resolves their
dependencies (markdown-it-py, mdit-py-plugins, ruamel.yaml, and so on)
to whatever is newest at install time. Two installs on different days can
format the same Markdown differently while the `--version` check still
passes, so `format-check` can flip on an unchanged tree. This predates the
pin (the old global install had the same gap); it is not a regression.

## Reproduction

1. Install with `make install-mdformat` and record
   `uv pip list --python bin/tools/python/mdformat/bin/python`; on
   2026-10-19 it held markdown-it-py 4.2.0, mdit-py-plugins 0.6.1,
   ruamel-yaml 0.19.1, mdurl 0.1.2 and wcwidth 0.9.2, all unpinned.
1. A dependency publishes a release that changes rendering.
1. A fresh install picks it up; the version check still passes.

## Expected vs Actual

- Expected: the whole tool environment is reproducible, for example by
  `uv tool install --exclude-newer <date>` or a constraints file, and the
  version check covers what decides formatting.
- Actual: only the three top-level packages are fixed.
