---
id: ISSUE-58
title: "Host fields accept names outside the device catalog grammar"
type: bug
priority: P3
severity: low
discovered-in: "gzh-cli-devbox ISSUE-350 follow-up: host label review"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

Host fields are checked with the generic `validateOpaqueToken`
(`pkg/integrate/receipt.go:102`), used for the wait-notice label
(`pkg/integrate/hostlock.go:197`) and owner hosts
(`pkg/integrate/legacy_cleanup_plan.go:485`, `receipt.go` owner checks).
It accepts uppercase letters, `;` and other punctuation. The device
catalog that owns `ce.workHost` names (ce-devenv
`config/agents/work-identities.yaml`, grammar
`^[a-z0-9]+(?:-[a-z0-9]+)*$` in `scripts/device_catalog.py:11`) is
stricter, and each consumer (gz-git, the ce-devenv slot audit) keeps its
own rule, so they drift: a `;` in a label would split a field if the
notice ever grew another trailing field, and the audit already differs
from gz-git on C1 controls and byte length.

## Reproduction

1. `GIT_WORK_HOST='m;x'` with a contended high-load gate prints
   `; host m;x` instead of `; host invalid`.

## Expected vs Actual

- Expected: host fields accept exactly the catalog grammar and render a
  malformed value as `host invalid` with a hint.
- Actual: any opaque token is accepted.

## Scope

- A dedicated host validator applying the catalog grammar to host fields
  only; execution-id, generation and actor validation stay unchanged.
- Existing declared hosts (`mbp`, `mst`) already conform.
- Reading the catalog itself stays out of gz-git; the grammar is the
  shared contract.

## Resolution

Resolved on 2026-10-19.

- `pkg/integrate` applies exactly `^[a-z0-9]+(?:-[a-z0-9]+)*$` with
  the same 128-character cap as the opaque tokens. It replaces
  `validateOpaqueToken` for the run owner host
  (`ExecutionContext.Validate`, which the `--owner-host` flag and stored
  pre-mutation outcomes go through), the legacy cleanup plan owner host,
  the cleanup caller host (`ResolveCallerOwner`) and the host-lock
  device label. Execution id, generation and actor stay opaque tokens.
  The execution-context fields wrap `ErrInvalidExecutionContext`.
- `ResolveCallerOwner` now tells a malformed host apart from a missing
  one, with its own message rather than the execution-context sentinel:
  `device identity is invalid: host "<value>" is not a device name ...; set GIT_WORK_HOST or global Git ce.workHost`.
- `GIT_WORK_HOST='m;x'` renders `host invalid`. The run provenance line
  from ISSUE-57 adds one hint per run saying the declared name is
  invalid and how to fix it.
- Receipts are not re-validated on read, so existing receipts stay
  readable. A pre-mutation outcome or legacy cleanup plan written with a
  non-conforming host is refused on read; `mbp` and `mst` conform, and
  no such host appears in the test fixtures.
- A receipt whose owner host breaks the grammar stays readable through
  `integrate receipt` but can no longer be cleaned up. A caller that
  declares the same host is refused by `ResolveCallerOwner`; a caller
  with a valid host fails the owner comparison in `cleanupIdentityCAS`
  (`ErrCleanupOwnerMismatch`). Only a manual run such as
  `--owner-host MBP` could have written one. On mbp, every file under
  the gitforge `gz-integrate/v1` store that records an owner host (12
  of 34, 2026-10-19) records `mbp`.
- `internal/runtask` normalizes `GIT_WORK_HOST` (`MBP` becomes `mbp`,
  like CE `NormalizeIdentity`), while integrate validates without
  normalizing and shows `device=invalid`. "The provenance line and the
  host-lock label cannot disagree" holds within `pkg/integrate` only;
  a task run and an integrate run can still label one device
  differently.
- Open release dependency: ce-agent-kit
  `internal/usecase/taskruntime/finish_reason.go` takes the first
  non-empty stderr line as the failure reason when no `FAIL ` line is
  present. With the provenance line now first, CE would record it as
  the reason. The CE fix (skip `integrate: run device=` and the hint
  lines, or prefer the last stderr line) must ship before any gz-git
  release that contains this change. It is not made here; the hold is
  tracked as ISSUE-69.
- Verified by `TestValidateHostNameCatalogGrammar` (accepts `mbp`,
  `mst`, `a1-b2`, 128 characters; rejects uppercase, `;`, leading,
  trailing and double hyphens, empty, non-ASCII, spaces and 129
  characters),
  `TestHostFieldsUseCatalogGrammar`, `TestResolveCallerOwnerHostGrammar`,
  `TestResolveDeclaredDevice` and
  `TestIntegrateRunOwnerHostFlagUsesDeviceGrammar` (`--owner-host MBP`,
  `m;x` and `a--b` exit 2 naming `owner-host`).
