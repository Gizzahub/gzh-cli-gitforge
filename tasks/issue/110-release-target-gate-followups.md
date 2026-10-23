---
id: ISSUE-110
title: "Release target gate: --expect-target is compared after the target declaration read, and a symlinked target declaration is undocumented"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-109 third review (f02ae7a4, released in v0.13.1)"
discovered-at: 2026-10-23
created: 2026-10-23
---

## Summary

[ISSUE-109](../_archive/issue/109-release-reads-target-working-tree-declaration.md) gates a
`--release` on the `branch.integrationBranch` committed at the target commit.
Its third review left two non-blocking follow-ups. Neither weakens the gate:
both fail closed.

### (a) `--expect-target` is compared after the target declaration read

`readReleaseDeclarations` (`pkg/integrate/check_target.go:231-249`) resolves
the target with `resolveReleaseTarget` and reads its declaration with
`loadTargetIntegrationBranch` at once. The `--expect-target` comparison
happens later, in `checkTargetSHA` (`pkg/integrate/check_target.go:253-261`).
When the target declaration is unreadable, for example a YAML syntax error or
a root `.gz-git.*` committed as a symbolic link, the release fails with the
declaration error, even when `--expect-target` also mismatches. The caller
then gets a generic failure, not `ErrTargetMismatch`, which maps to exit 4
(`cliutil.ExitSourceMismatch`) in `integrate check` and `integrate run`.

The "release target %s moved from %s to %s while the plan was resolved" error
in `checkTargetSHA` (line 255) is not wrapped in `ErrTargetMismatch` either,
so it does not map to exit 4.

### (b) A symlinked target declaration refuses every release

`committedDeclarationFiles` (`pkg/integrate/release_declaration.go`) refuses
a repository-root `.gz-git.*` committed as a symbolic link. Since ISSUE-109
it reads the target commit too, so such a target refuses every release. The
pre-ISSUE-109 working-tree read followed the link. The refusal message is
clear (`<file> at commit <short> is a symbolic link; commit the declaration as a regular file`). `docs/commands/integrate-release.md` mentions the refusal
"on either side" in passing. It does not say that this changed for the
target, or how to recover, which is to commit the declaration as a regular
file on the default branch.

## Reproduction

- (a) Use the `releaseDeclarationFixture` of
  `pkg/integrate/release_declaration_test.go` with an unparsable target
  declaration. `TestCheck_ReleaseRejectsUnparsableTargetDeclaration` builds
  one. Then set `ExpectTarget` to any full commit ID other than the target's.
  `Check` returns the `release target declaration: parse .gz-git.yaml ...`
  error, and `errors.Is(err, ErrTargetMismatch)` is false. The same holds
  for a target whose root `.gz-git.yaml` is committed as a symbolic link.
  This was found by reading the code in the review; no test asserts it yet.
- (b) Commit the default branch's root `.gz-git.yaml` as a symbolic link to a
  regular file, then run `gz-git integrate check <remote>/develop --target <remote>/<default> --release`. The check is refused with
  `.gz-git.yaml at commit <short> is a symbolic link; commit the declaration as a regular file`.

## Expected vs Actual

- Expected (a): a release whose target is not the `--expect-target` commit
  fails with `ErrTargetMismatch` (exit 4), whatever its declaration holds. A
  target that moves while the plan is resolved is a target mismatch too.
- Actual (a): an unreadable target declaration masks the mismatch, and the
  "moved" error is unwrapped.
- Expected (b): the release documentation states that a symlinked root
  declaration on the target refuses the release, that the working-tree read
  used to follow it, and how to recover.
- Actual (b): undocumented.

## Suggested fix

- (a) Compare `opts.ExpectTarget` right after `resolveReleaseTarget`, before
  `loadTargetIntegrationBranch`, and wrap the "moved" error with
  `ErrTargetMismatch`. Add a test with a broken target declaration and a
  mismatched `--expect-target` that asserts `ErrTargetMismatch`.
- (b) Add a paragraph to the "Declaration source" section of
  `docs/commands/integrate-release.md`.

## Scope

- In scope: the ordering and error wrapping in `pkg/integrate/check_target.go`
  and the documentation of (b).
- Out of scope: following symbolic links in committed declarations. The
  refusal stays.
