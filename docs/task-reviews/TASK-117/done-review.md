# TASK-117 done review

Independent review by an equal-grade reviewer (opus, read-only, no repo
gates), first pass delivered 2026-11-04T17:23Z: **1 blocker, 2
recommended fixes** — all applied before the close. The report itself
validated the sweep-test re-architecture: the reviewer ran the sweep on
the pre-fix tree and `TestGroupParentsRefuseUnknownSubcommands/forge_config`
was the package's only failure.

## Findings and resolutions

1. **Blocker — `forge config` parent missed.** Registered by the
   `reposynccli` factory rather than `cmd/gz-git/cmd`, so the
   implementation list missed it. Resolution, the four coordinated pieces
   exactly as the reviewer scoped: the RunE help pattern in
   `pkg/reposynccli/config_command.go`, `"forge config": nil` in
   `commandEffects`, a `readOnlyRuns` entry, and the
   `expectedGroupParents` pin.
1. **Duplicated comment block** at `command_effect.go:69-74` — removed.
1. **Fence symlink test not discriminating.** The reviewer verified
   empirically that `rev-parse --git-common-dir` returns the physical
   path for both spellings and that flock crosses spellings on one inode,
   so the test passed even with the canonicalization reverted.
   Resolution went one step past the finding: a gitdir pointer file
   rewritten through a symlink is ALSO canonicalized by git before
   rev-parse answers (new empirical result, now asserted by the test's
   sentinel), so a black-box derivation pin is impossible in principle —
   no git-reported spelling reaches the fence un-canonicalized. The
   discrimination moved to a direct unit pin
   (`TestCanonicalCommonDirResolvesSymlinkedSpellings`:
   `canonicalCommonDir` of a symlinked spelling must equal the
   EvalSymlinks-physical spelling, and an unusable input fails fast),
   and the fence test now claims only what it proves — exclusion across
   spellings, current-derivation consistency, and a sentinel that fails
   with upgrade instructions if git ever starts reporting symlinked
   spellings.

## Confirmed dimensions (first pass)

- Every other enumerated parent carries the pattern correctly with
  `Use`/`Short`/`Long` and flags untouched (the reviewer enumerated the
  `AddCommand` registrations independently and found `forge config` the
  only miss).
- No broken callers (`branch_test.go:43` is a comment; the doc hits are
  stale archive docs).
- Effect-table keys and the exact-key contract claim match the
  implementation.
- No shadowing in the fence edit; the `canonicalCommonDir` comment is
  accurate.
- `validateOwner` removal confirmed; kind/host validation remains
  enforced at `receipt.go:123-143`.
- The parity supersession note matches the v0.15.0 facts (`89e43b2c`,
  removed subcommands per the TASK-114 close).

## Confirmation pass (post-fix, f31fb93a)

The same reviewer was asked to confirm the resolutions on
`5e3d9061..f31fb93a` before the card closed (verdict delivered
2026-11-04T17:57Z): **all three findings RESOLVED, no omissions, ready
to close** — re-verified by the reviewer's own read-only test run
(`TestGroupParentsRefuseUnknownSubcommands`,
`TestCommandEffectGroupParentContract`,
`TestEveryRunnableCommandDeclaresEffect`,
`TestReadOnlyRunsCoverDeclarations`,
`TestCanonicalCommonDirResolvesSymlinkedSpellings`,
`TestTaskResourceFence` — all PASS, including the forge_config subtest
and all 22 walked parents).

On criterion 3 the reviewer judged the approach **adequate — "as
strongly as is observable"**, and better than a plain derivation test:

- The unit pin discriminates exactly where discrimination is possible;
  a canonicalizer regression fails it.
- The fence test asserts a real string-level observable (the derived
  lock dir equals physical-common + fence dir) plus cross-spelling
  exclusion on the one derived file.
- The sentinel is the key move: it turns "git canonicalizes every
  spelling it reports" from the first pass's empirical claim into a
  tested invariant with an actionable failure message. Under that
  pinned invariant, raw-join and canonical derivations are provably
  identical strings, so the residual gap (deleting the
  `canonicalCommonDir` call inside `acquireTaskResourceFence` would not
  fail a test) has no observable behavior to discriminate — which the
  test comments now state honestly. A full wiring pin would need a fake
  executor returning a symlinked spelling, but `gitcmd.Executor` is a
  concrete struct, so that is an interface extraction — production
  refactoring outside this card, recorded as optional future hardening.

Range hygiene confirmed: single commit, task files only, clean working
tree (post-review deltas were the four blocker pieces, the comment
dedup, and the fence/unit test work), gofmt clean on all fix-touched
files.

## Out-of-scope findings (recorded per findings)

1. `TestExecutionRunLockSharesOneFileAcrossPathSpellings`
   (`execution_run_lock_test.go`) has the same exclusion-only shape the
   fence test was fixed out of — its `ErrResourceBusy` assertion holds
   through a reverted canonicalization because flock contends on one
   inode. It predates this task; the shared helper's new unit pin covers
   the derivation it relies on, so upgrading that test the same way is a
   candidate for a later card, not this one.
