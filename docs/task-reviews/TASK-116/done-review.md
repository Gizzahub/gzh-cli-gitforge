# TASK-116 done review — prepare the v0.15.0 release

- Reviewer: independent read-only opus subagent (`review-116`), 2026-11-04.
- Verdict: **PASS WITH MINORS** — one factual error in the preparation
  doc, fixed before commit; nothing release-blocking.
- Implementation was lead-authored (version/docs-only chore), so the
  review was assigned to a separate agent per the review-separation
  rule; the reviewer ran no repo gates (a full gate had already passed
  on this exact tree) and inspected only.

## What the reviewer verified

1. **Diff scope** — `git status` shows exactly `VERSION`, `version.go`
   (one hunk, `defaultVersion = "0.15.0"` at line 42), the new
   `docs/releases/0.15.0-preparation.md`, and the task card. The
   committed result is exactly `944fea97` + this change.
1. **Version consistency** — `cat VERSION` → `0.15.0`; no stale `0.14.2`
   references remain outside range/history mentions. Release builds
   stamp the version via ldflags (`.goreleaser.yaml:55`), so
   `defaultVersion` is only the unstamped fallback — no behavior change.
1. **Included-commits list** — `git log --oneline v0.14.2..944fea97`
   returns exactly the six documented commits, none missing or extra.
1. **Surface-removal claims** — live CLI registers exactly
   run/cleanup/receipt/queue/provision on `integrate`; the three removed
   subcommand files are absent; all seven legacy flag literals grep to
   nothing in `cmd/gz-git/cmd/`; ADR-0205 decision 5 authorizes exactly
   this removal release.
1. **release.yml description** — `workflow_run` on master CI success,
   publish gated on `conclusion == 'success' && event == 'push'`, the
   tag-a-VERSION-bump step tags `HEAD_SHA` when newer than the latest
   release tag and pushes it, then GoReleaser publishes binaries and the
   Homebrew cask in the same job. v0.15.0 > v0.14.2, so the path fires.
1. **Card criteria reproduction** — criteria 1-2 reproduce (criterion 2
   counts 7 hits: `944fea97` appears twice, other SHAs once).

## Findings applied before commit

- `docs/releases/0.15.0-preparation.md` line for `0644befd` said the
  tasksync co-process was "not wired into the CLI" — false: the commit
  adds `cmd/gz-git/cmd/task_sync_apply.go` registering the hidden
  `task-sync-apply` stdio command via `rootCmd.AddCommand`
  (`task_sync_apply.go:57`). Reworded to the reviewer's exact
  recommendation.
- List order corrected to actual log order (`52ef273d` before
  `0644befd`); confirmed by the lead against `git log`.
- The verification-path section omitted that the widened full profile
  passed exit 0 on this tree; one sentence added so the record is
  self-contained.

## Accepted minors (left as-is, with rationale)

- The ADR-0161 citation attaches to the removal claim; the resulting
  five-command surface sentence is verified against the tree. ADR-0161
  itself specifies four commands with provision out of scope/retained
  (owner decision, ISSUE-098); TASK-114's card records that precisely.
- The flag enumeration lists the ADR contract arguments, not every
  retained flag (run also keeps `--direct-to-default`/`--release`/
  `--controller-config`/`--no-fetch`/`--json`; cleanup/receipt `--json`).
- "`5756e6f7c` 릴리스" is loose — no tag contains `5756e6f7c`; the
  released host build was `v0.11.0-5-g5756e6f7c`. The zero-need premise
  holds under every reading because ce v0.11.0 itself removed the six
  deprecated call sites.

## Noted for the release-notes intro (post-publish)

Generated notes exclude `docs:`/`chore:` subjects
(`.goreleaser.yaml` changelog filters), so `17403bb4`, `52ef273d`, and
`618469b7` will not appear in the generated body; the planned removal
summary intro should name them so the release record stays complete
(TASK-114 note).
