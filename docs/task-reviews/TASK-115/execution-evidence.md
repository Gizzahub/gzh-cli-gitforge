# TASK-115 execution evidence

All commands run in the task worktree
`claude__mbp__fix__run-lock-canonicalize-and-unknown-refusal` (branch
`dev/claude/mbp/fix/run-lock-canonicalize-and-unknown-refusal`, base
origin/master `f640293b`). `GOWORK=off` on every go/make invocation.

- Build: `go build ./...` → clean (re-run clean after `make fmt`).
- Criterion 1: `go test ./pkg/integrate/ -run 'TestExecutionRunLock|TestSameExecution' -count=1`
  → `ok pkg/integrate 1.465s` (impl; reviewer re-ran the same command
  PASS; lead re-ran on the final tree after comment rewording → PASS).
- Criterion 2: `go test ./cmd/gz-git/cmd/ -run TestIntegrateRefusesUnknownSubcommands -count=1`
  → `ok cmd/gz-git/cmd 0.510s` (impl, reviewer, lead final re-run).
- Format: `GOWORK=off make fmt` → complete, no complaints.

## verify-full history

- Run 1 (lead, code-final tree, card still in `todo/`): stopped by the
  lead after ~4 minutes — the card had not yet moved to `done/` with its
  done frontmatter, so the run would not have validated the exact commit
  tree (the stale-tree failure mode TASK-114 hit). Stopped before any
  verdict; not a gate failure.
- Run 2 (authoritative): exact final tree — card in `done/`, criterion 3
  ticked, review docs in place, `make fmt` clean — launched immediately
  before commit → exit 0, full verification profile passed; no tree
  change between gate and commit.

Criterion 3 binds `VERIFY_BASE=origin/master make verify-commit`; the
diff contains no deleted paths so the commit profile would not widen,
but the lead ran the full profile for v0.15.0 release confidence (the
user's 2026-11-04 scope decision fixed both findings before tagging).
