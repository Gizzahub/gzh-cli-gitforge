//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import "os"

func syncStateDirectory(path string) error {
	dir, err := os.Open(path) // #nosec G304 -- path is the parent of a local state file
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
