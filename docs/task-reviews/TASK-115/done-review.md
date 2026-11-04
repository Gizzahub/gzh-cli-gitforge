# TASK-115 done review

Independent review by a stronger-grade reviewer (opus, read-only, no repo
gates), verdict delivered 2026-11-04T12:07:18Z: **PASS WITH MINORS** —
nothing must-fix before commit.

## Premise correction (finding 1)

The automated security review had flagged the run lock's raw `common`
derivation as a path-aliasing mutual-exclusion bypass (MEDIUM). The
reviewer refuted that premise experimentally: `tryAcquireResourceLock`
flocks the lock file's open file description, and two spellings of one
common dir (canonical + symlink alias) open the same inode, so a second
acquisition already fails with `EWOULDBLOCK` (`os.SameFile` true). No
exploitable bypass existed; the only theoretical gap was a relative
`common`, and no production caller passes one (rev-parse is asked for
absolute).

Correction applied by the lead before commit in three places: the
`acquireExecutionRunLock` doc comment, the alias test's comment, and the
task card's Summary. The change now lands for derivation consistency
with the receipt paths and fail-fast on an unusable common — not as an
exploitable-hole closure.

## Confirmed dimensions

- Criterion 1 command re-run by the reviewer: PASS (both tests).
- Criterion 2 refusal behavior: PASS — retired and nonsense subcommand
  names exit 1 with an unknown-command message naming the arg; the bare
  parent prints the Quick Start with exit 0; real-subcommand dispatch
  unbroken (readonly command suite).

## Minor follow-up candidates (recorded, out of scope)

1. `task_resource_fence.go:45-57` derives from the raw common without
   canonicalization; weigh applying the same helper or documenting the
   cross-version contention intent at lines 17-19.
1. Nine other group parents (`branch`, `cleanup`, `conflict`, `forge`,
   `handoff`, `history`, `pr`, `sync`, `workspace`) still swallow unknown
   subcommand args the way `integrate` did before this task.
