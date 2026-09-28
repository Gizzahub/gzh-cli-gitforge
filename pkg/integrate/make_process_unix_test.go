//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
)

func TestConfigureMakeProcessCreatesProcessGroup(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "sleep", "30")
	processTree, err := newMakeProcessTree()
	if err != nil {
		t.Fatalf("create process tree: %v", err)
	}
	defer processTree.close()
	if err := processTree.configure(cmd); err != nil {
		t.Fatalf("configure process tree: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer func() {
		_ = processTree.cancel()
		_ = cmd.Wait()
	}()
	if err := processTree.attach(cmd.Process); err != nil {
		t.Fatalf("attach process: %v", err)
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("get process group: %v", err)
	}
	if pgid != cmd.Process.Pid {
		t.Fatalf("process group = %d, want leader PID %d", pgid, cmd.Process.Pid)
	}
}
