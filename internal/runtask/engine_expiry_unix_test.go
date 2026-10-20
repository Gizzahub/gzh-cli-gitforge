//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package runtask

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestEngineBridgeResourceExpiryRetainsReport(t *testing.T) {
	t.Setenv("HOST_SLOTS_HELD", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	fx := testutil.TempWorktreeWithBareOrigin(t)
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	// Only a high-load readiness contract takes the host measurement lock;
	// gz-git runs no repository code without one.
	git(fx.Clone, "checkout", "-b", "develop")
	marker := filepath.Join(t.TempDir(), "gate-ran")
	if err := os.MkdirAll(filepath.Join(fx.Clone, ".gz-git", "readiness"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		".gz-git.yaml":            "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n  lockWait: 200ms\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n",
		".gz-git/readiness/check": "#!/bin/sh\ntouch '" + marker + "'\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ok\"}'\n",
	} {
		if err := os.WriteFile(filepath.Join(fx.Clone, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(fx.Clone, ".gz-git", "readiness", "check"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(fx.Clone, "add", ".")
	git(fx.Clone, "commit", "-m", "declare high-load readiness")
	git(fx.Clone, "push", "-u", fx.Remote, "develop")
	git(fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	if err := os.WriteFile(filepath.Join(fx.Worktree, "task.txt"), []byte("task\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(fx.Worktree, "add", ".")
	git(fx.Worktree, "commit", "-m", "resource expiry fixture")
	git(fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	dir := filepath.Join(os.Getenv("XDG_STATE_HOME"), "host-slots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "integrate.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "integrate.lock.json"), []byte(`{"pid":1,"repository":"/integration branch --target not ready","branch":"dev/holder"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bridge := engineIntegrate{}
	check, err := bridge.Check(context.Background(), fx.Worktree, false)
	if err != nil || check.ExitCode != 2 || !strings.Contains(check.Stdout, "host measurement lock wait expired") || !strings.Contains(check.Stdout, "no gate measured") {
		t.Fatalf("check=%+v err=%v", check, err)
	}
	run, err := bridge.Run(context.Background(), fx.Worktree, false)
	if err != nil || run.ExitCode != 2 {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("gate ran: %v", err)
	}
}
