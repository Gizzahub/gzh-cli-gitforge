//go:build darwin || linux

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A lifecycle script that leaves a child holding stdout must not keep the
// install running past its budget: the group kill and WaitDelay return it.
func TestRunPnpmFrozenInstallTimeoutKillsDescendants(t *testing.T) {
	dir := t.TempDir()
	bin := fakeCargo(t, "sleep 60 &\nsleep 60")
	if err := os.Rename(filepath.Join(bin, "cargo"), filepath.Join(bin, "pnpm")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	started := time.Now()
	err := runPnpmFrozenInstall(context.Background(), dir, time.Second)
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second+pnpmPrepareWaitDelay {
		t.Fatalf("install outlived its bound: %s", elapsed)
	}
}

func TestRunPnpmFrozenInstallReportsCancelNotTimeout(t *testing.T) {
	dir := t.TempDir()
	bin := fakeCargo(t, "sleep 60")
	if err := os.Rename(filepath.Join(bin, "cargo"), filepath.Join(bin, "pnpm")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := runPnpmFrozenInstall(ctx, dir, time.Minute)
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("err = %v, want a cancellation", err)
	}
}
