# Integrating without fetching

`gz-git integrate check --no-fetch` and `gz-git integrate run --no-fetch` finish
an integration without reading anything from the remote. They exist for callers
that run under a policy forbidding network reads, such as a CE task runtime with
`integration-network-policy: no-fetch`.

```sh
gz-git integrate check --no-fetch
gz-git integrate run --no-fetch
```

## Decision: complete, not merely refuse

A no-fetch policy could have been answered in two ways: by guaranteeing that
gz-git refuses, or by giving it a path that actually lands the work. gz-git
takes the second. A guaranteed refusal is safe but leaves every repository under
that policy without a finishing path, so the caller has to switch to a different
command and clean up its own bookkeeping by hand. That turns a policy about
network access into a permanent workflow fork, which is a worse outcome than the
risk the policy was meant to contain.

Completing is only acceptable if it cannot land work on a stale target. The rest
of this page is why it cannot.

## What "local refs" means

Without a fetch, the check judges against what the repository already knows:

- the target is the remote-tracking ref already present locally, for example
  `refs/remotes/origin/develop`;
- the default branch is `refs/remotes/<remote>/HEAD`, which is read locally in
  both modes because `git fetch --prune` never updates it;
- freshness, merge-tree, and cross-merge rows compare against those local refs.

The check says so. Its report gains a `SKIP fetch` row naming the local ref and
commit it judged against, and the freshness row ends with
`(local ref, not fetched)`. A no-fetch `READY` means "ready against what this
checkout last saw", never "ready against the remote now".

## Why the push is a sufficient freshness guard

A local ref can be stale, so the check alone could approve a branch whose target
has since moved. The guard is the push itself. `run` updates the target with
`--force-with-lease=refs/heads/<target>:<checked target commit>`, having first
verified that the checked target commit is an ancestor of the source commit. The
remote accepts that update only if its target still points at exactly the
commit the check judged. So the update is a fast-forward from the checked
commit, and it is applied to that commit or not at all.

If anyone moved the target after the local ref was last fetched, the remote
rejects the push. `run` then fails with a non-zero exit, names the stale local
judgement in its error, and does not reclaim anything: the task worktree, local
branch, and remote branch all stay. The fix is the ordinary one: fetch, rebase,
and check again, outside the no-fetch policy if necessary.

The push still reads the remote's ref advertisement as part of the push
protocol. That is inherent to pushing, and the policy forbids fetching, not
pushing.

## What still happens, and what is refused

- Reclaim still runs after a successful integration. Removing the worktree and
  the local branch is local; deleting the remote task branch is a leased push.
- When that leased delete is refused, a normal run asks `git ls-remote` whether
  the branch is already gone. Under `--no-fetch` that read is skipped, and the
  refused delete is reported as an incomplete reclaim (exit 3) instead of being
  guessed as already deleted.
- A target ref that is missing locally is an error. `--no-fetch` never
  substitutes a fetch to find it.
- The repository's own readiness gate (the contract runner or `make check` and
  `make lint`) runs unchanged. gz-git does not stop that gate from reaching the
  network; a repository that needs its gate offline must make the gate offline.
- `integrate bootstrap` and `integrate readiness update` do not take this flag
  and still fetch.

Without `--no-fetch`, both commands behave exactly as before.
