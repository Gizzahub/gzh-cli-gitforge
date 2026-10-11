//go:build windows

package integrate

import (
	"errors"

	"golang.org/x/sys/windows"
)

func processAlive(pid int) (bool, error) {
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return false, nil
		}
		return false, err
	}
	defer windows.CloseHandle(p)
	var exitCode uint32
	if err := windows.GetExitCodeProcess(p, &exitCode); err != nil {
		return false, err
	}
	return exitCode == 259, nil // STILL_ACTIVE
}

func unlockFile(f interface{ Fd() uintptr }) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}

// Windows has no supported directory fsync. Rename is still atomic locally;
// the file itself is synced before replacement above.
func syncAdmissionDirectory(string) error { return nil }
