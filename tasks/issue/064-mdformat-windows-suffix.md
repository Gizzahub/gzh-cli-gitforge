---
id: ISSUE-64
title: "Pinned mdformat path lacks the Windows executable suffix"
type: bug
priority: P3
severity: low
discovered-in: "release-gate-ci review of the bin/tools mdformat pin (d0c1197b)"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

`.make/tools.mk` sets `MDFORMAT := $(CURDIR)/bin/tools/mdformat`. On
Windows, uv writes the entry point as `mdformat.exe`, so the version check
never passes and `format-check` would reinstall and then fail. The Go tools
in the same file (gofumpt, goimports, golangci-lint) already append
`$(shell go env GOEXE)`; mdformat is the exception. CI runs the quality
gate on `ubuntu-latest` only, so no current job is affected.

## Reproduction

1. Run `make install-mdformat` on a Windows host with uv.
1. The re-check after install reports a missing or wrong version.

## Expected vs Actual

- Expected: the mdformat path carries the platform suffix like the Go
  tools (uv's entry point is `.exe` on Windows), or the gate declares
  Windows unsupported.
- Actual: the suffix is absent and the limitation is undeclared.
