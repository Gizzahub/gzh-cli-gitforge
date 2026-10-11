//go:build !windows

package integrate

import (
	"os"

	"golang.org/x/sys/unix"
)

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
func unlockFile(f interface{ Fd() uintptr }) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }

func syncAdmissionDirectory(path string) error {
	dir, err := os.Open(path) // #nosec G304 -- path is the fixed local admission-state parent
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
