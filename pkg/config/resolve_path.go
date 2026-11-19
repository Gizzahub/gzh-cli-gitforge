// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath resolves a configured path against the directory of the config
// that declares it. It is the one base-directory rule shared by the config
// loaders, `workspace sync` and the status loaders (ISSUE-391):
//
//   - "~/foo"  resolves under the user's home directory;
//   - an absolute path is returned unchanged;
//   - any other path, the empty string included, is joined onto baseDir.
//
// Only the "~/" form is expanded: a bare "~" or "~user" is an ordinary
// relative path. An error is returned only when the home directory cannot be
// determined for a "~/" path.
func ResolvePath(baseDir, path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(baseDir, path), nil
}
