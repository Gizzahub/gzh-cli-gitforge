//go:build linux

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// legacyBirthTime reads the creation time through statx. The inode it reports
// must match the one Lstat saw, or the path was replaced between the two calls.
// A kernel without statx (before 4.11) or a sandbox that blocks it reports no
// birth time, like a filesystem that records none.
func legacyBirthTime(path string, inode uint64) (int64, error) {
	var stx unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_INO|unix.STATX_BTIME, &stx)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read physical resource birth time: %w", err)
	}
	if stx.Ino != inode {
		return 0, fmt.Errorf("physical resource changed while it was inspected: %s", path)
	}
	if stx.Mask&unix.STATX_BTIME == 0 {
		return 0, nil
	}
	return stx.Btime.Sec*1_000_000_000 + int64(stx.Btime.Nsec), nil
}
