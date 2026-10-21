//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLockFile takes an exclusive flock without blocking. The lock belongs to
// the open file description, so the kernel drops it when the holder closes
// the file or dies; a second open in the same process conflicts like another
// process would. A signal that interrupts the call says nothing about the
// holder, so EINTR is retried; the retry never waits for a held lock, which
// still refuses with EWOULDBLOCK.
func tryLockFile(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) // #nosec G115 -- a file descriptor fits in int
	for errors.Is(err, unix.EINTR) {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) // #nosec G115 -- a file descriptor fits in int
	}
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, unix.EWOULDBLOCK):
		return false, nil
	default:
		return false, err
	}
}
