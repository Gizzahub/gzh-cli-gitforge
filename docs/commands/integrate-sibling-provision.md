# Sibling path-dependency provisioning

`gz-git integrate provision siblings acquire --json` materializes the
repository-root `branch.siblingProvision` declaration beside the current task
worktree. It accepts no commands or source paths from the declaration. A
declared sibling can be supplied with `--source name=/absolute/checkout`; when
it is not supplied, the source is discovered adjacent to the registered primary
checkout. The source must be clean, primary, and have the declared canonical
remote; `source-tip` must equal its local `origin/<sourceBranch>` tracking tip.

As observed with official build `b323`, an undeclared `name` in `--source` is
currently ignored and acquire falls back to the declared sibling. Do not treat
that form as arbitrary source selection; this known limitation is tracked in
`tasks/issue/49-undeclared-sibling-source-override.md`.

The command prints an opaque lease ID. `inspect --json` lists unchanged leases
bound to the current task root and reports `RESUMABLE` for a retained lease; it
does not provide a separate resume command. A new acquire colliding with an
existing archive fails closed. `release --lease ID --json` removes only the
matching, unchanged owned archive. A changed archive, symlink, or mismatched
receipt also fails closed. Receipts live under the repository Git common
directory, never in the committed worktree.

Sibling archive acquisition materializes instruction-file aliases before the
archive extractor runs. At the repository root, the existing `AGENTS.md`,
`CLAUDE.md`, and `GEMINI.md` alias compatibility remains available when the
target is a regular root instruction file. In nested directories, only
`CLAUDE.md` and `GEMINI.md` may alias the exact same-directory `AGENTS.md`;
they are copied as regular files with the target bytes and mode. Other target
forms, duplicate instruction entries, missing or non-regular targets, and
alias chains fail closed. Both alias and target use canonical slash archive
paths, and the rewritten archive is bounded by the normal task-chain archive
limit so aliases cannot amplify the accepted archive beyond that limit.
