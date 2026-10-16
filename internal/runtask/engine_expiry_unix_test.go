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
	git(fx.Clone, "branch", "develop")
	git(fx.Clone, "push", "-u", fx.Remote, "develop")
	git(fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	marker := filepath.Join(t.TempDir(), "gate-ran")
	for name, content := range map[string]string{
		".gz-git.yaml": "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n  lockWait: 200ms\n",
		"Makefile":     "check:\n\t@touch '" + marker + "'\nlint:\n\t@touch '" + marker + "'\n",
	} {
		if err := os.WriteFile(filepath.Join(fx.Worktree, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
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
	if err != nil || check.ExitCode != 2 || !strings.Contains(check.Stdout, "resource slot") || !strings.Contains(check.Stdout, "no gate measured") {
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
