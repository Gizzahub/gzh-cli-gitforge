---
id: ISSUE-57
title: "Integrate output carries no run provenance line"
type: bug
priority: P2
severity: medium
discovered-in: "gzh-cli-devbox ISSUE-350 follow-up: slot wait device attribution"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

Callers capture integrate output into their own log files, and nothing in
that output says which device or gz-git build wrote it. An mst log pasted
on mbp was attributed to mbp's lock holder. 0.10.5 labels only the
host-slot wait notice (`; host <name>`), so expiry errors, FIFO and target
notices, and every future message still cannot be attributed, and a log
from an older build looks the same as one with an undeclared device.

## Reproduction

1. Run a high-load integrate gate and let `--lock-wait` expire.
1. The expiry error (`pkg/integrate/hostlock.go:120`, `:149`, `:221`;
   `pkg/integrate/admission.go:67`, `:77`, `:112`) names no device, and
   the log has no line naming the gz-git version.

## Expected vs Actual

- Expected: each integrate run prints one provenance line first, for
  example `integrate: run device=mst gz-git=0.10.6`, so a reader or
  `ce-devenv scripts/slot_wait_audit.py` attributes the whole file; a log
  without it is identifiable as an older build.
- Actual: only wait notices carry a device label.

## Scope

- Print the provenance line once per integrate run with the device from
  the existing chain (`GIT_WORK_HOST`, then global `ce.workHost`, no
  hostname fallback), shown as `undeclared` or `invalid` when absent.
- When the device is undeclared, add one line saying how to declare it.
- Keep the per-notice label for pasted fragments.
- The audit consumer change is tracked in ce-devenv.

## Resolution

Resolved on 2026-10-19.

- `integrate check`, `integrate run`, `integrate cleanup` and
  `integrate readiness update review|auto|run` print
  `integrate: run device=<host> gz-git=<version>` as their first stderr
  line. These are the subcommands that run gates or wait on host or
  target locks, so their notices share the stream. `queue`, `receipt`,
  `reconcile`, `bootstrap`, `provision` and `legacy-cleanup` print no
  notices and are unchanged. Stdout is untouched: `--json` stays one
  protocol document. The line ignores `--quiet`, like the lock notices
  it attributes.
- The device comes from `pkg/integrate` `resolveDeclaredDevice`
  (`GIT_WORK_HOST`, then global `ce.workHost`, no hostname fallback).
  The host-lock `; host <name>` label now uses the same function, so
  within `pkg/integrate` the two cannot disagree. Other resolvers can:
  `internal/runtask` lowercases `GIT_WORK_HOST` (see ISSUE-58). An
  absent name shows `undeclared`, followed by one line naming
  `GIT_WORK_HOST=<name>` and `git config --global ce.workHost <name>`.
  A malformed name shows `invalid`, followed by one line saying the
  declared name is invalid and must be fixed to the grammar.
- The readiness `review`, `auto` and `run` commands print the line as
  the first statement of their command, before the policy load, TTY,
  `--expect-source` and `--statement` refusals.
- The version is the `appVersion` that `cmd.Execute` receives from
  `main.version` (ldflags, or the module version for `go install`),
  passed to `integrate.WriteRunProvenance`; an unstamped build prints
  `unknown`.
- Verified by `TestFormatRunProvenance`, `TestDeviceIdentityLabel`,
  `TestIntegrateCheckPrintsProvenanceFirst` (declared by env and by
  global config, undeclared, invalid),
  `TestIntegrateCleanupPrintsProvenance`,
  `TestIntegrateReadinessPrintsProvenanceBeforeEarlyError` and
  `TestIntegrateRunJSONKeepsProvenanceOffStdout`, with an isolated
  `HOME` and `GIT_CONFIG_GLOBAL`.
