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

## Resolution

Resolved on 2026-10-19 with a constraints file rather than
`--exclude-newer`. A date bound still lets a dependency resolve differently
when the top-level pins change. An explicit list puts the resolved versions in
review.

- `.make/mdformat-constraints.txt` pins the five transitive packages at the
  versions recorded above. `install-mdformat` passes it to
  `uv tool install --constraints`.
- `MDFORMAT_DEPS_OK` reads each pin with `importlib.metadata` from the tool's
  own interpreter. The target reinstalls when either check fails and refuses
  when the result still differs, naming the offending lines. An empty list, a
  missing package, a different version and a line that is not
  `name==version` all fail the check.
- Checked on macOS with uv 0.12.0. A fresh install passes and a rerun does not
  reinstall. A changed pin, a loose `>=` line, a missing package and an empty
  list each exit 1; a comment that contains `==` is ignored.
  An unsatisfiable pin prints uv's resolution error, the target exits 1, and
  the previous install stays usable. The install command also passes under
  uv 0.9.17, the version CI pins.
- The check proves the listed pins, not that the list is complete. A
  dependency a future top-level bump adds stays unpinned until the list is
  re-resolved; `.make/tools.mk` says so next to the versions. On Python 3.10
  uv also installs tomli, which only reads `.mdformat.toml` (none here).
- The Python interpreter version is still chosen by uv. It is not pinned here.
