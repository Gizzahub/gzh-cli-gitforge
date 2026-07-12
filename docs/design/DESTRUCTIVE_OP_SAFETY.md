# Destructive-Operation Safety Policy

Status: **active** · Applies to: all `gz-git` commands that delete or overwrite data.

This is the review criterion for any command that removes branches, files,
worktrees, stashes, or history. New destructive commands MUST comply, and code
review SHOULD link here.

## Why

`gz-git` is bulk-first: one command can act on dozens of repositories at once.
The most dangerous combination (bulk × destructive) previously had the lightest
guard — a single `--force` flag with no prompt — while a non-destructive
`workspace sync` asked for `--interactive`. This policy removes that inversion by
making the guard proportional to the blast radius.

## The Policy

1. **Dry-run is the default.** A destructive command run with no execution flag
   previews what it would do and changes nothing.

2. **Execution requires `--force`. Bulk × destructive additionally requires
   confirmation.** On an interactive terminal the command prints the deletion
   summary and asks `y/N`. In a non-interactive environment (pipe / CI) it
   refuses unless `--yes` is also given. `--yes` never substitutes for `--force`.

3. **Every destructive command offers `--dry-run` (`-n`).** Even single-target
   commands (e.g. `worktree remove`) must let the user preview without acting.

4. **Mutating bulk commands handle SIGINT/SIGTERM gracefully.** On interrupt the
   command prints `cancelling...`, stops dispatching new work, and reports the
   partial result — it is never hard-killed mid-write.

## Decision Matrix

| Command             | Destructive | Bulk | Default   | Execute flag | Confirm gate            |
| ------------------- | ----------- | ---- | --------- | ------------ | ----------------------- |
| `clean`             | yes         | yes  | dry-run   | `--force`    | prompt / `--yes`        |
| `cleanup branch`    | yes         | both | dry-run   | `--force`    | bulk: prompt / `--yes`  |
| `worktree remove`   | yes         | no   | acts¹     | (n/a)        | none (single target)    |

¹ `worktree remove` acts by default but now supports `--dry-run (-n)` for preview
(policy point 3). Single-target destructive ops do not require the confirm gate
(policy point 2 scopes it to bulk); `git`'s own uncommitted-change guard still
applies unless `--force` is passed.

## Shared Implementation

The primitives live in `cmd/gz-git/cmd/bulk_common.go` so every command behaves
identically. Reuse them — do not reimplement:

- `withInterruptCancel(ctx)` — derives a context cancelled on SIGINT/SIGTERM
  (policy point 4). Defer the returned cancel func.
- `confirmDestructiveBulk(assumeYes)` — the confirm gate (policy point 2). The
  caller prints the summary first; this asks `y/N` on a TTY and refuses a
  non-interactive run unless `assumeYes` (the `--yes` flag) is set.
- `stdinIsInteractive()` / `readYesNo(r)` — TTY detection and answer parsing.

## Checklist for a New Destructive Command

- [ ] Preview-only by default; no data changes without an explicit execute flag.
- [ ] `--dry-run` / `-n` previews the action.
- [ ] If bulk: prints a deletion summary, then calls `confirmDestructiveBulk`;
      refuses non-interactive runs without `--yes`.
- [ ] Wraps its context with `withInterruptCancel` and reports partial results.
- [ ] Exit codes follow the bulk contract (0 all-success, 1 tool/config error,
      2 partial repo failure).
