// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import "os"

// pointsAtConfigDir reports whether a resolved workspace path is the config
// directory itself, so sync never plans the devbox that holds the config.
// The string comparison catches the plain spellings; file identity catches
// a symlink or relative spelling of the same directory (ISSUE-80). A path
// that does not exist yet cannot be the config directory.
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
