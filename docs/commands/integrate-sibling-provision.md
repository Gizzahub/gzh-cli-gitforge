# Sibling path-dependency provisioning

`gz-git integrate provision siblings acquire --json` materializes the
repository-root `branch.siblingProvision` declaration beside the current task
worktree. It accepts no commands or source paths from the declaration: a
source is either explicitly supplied with `--source name=/absolute/checkout`
or discovered as the declared sibling adjacent to the registered primary
checkout. The source must be clean, primary, and have the declared canonical
remote; `source-tip` must equal its local `origin/<sourceBranch>` tracking tip.

The command prints an opaque lease ID. `inspect --json` lists unchanged leases
bound to the current task root, and `release --lease ID --json` removes only
the matching, unchanged owned archive. A collision, changed archive, symlink,
or mismatched receipt fails closed. Receipts live under the repository Git
common directory, never in the committed worktree.
