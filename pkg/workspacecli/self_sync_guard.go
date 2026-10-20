// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// pointsAtConfigDir reports whether a resolved workspace path is the config
// directory itself, so sync never plans the devbox that holds the config.
// The string comparison catches the plain spellings; file identity catches
// a symlink to the same directory, or a spelling that differs only in case
// on a case-insensitive volume (ISSUE-80). A path that does not exist yet
// cannot be the config directory.
func pointsAtConfigDir(wsPath, configDir string) bool {
	if wsPath == configDir {
		return true
	}
	wi, err := os.Stat(wsPath)
	if err != nil {
		return false
	}
	ci, err := os.Stat(configDir)
	if err != nil {
		return false
	}
	return os.SameFile(wi, ci)
}

// reachesEnclosingConfigDir reports whether path is the config directory of
// a workspace that recursed into the one being planned, or an ancestor of
// such a directory. enclosing lists those directories and is empty when the
// root config is planned. A child's own guards compare its entries only with
// the child directory, so without this a child entry such as `path: ..`
// would plan, hook into, or delete the devbox that holds the root config
// (ISSUE-85). Ancestors are refused too, since a forge workspace there plans
// orphan deletes among its subdirectories.
func reachesEnclosingConfigDir(path string, enclosing []string) bool {
	for _, dir := range enclosing {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		starts := []string{dir}
		if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != dir {
			starts = append(starts, resolved)
		}
		for _, start := range starts {
			for d := start; ; d = filepath.Dir(d) {
				if pointsAtConfigDir(path, d) {
					return true
				}
				if filepath.Dir(d) == d {
					break
				}
			}
		}
	}
	return false
}

// keepActionsOffEnclosingConfigDirs drops the actions a child workspace plans
// for an enclosing config directory or one of its ancestors. The workspace
// guards stop a child workspace that reaches one before any hook or config
// write runs; this catches the per-repository targets inside a workspace,
// such as a child repositories entry or a forge repository or orphan whose
// directory is the devbox. A target is resolved the way the executor
// resolves it.
func keepActionsOffEnclosingConfigDirs(actions []reposync.Action, enclosing []string, workspace string, out io.Writer) []reposync.Action {
	kept := actions[:0]
	for _, action := range actions {
		target, err := filepath.Abs(action.Repo.TargetPath)
		if err != nil {
			target = action.Repo.TargetPath
		}
		if reachesEnclosingConfigDir(target, enclosing) {
			fmt.Fprintf(out, "⚠️  Skipping '%s' in workspace '%s': path '%s' reaches an enclosing config directory\n", action.Repo.Name, workspace, action.Repo.TargetPath)
			continue
		}
		kept = append(kept, action)
	}
	return kept
}
