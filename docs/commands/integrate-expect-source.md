# Pin the approved source and target commits

`gz-git integrate run --source <sha>` refuses to measure or integrate a source
that is not exactly that commit. A caller that holds an approval for one
commit — CE `ce task run-release` holding a release record's `source-sha` —
passes it so the promotion cannot widen to a descendant that a concurrent fetch
(an IDE auto-fetch, another shell) moved the tracking ref to.

`--target <ref>@<sha>` pins the integration target. It prevents an immutable
request from silently adopting a target tip that moved after the request was
created. The pin after the last `@` uses the same full-SHA syntax as `--source`
but names the other side of the integration.

```sh
gz-git integrate run origin/develop --target origin/master@<40-hex> --release
gz-git integrate run origin/develop --target origin/master@<40-hex> --release --no-fetch
```

## Rules

- Both values are full object IDs: 40 (SHA-1) or 64 (SHA-256) hex digits.
  Uppercase is lowercased to match `rev-parse`. An abbreviation or any other
  shape is a usage error (exit 2).
- The source is read after the target fetch, and that read is both the value
  compared and the commit the git checks judge (`Plan.BranchSHA`). A stale local
  tracking ref that the fetch advances to the expected commit therefore passes.
- `run` compares again after revalidating the checked refs, before the push.
- A mismatch exits **4** with `source <branch> is <sha>, expected <sha>` and
  pushes nothing. Callers distinguish it from NOT READY (1) by the code alone
  (`integrate.ErrSourceMismatch` in Go).
- `--source` is not tied to `--release`; it means the same on a task branch.
- The target pin is compared after fetch and before any intent receipt. `run`
  compares it again after revalidating the checked refs; a confirmed lease rejection caused
  by target movement exits **4**, while authentication, policy, and unrelated
  push failures retain their own error classification.
- A `--target` value whose text after the last `@` is a hex run of 7 or more
  characters that is not a full SHA is refused as truncated; any other value,
  `release@2026` included, stays the plain target ref.

Capability detection: the flags appear in the `Flags:` section of
`integrate run --help`, as `--no-fetch` does.

## Freshness before the other git checks

An approved source SHA can still be behind the fetched integration target.
`--source` pins identity; it does not make the candidate current.
Freshness failure returns NOT READY (`run` exits 1) before the remaining git
checks. Nothing is pushed or reclaimed. Rows of checks that did not run are absent,
not PASS.

Refresh through the adopted repository lifecycle and review the new input before
invoking native integration once. Fresh candidates retain their git checks,
ref revalidation, push leases, and reclaim protections.
This does not introduce automatic rebase, retries, batching, or a result cache.
