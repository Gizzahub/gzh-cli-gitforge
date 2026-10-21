//go:build darwin || linux

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// ISSUE-84: a suite that is killed, or panics on -timeout, never removes its
// state dir. The next suite reclaims a dir whose owner has exited and keeps
// every dir it cannot prove dead.
func TestReclaimDeadTestStateDirs(t *testing.T) {
	exited := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit 0")
	if err := exited.Run(); err != nil {
		t.Fatal(err)
	}
	deadPID := exited.Process.Pid
	tmp := t.TempDir()
	names := map[string]bool{ // name -> want reclaimed
		testStatePrefix + strconv.Itoa(deadPID) + "-1":      true,
		testStatePrefix + strconv.Itoa(os.Getppid()) + "-2": false,
		testStatePrefix + strconv.Itoa(os.Getpid()) + "-3":  false,
		testStatePrefix + "1190862848":                      false,
		testStatePrefix + "x-4":                             false,
		strconv.Itoa(deadPID) + "-5":                        false,
	}
	for name := range names {
		if err := os.Mkdir(filepath.Join(tmp, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	file := testStatePrefix + strconv.Itoa(deadPID) + "-file"
	if err := os.WriteFile(filepath.Join(tmp, file), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	names[file] = false

	removed := reclaimDeadTestStateDirs(tmp)
	for name, wantGone := range names {
		_, err := os.Stat(filepath.Join(tmp, name))
		if gone := os.IsNotExist(err); gone != wantGone {
			t.Errorf("%s: reclaimed=%t, want %t", name, gone, wantGone)
		}
	}
	if len(removed) != 1 {
		t.Errorf("reclaimDeadTestStateDirs() = %v, want one entry", removed)
	}
}
