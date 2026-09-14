# Readiness bootstrap

`gz-git integrate bootstrap` is the deliberately narrow recovery path for a
target that predates the target-owned readiness contract.

```sh
gz-git integrate bootstrap plan --issuer alice --target origin/master \
  --output /tmp/readiness-bootstrap.json
gz-git integrate bootstrap apply --plan /tmp/readiness-bootstrap.json \
  --confirm <CONFIRM_DIGEST>
```

The two commands cannot be chained. `plan && apply` can never work: `apply`
requires both the plan file and the `CONFIRM_DIGEST` that `plan` prints to
stderr, and a human has to read that digest before retyping it. `plan` prints
the exact `apply` command to run next, but running it stays a separate,
deliberate act.

## Issuer

`--issuer` records who is accountable for the plan. When it is omitted, `plan`
reads `git config gzgit.issuer`, then `git config user.email`, and fails naming
all three when neither is set:

```sh
git config --global gzgit.issuer alice@example.com
```

`gzgit.issuer` is owned by gz-git, so external identity tooling can populate it
without gz-git depending on any particular identity framework or file format.

The environment is never consulted for the issuer. It is an audit field, and an
environment variable is the one source a process can set for itself, in flight,
without leaving a reviewable trace on disk.

`plan` never changes remote refs, though it may update local fetch state. It emits an expiring canonical
confirmation plan (it is not a signed authorization). `apply` is an
explicitly human-operated action: it requires `--confirm <sha256>` equal to
the displayed canonical plan digest. It succeeds only when the source is exactly one fast-forward
commit ahead, the target has no readiness declaration, and that commit changes
only `.gz-git.yaml` by adding `branch.readiness` plus regular files below
`.gz-git/readiness/`. The plan records the repository URL, remote, both refs
and object IDs, manifest/runner/tree IDs, tree digest, issuer, operation ID,
and expiry.

`apply` accepts only a plan file. It fetches and recomputes all of those facts,
fails closed on drift or expiry, and performs one exact
`--force-with-lease` fast-forward push. Once the target declares readiness the
plan cannot be reused. No cleanup is attempted before the push.

This is a bootstrap transaction, not a general-purpose file copier: symlinks,
submodules, additional config changes, and multi-commit branches are rejected.

The CLI does not authenticate the operator, and `apply` refuses to run without
an interactive terminal: the digest is a human review acknowledgement, and a
pipe or CI runner has nobody to supply it. There is no `--yes` or environment
bypass. The downstream agent policy and PreToolUse hook must still deny
`integrate bootstrap apply` -- the terminal check narrows the blast radius, it
does not identify the operator -- and execution stays reserved for a separately
reviewed human-operated procedure.
