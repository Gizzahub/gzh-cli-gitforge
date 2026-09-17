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
- the default branch is `refs/remotes/<remote>/HEAD`, read locally in both
  modes. A normal run's fetch can create that ref when it is missing (git 2.48
  and later, through `remote.<name>.followRemoteHEAD`). Without a fetch a
  missing ref stays missing, so `--no-fetch` fails with
  `<remote>/HEAD is missing; cannot detect the default branch` instead of
  guessing. Recreate it with `git remote set-head <remote> --auto` outside the
  no-fetch policy;
- freshness, merge-tree, and cross-merge rows compare against those local refs.

The check says so. When the repository has a remote configured, its report
gains a `SKIP fetch` row naming the local ref and commit it judged against, and
the freshness row ends with `(local ref, not fetched)`. A repository with no
remote never fetches in either mode, so neither marker appears there. A no-fetch `READY` means "ready against what this
checkout last saw", never "ready against the remote now".

## Why the push is a sufficient freshness guard

A local ref can be stale, so the check alone could approve a branch whose target
has since moved. The guard is the push itself. `run` updates the target with
`--force-with-lease=refs/heads/<target>:<checked target commit>`, having first
verified that the checked target commit is an ancestor of the source commit. The
remote accepts that update only if its target still points at exactly the
commit the check judged. So the update is a fast-forward from the checked
commit, and it is applied to that commit or not at all. That includes a target
rewound to an older commit: a plain push would accept the update as a
fast-forward from there, but the lease names the checked commit and refuses it.

If anyone moved the target after the local ref was last fetched, the remote
rejects the push. `run` then fails with a non-zero exit, names the stale local
judgement in its error, and does not reclaim anything: the task worktree, local
branch, and remote branch all stay. The fix is the ordinary one: fetch, rebase,
and check again, outside the no-fetch policy if necessary.

The push still reads the remote's ref advertisement as part of the push
protocol. That is inherent to pushing, and the policy forbids fetching, not
pushing.

## Lazy fetches in partial clones

A partial clone (`git clone --filter=...`) fetches a missing object on demand
whenever a command needs it, and merge-tree or diff can need one. Under
`--no-fetch`, every git subprocess that `check` and `run` start themselves runs
with `GIT_NO_LAZY_FETCH=1`, so such a command fails on the missing object
instead of reading the remote. The variable is not set without the flag.

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
- Two commands that gz-git itself starts can still reach the network, in
  addition to the repository gate above. Both run only when a controller
  preparation profile is in use. The profile runs `go generate ./ent`, which
  may download a Go toolchain or modules into its isolated module cache, and
  each prepared probe then runs `go env GOROOT`, which may download a toolchain
  when the checked-out module asks for one. Neither is a git subprocess, so
  `GIT_NO_LAZY_FETCH` does not apply to them.
- `integrate bootstrap` and `integrate readiness update` do not take this flag
  and still fetch.

Without `--no-fetch`, both commands behave exactly as before.
