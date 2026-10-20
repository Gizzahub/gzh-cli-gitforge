//go:build darwin || linux

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
