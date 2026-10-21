//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import "golang.org/x/sys/unix"

// processAlive reports whether pid names a live process; EPERM means it
// exists under another user.
func processAlive(pid int) (bool, error) {
	err := unix.Kill(pid, 0)
	if err == nil || err == unix.EPERM {
		return true, nil
	}
	if err == unix.ESRCH {
		return false, nil
	}
	return false, err
}
