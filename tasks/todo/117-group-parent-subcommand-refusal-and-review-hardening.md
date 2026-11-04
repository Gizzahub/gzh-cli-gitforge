---
id: TASK-117
title: "Refuse unknown subcommands on every group parent and land the TASK-114/115 review hardening follow-ups"
type: bug
priority: P2
effort: M
exec-tier: strong
created: 2026-11-04
status: todo
---

## Summary

TASK-115 made the `integrate` parent refuse an unknown subcommand with
exit 1 instead of printing the Quick Start and exiting 0. The same
swallow pattern is still open on every other group parent, so a stale
caller of any retired or mistyped subcommand reads success. The TASK-114
and TASK-115 reviews also left three named hardening follow-ups that
belong with this sweep. This card carries the whole post-review
hardening batch.

Group parents with children today (from `AddCommand` registrations,
2026-11-04): `branch`, `cleanup`, `config` (plus nested `profile`,
`token`), `conflict`, `forge` (plus nested `bootstrap`), `handoff`,
`history`, `pr`, `run`, `stash`, `tag`, `worktree`, and nested
`integrate provision`/`provision siblings`. The implementing session
re-enumerates this set from the tree rather than trusting the list.

## Execution Boundary

`cmd/gz-git/cmd/` group-parent files and their tests (Args validators of
the `cobra.NoArgs` family or an explicit unknown-command error), the
`command_effect.go` leaf/has_subcommands contract plus its table entries
if a parent's refusal changes the effect surface, the task resource
fence derivation (`pkg/integrate/task_resource_fence.go`),
`validateOwner` removal in `pkg/integrate/device.go` with its test, and
the parity-supersession doc
note. No behavior additions to `integrate`'s final surface, no flag
changes, no version bump beyond the normal release flow.

## Completion Criteria

- [ ] every group parent refuses an unknown subcommand argument with exit 1 and an explicit unknown-command message, while a bare parent invocation keeps its current help behavior | verify: a table-driven test per parent (or one sweep test) fails on `parent unknown-sub` before the change and passes after; `go test ./cmd/gz-git/... -count=1`
- [ ] the `command_effect.go` effect table encodes the leaf/has_subcommands contract explicitly — an unknown subcommand under a group parent cannot resolve to a leaf effect by exact-path lookup ambiguity, and the table documents that lookup is exact-key only | verify: `go test ./cmd/gz-git/cmd/ -run 'TestCommandEffect' -count=1` covers a group-parent/unknown-sub combination
- [ ] `pkg/integrate/task_resource_fence.go:45-57` derives its common dir through the same canonicalization the run lock and receipts use (`canonicalCommonDir`), not the raw `rev-parse --git-common-dir` value | verify: `go test ./pkg/integrate/ -run 'TestTaskResourceFence|TestSame' -count=1` with a symlink-spelling test pinning derivation consistency
- [ ] `validateOwner` (`pkg/integrate/device.go:51`) — production callers removed by TASK-114, only `device_test.go:52,55` exercises it — is removed together with its test, or regains a production caller if one is found | verify: `grep -rn 'validateOwner' pkg/ cmd/ internal/` shows no definition-and-test-only remnant
- [ ] the TASK-114 parity documentation records that the parity fixtures are superseded by the v0.15.0 final surface (the note the TASK-114 review requested) | verify: `grep -rn 'superseded' docs/` names the parity fixtures and the final-surface release

## Notes

- The TASK-115 automated-security finding on `command_effect.go` was
  judged not exploitable (exact-key lookup, no prefix consumer); the
  contract hardening here is prophylactic, and that analysis is recorded
  on the TASK-115 card and its review docs.
- The `integrate` parent itself already refuses (TASK-115
  `944fea97`); nested `integrate provision` parents are in scope for the
  same refusal.
- Discovered 2026-11-04, post-v0.15.0; nothing here is release-blocking.
