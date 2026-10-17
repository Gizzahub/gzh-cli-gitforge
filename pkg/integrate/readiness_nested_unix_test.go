//go:build darwin || linux

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// A board gate owns a separate binding process group. The outer runner must
// allow its TERM handler to cancel that group before killing the runner.
func TestExecuteReadinessCancelLetsGateCleanNestedGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "binding.pid")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("READINESS_NESTED_HELPER", "1")
	t.Setenv("READINESS_NESTED_PID", pidFile)
	t.Setenv("READINESS_TEST_BINARY", binary)
	runner := filepath.Join(dir, "runner")
	if err := os.WriteFile(runner, []byte("#!/bin/sh\nexec \"$READINESS_TEST_BINARY\" -test.run=^TestReadinessNestedGroupHelper$\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := executeReadinessWithTimeout(ctx, runner, dir, dir, "a", "b", time.Minute)
		done <- err
	}()
	pid := waitForReadinessPID(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel result = %v", err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("nested binding %d survived runner cancellation: %v", pid, err)
	}
}

func TestReadinessNestedGroupHelper(t *testing.T) {
	if os.Getenv("READINESS_NESTED_HELPER") != "1" {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	cmd := exec.CommandContext(context.Background(), "sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("READINESS_NESTED_PID"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		os.Exit(3)
	}
	<-ctx.Done()
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Wait()
	stop()
	os.Exit(0)
}

func TestExecuteReadinessEscalatesIgnoredTERM(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "stubborn.pid")
	t.Setenv("READINESS_STUBBORN_PID", pidFile)
	runner := filepath.Join(dir, "runner")
	if err := os.WriteFile(runner, []byte("#!/bin/sh\ntrap '' TERM\nprintf '%s' \"$$\" > \"$READINESS_STUBBORN_PID\"\nwhile :; do sleep 1; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := executeReadinessWithTimeout(ctx, runner, dir, dir, "a", "b", time.Minute)
		done <- err
	}()
	pid := waitForReadinessPID(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
	started := time.Now()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel result = %v", err)
	}
	if elapsed := time.Since(started); elapsed > readinessTerminationGrace+readinessWaitDelay+time.Second {
		t.Fatalf("escalation took %s", elapsed)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("TERM-ignoring runner %d survived: %v", pid, err)
	}
}
