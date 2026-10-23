//go:build darwin || freebsd || netbsd

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// legacyBirthTime reads the creation time BSD stat records. The inode it
// reports must match the one Lstat saw, or the path was replaced in between.
func legacyBirthTime(path string, inode uint64) (int64, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, fmt.Errorf("read physical resource birth time: %w", err)
	}
	if uint64(st.Ino) != inode { //nolint:unconvert // Ino is narrower than uint64 on some BSDs.
		return 0, fmt.Errorf("physical resource changed while it was inspected: %s", path)
	}
	return st.Btim.Nano(), nil
}
