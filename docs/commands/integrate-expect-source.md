# Pin the approved source and target commits

`gz-git integrate check --expect-source <sha>` and
`gz-git integrate run --expect-source <sha>` refuse to measure or integrate a
source that is not exactly that commit. A caller that holds an approval for one
commit — CE `ce task run-release` holding a release record's `source-sha` —
passes it so the promotion cannot widen to a descendant that a concurrent fetch
(an IDE auto-fetch, another shell) moved the tracking ref to.

`--expect-target <sha>` independently pins the integration target. It prevents
an immutable request from silently adopting a target tip that moved after the
request was created. The source and target values use the same full-SHA syntax
but name different sides of the integration.

```sh
gz-git integrate check origin/develop --target origin/master --release --expect-source <40-hex> --expect-target <40-hex>
gz-git integrate run   origin/develop --target origin/master --release --no-fetch --expect-source <40-hex> --expect-target <40-hex>
```

## Rules

- The value is a full object ID: 40 (SHA-1) or 64 (SHA-256) hex digits.
  Uppercase is lowercased to match `rev-parse`. An abbreviation or any other
  shape is a usage error (exit 2).
- The source is read after the target fetch, and that read is both the value
  compared and the commit readiness measures (`Plan.BranchSHA`). A stale local
  tracking ref that the fetch advances to the expected commit therefore passes.
- `run` compares again after revalidating the checked refs, before the push.
- A mismatch exits **4** with `source <branch> is <sha>, expected <sha>` and
  pushes nothing. Callers distinguish it from NOT READY (1) by the code alone
  (`integrate.ErrSourceMismatch` in Go).
- The flag is not tied to `--release`; it means the same on a task branch.
- The target is compared after fetch and before any gate or intent receipt.
  `run` compares it again after the gate; a confirmed lease rejection caused
  by target movement exits **4**, while authentication, policy, and unrelated
  push failures retain their own error classification.

Capability detection: the flag appears in the `Flags:` section of
`integrate check --help` and `integrate run --help`, as `--no-fetch` does.

## Freshness before expensive validation

An approved source SHA can still be behind the fetched integration target.
`--expect-source` pins identity; it does not make the candidate current.
Freshness failure returns NOT READY (`run` exits 1) before readiness runners,
preparation, or Make gates. Nothing is pushed or reclaimed. Unexecuted gate rows
are absent, not PASS, and validation / resource-wait durations remain zero.

Refresh through the adopted repository lifecycle and review the new input before
invoking native integration once. Fresh candidates retain their mandatory gates,
post-gate ref checks, push leases, reclaim protections, and validation timing.
This does not introduce automatic rebase, retries, batching, or a result cache.
