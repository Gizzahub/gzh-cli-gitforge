# ISSUE-41 CE429 parity reference refresh

Original investigation: 2026-10-18. Current rebase addendum: 2026-10-19.

## Reference and scope

- Installed CE: `/Users/archmagece/go/bin/ce`, 0.10.13, build `288-gd7c88ff6`.
- Full clean revision: `d7c88ff69dfdea6278e6ff87f5cbba80c7dc0d6d` (`vcs.modified=false`).
- Current target: `9174d8e64913b71aba835450890958d39e8572ff`.
- Literal physical capture host: `CE-Mac-Studio.local`.

The target's provider-capture code and intervening history were preserved during
conflict resolution. Only the CE reference pin, twelve generated fixtures and
reference evidence change. No normalizer was added or widened.

## Historical 2026-10-18 evidence

Before the latest rebase, capture passed in 33.611s and fresh non-record package
verification passed in 28.102s. The comparison against that earlier base had
37 changed leaves. These timings and classification are historical, not the
acceptance evidence for the current target.

## 2026-10-19 rebase evidence

Fresh twelve-scenario machine capture passed in 32.147s. A focused non-record
`TestCEParity` run passed in 34.290s. A subsequent full non-record parity package
run, including reference-pin and provider-capture tests, passed in 30.199s:

```bash
env -u PARITY_TARGET -u PARITY_RECORD \
  CE_BIN=/Users/archmagece/go/bin/ce GOWORK=off GOMAXPROCS=4 \
  go test -p 4 -parallel 1 -timeout 4m ./tests/parity/...
```

The complete decoded comparison against current target `9174d8e6` has 39 leaves:

- 12 golden reference fields;
- 12 CE toolchain build/revision stamps;
- 8 direct receipt `tool_version` fields;
- 5 `tool_version` fields inside captured JSON stdout;
- 2 literal physical `processHost` fields.

In `finish-explicit.json` and `finish-noarg-single-active.json`, physical host
provenance changes from the target capture's `CEui-MacBookPro.local` to this
capture's `CE-Mac-Studio.local`. Both values remain literal. Status, exit code,
allowed action, logical owner, refs, push, worktree and cleanup outcomes do not
change. This is reference adoption, not native-port or full-gate acceptance.

Raw logs are retained in the primary checkout under
`tmp/issue41-ce429-parity-reference/`: historical logs plus
`capture-d7c88ff6-rebase.log`, `verify-d7c88ff6-rebase.log` and
`verify-full-rebase.log`.
