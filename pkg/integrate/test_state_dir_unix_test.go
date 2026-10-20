//go:build darwin || linux

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ISSUE-84: a re-executed helper child ends in os.Exit, so a state dir it
// created in TestMain would never be removed. The child must create none.
func TestHelperChildLeavesNoStateDir(t *testing.T) {
	tmp := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "binding.pid")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestReadinessNestedGroupHelper$")
	child.Env = append(os.Environ(), "TMPDIR="+tmp, readinessNestedHelperEnv+"=1", "READINESS_NESTED_PID="+pidFile)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	pid := waitForReadinessPID(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
	if err := child.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatalf("helper child: %v", err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "gz-git-integrate-state-") {
			t.Fatalf("helper child left %s in its TMPDIR", entry.Name())
		}
	}
}

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
