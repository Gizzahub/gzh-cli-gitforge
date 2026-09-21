# Integrate without fetching

`gz-git integrate check --no-fetch` and `gz-git integrate run --no-fetch`
resolve the target from the local remote-tracking refs and never fetch.
Use it when the network is unavailable or the local snapshot is already
current — for example, right after a fetch, or inside an offline finish
sequence where the tracking refs are known good.

`--no-fetch` only skips the read that refreshes local refs from the
remote. It does not make the command offline: `run --no-fetch` still
writes to the network — it pushes the integrated commit to the target
branch, and reclaim pushes a leased delete of the remote task branch.
Both of those pushes require connectivity even with the flag set.

```sh
gz-git integrate check --no-fetch
gz-git integrate run --no-fetch
```

## Fail-closed rules

No-fetch trusts nothing it cannot see locally, so every gap fails instead
of guessing:

- The declared integration branch must have a local tracking ref
  (`<remote>/<branch>`). When it is absent, check fails rather than
  falling back to a possibly stale local branch of the same name.
  Fetch once without the flag to create the ref.
- `run` revalidates the checked target SHA against the same local ref and
  pushes with `--force-with-lease` against that SHA. A target that moved
  remotely after the local snapshot refuses the lease and the run fails.
- Reclaim deletes the remote task branch with a lease against the commit
  that just landed. When that delete fails, no-fetch cannot probe the
  remote to tell "already deleted" from "delete refused", so reclaim
  reports incomplete instead of claiming success. Retry without
  `--no-fetch` to let the verification run.

## When not to use it

Do not use `--no-fetch` as the default path. A normal run fetches first,
which is what keeps the freshness check and the push lease honest. Use
no-fetch only as a deliberate finish step on a fresh snapshot, and treat
any failure as a signal to fetch and re-run rather than to retry the flag.
