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

// keepActionsOffConfigDir drops the actions a child config workspace plans
// for the config directory that recursed into it. The child's planners
// compare its entries only with the child directory, so a child entry such
// as `path: ..` would otherwise plan the devbox itself (ISSUE-85). A target
// is resolved the way the executor resolves it.
func keepActionsOffConfigDir(actions []reposync.Action, configDir, workspace string, out io.Writer) []reposync.Action {
	kept := actions[:0]
	for _, action := range actions {
		target, err := filepath.Abs(action.Repo.TargetPath)
		if err != nil {
			target = action.Repo.TargetPath
		}
		if pointsAtConfigDir(target, configDir) {
			fmt.Fprintf(out, "⚠️  Skipping '%s' in workspace '%s': path '%s' points to config directory\n", action.Repo.Name, workspace, action.Repo.TargetPath)
			continue
		}
		kept = append(kept, action)
	}
	return kept
}
