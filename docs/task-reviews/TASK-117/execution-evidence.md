# TASK-117 execution evidence

All commands run in the task worktree
`claude__mbp__fix__117-group-parent-refusal-hardening` (branch
`dev/claude/mbp/fix/117-group-parent-refusal-hardening`, base origin/master
`5e3d9061`). `GOWORK=off` on every go/make invocation.

- Build: covered by `make verify-full` (run 1 and run 2 below).
- Criterion 1: `GOWORK=off go test ./cmd/gz-git/cmd/ -count=1` →
  `ok cmd/gz-git/cmd 167.003s` — the sweep
  (`TestGroupParentsRefuseUnknownSubcommands`) walks the tree-enumerated
  parent set, including `forge config` after the review blocker fix, and
  the full package including the readonly write-nothing suite passes.
- Criterion 2: `go test ./cmd/gz-git/cmd/ -run 'TestCommandEffect' -count=1`
  → PASS — `TestCommandEffectGroupParentContract` covers the
  group-parent/unknown-sub combination (probe key asserted absent from
  `commandEffects`, exact-key-only contract documented in the table).
- Criterion 3: `go test ./pkg/integrate/ -run 'TestTaskResourceFence|TestSame' -count=1`
  → PASS — fence derivation now goes through `canonicalCommonDir`;
  derivation consistency is pinned at the unit level
  (`TestCanonicalCommonDirResolvesSymlinkedSpellings`, receipt_v2_test.go)
  plus the fence test's exclusion check, current-derivation consistency
  check, and gitdir-pointer sentinel (see done-review.md for why a
  black-box derivation pin is impossible: git canonicalizes every spelling
  it reports).
- Criterion 4: `grep -rn 'validateOwner' pkg/ cmd/ internal/` → no matches.
- Criterion 5: `grep -rn 'superseded' docs/` →
  docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md:3 names the `tests/parity/`
  fixtures and the v0.15.0 final surface.
- Reposync/workspace factories touched by the sweep:
  `go test ./pkg/reposynccli/... ./pkg/workspacecli/... -count=1` → ok.
- Format: `make fmt` then `make format-check` → clean.

## verify-full history

- Run 1 (lead, code-final tree, before the delivery commit): exit 0, full
  verification profile passed — format, lint, security, build, unit, E2E,
  coverage floors (`reposync` landed exactly on its 61.0% floor). The tree
  gated here is byte-identical to the code committed as `f31fb93a`; the
  card had not yet moved to `done/`, so this run is supplementary, not the
  authoritative one.
- Run 2 (authoritative): exact final tree — card in `done/`, criteria
  ticked, review docs in place, `make fmt` clean — launched immediately
  before the docs(tasks) close commit; no tree change between gate and
  commit. Result: exit 0, full verification profile passed.
