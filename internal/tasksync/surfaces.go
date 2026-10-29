// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// safeCommonSurfaces refuses fixed Git metadata write surfaces that contain a
// symlink. It scans neither repository content nor arbitrary Git metadata.
func safeCommonSurfaces(commonDir, handle string) error {
	for _, path := range []string{
		"objects", "objects/pack", "objects/info", "worktrees",
		"refs", "refs/gz-git", "refs/gz-git/tasksync",
		"logs", "logs/refs", "logs/refs/gz-git", "logs/refs/gz-git/tasksync",
	} {
		if err := safeExistingPath(commonDir, path, true); err != nil {
			return err
		}
	}
	for _, path := range []string{"config", "packed-refs"} {
		if err := safeExistingPath(commonDir, path, false); err != nil {
			return err
		}
	}
	if err := safeObjectFanout(commonDir); err != nil {
		return err
	}
	if handle == "" {
		return nil
	}
	for _, path := range []string{"refs/gz-git/tasksync/" + handle, "logs/refs/gz-git/tasksync/" + handle} {
		if err := safeExistingPath(commonDir, path, false); err != nil {
			return err
		}
	}
	return nil
}

func safeObjectFanout(commonDir string) error {
	entries, err := os.ReadDir(filepath.Join(commonDir, "objects"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect object directory: %w", err)
	}
	for _, entry := range entries {
		if len(entry.Name()) == 2 && hexOnly(entry.Name()) {
			if err := safeExistingPath(commonDir, "objects/"+entry.Name(), true); err != nil {
				return err
			}
		}
	}
	return nil
}

func safeCheckpointSurface(commonDir, handle string) error {
	for _, path := range []string{"refs", "refs/gz-git", "refs/gz-git/tasksync-checkpoints", "logs", "logs/refs", "logs/refs/gz-git", "logs/refs/gz-git/tasksync-checkpoints", "refs/gz-git/tasksync-checkpoints/" + handle, "logs/refs/gz-git/tasksync-checkpoints/" + handle} {
		if err := safeExistingPath(commonDir, path, !strings.HasSuffix(path, "/"+handle)); err != nil {
			return err
		}
	}
	return nil
}

func safeExistingPath(base, relative string, requireDirectory bool) error {
	components := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	current := base
	for index, component := range components {
		if component == "." || component == ".." || component == "" {
			return fmt.Errorf("invalid repository metadata path")
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect repository metadata: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("repository metadata symlink is unsupported")
		}
		if (index < len(components)-1 || requireDirectory) && !info.IsDir() {
			return fmt.Errorf("repository metadata path is not a directory")
		}
		if index == len(components)-1 && !requireDirectory && !info.Mode().IsRegular() {
			return fmt.Errorf("repository metadata path is not a regular file")
		}
	}
	return nil
}
