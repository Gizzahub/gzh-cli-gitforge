//go:build windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"

	"golang.org/x/sys/windows"
)

// processAlive reports whether pid names a process that has not exited.
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
