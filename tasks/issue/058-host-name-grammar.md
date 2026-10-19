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

- `pkg/integrate` `validateHostName` applies exactly
  `^[a-z0-9]+(?:-[a-z0-9]+)*$` and wraps `ErrInvalidExecutionContext`.
  It replaces `validateOpaqueToken` for the run owner host
  (`ExecutionContext.Validate`, which the `--owner-host` flag and stored
  pre-mutation outcomes go through), the legacy cleanup plan owner host,
  the cleanup caller host (`ResolveCallerOwner`) and the host-lock
  device label. Execution id, generation and actor stay opaque tokens.
- `ResolveCallerOwner` now tells a malformed host
  (`device identity is invalid`) apart from a missing one.
- `GIT_WORK_HOST='m;x'` renders `host invalid`, and the run provenance
  line from ISSUE-57 adds the declaration hint once per run.
- Receipts are not re-validated on read, so existing receipts stay
  readable. A pre-mutation outcome or legacy cleanup plan written with a
  non-conforming host is refused on read; `mbp` and `mst` conform, and
  no such host appears in the test fixtures.
- Verified by `TestValidateHostNameCatalogGrammar` (accepts `mbp`,
  `mst`, `a1-b2`; rejects uppercase, `;`, leading, trailing and double
  hyphens, empty, non-ASCII and spaces),
  `TestHostFieldsUseCatalogGrammar`, `TestResolveCallerOwnerHostGrammar`,
  `TestResolveDeclaredDevice` and
  `TestIntegrateRunOwnerHostFlagUsesDeviceGrammar` (`--owner-host MBP`,
  `m;x` and `a--b` exit 2 naming `owner-host`).
