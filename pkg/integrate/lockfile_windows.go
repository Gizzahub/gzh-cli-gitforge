//go:build windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockByteOffsetHigh places the locked byte at offset 1<<32, far past the
// holder record at offset 0. LockFileEx locks are mandatory, so locking the
// record itself would make another process's read of it fail with
// ERROR_LOCK_VIOLATION and every refusal name an unidentified holder.
// Windows allows locking a range past the end of the file.
const lockByteOffsetHigh = 1

// tryLockFile takes an exclusive LockFileEx lock on one byte past the holder
// record without blocking. Windows releases it when the handle closes or the
// process exits.
func tryLockFile(f *os.File) (bool, error) {
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{OffsetHigh: lockByteOffsetHigh})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION), errors.Is(err, windows.ERROR_IO_PENDING):
		return false, nil
	default:
		return false, err
	}
}
