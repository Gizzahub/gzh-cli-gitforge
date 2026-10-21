# Reclaiming declared workspace clones

A caller's own gate may run `gz-git sync`, which clones the
`workspaces:` entries of the repo-root `.gz-git.yaml` into ignored directories
of the task worktree. `integrate run` reclaim used to refuse every ignored
nested git repo, so such a repository ended each integration with reclaim
incomplete. Reclaim now removes a nested repo only when it is provably a
disposable copy of a declared clone.

## Conditions

All must hold for each nested repo; otherwise reclaim stops, names the path
and the failed condition, and leaves the worktree and branches in place.

- The worktree-relative path is a declared workspace path.
- `.git` is a directory. A gitfile or linked worktree keeps its history
  elsewhere, so deleting the directory proves nothing.
- `remote.origin.url` is the declared URL (trailing slash and `.git` ignored).
- The working tree is clean, untracked files included, and there are no stash
  entries.
- The clone has no initialized submodule (`.git/modules` is empty and
  `git submodule status --recursive` prints nothing) and no linked worktree
  (`git worktree list` shows one entry). A submodule's commits live outside
  the clone's own refs, and deleting the clone would orphan a linked worktree,
  so both are refused rather than inspected. A declared but uninitialized
  submodule (a `-` line in `git submodule status`) is ignored only while its
  path holds no `.git`; a repository placed there by hand is refused.
- No ignored entry inside the clone contains another git repo, at any depth.
- Every commit reachable from any ref (branches, tags, detached HEAD) is also
  reachable from a remote-tracking ref (`git rev-list --all --not --remotes`
  is empty), and every branch upstream is itself a remote-tracking ref.

## Accepted policy and known limits

- Ignored files that are not repos (a `.env`, build output) are deleted with
  the clone. This is what `git worktree remove` already does to the parent's
  ignored files; a clone is held to the parent's standard, not a stricter one.
- Only the repo-root `.gz-git.yaml` is read. Workspace configs in
  subdirectories (for example `deps/.gz-git.yaml`) are out of scope and their
  clones keep the refusal.
- Reflogs are not consulted.
- Verification is all-or-nothing: nothing is deleted unless every nested repo
  qualifies. Deletion is not atomic: if one removal fails, earlier ones stay
  removed and the failure is reported.
- Each deletion first re-resolves the path and requires it to be the verified
  location inside the resolved task worktree, so a symlink swapped in after
  verification cannot redirect it.
