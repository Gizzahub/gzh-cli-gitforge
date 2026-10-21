// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Package integrate implements the gz-git integrate command surface.
//
// queue lists unfinished task branches. check answers readiness. run
// fast-forwards an authorized target and reclaims the task branch.
// Remote reclaim deletes with --force-with-lease against the integrated SHA.
// gz-git never builds, executes, or lints repository code: the git checks
// (freshness, merge-tree, other branches, a clean working tree, a pushed
// source, and post-gate revalidation) alone decide readiness. Repository
// checks belong to the caller's own gate, run before integrate.
//
// Integration-branch lookup is not ResolveBase. ResolveBase only sees local
// refs/heads and prefers master over develop. This package uses a declared
// integrationBranch when present, otherwise the remote HEAD, and treats a
// missing base as a reportable state.
package integrate
