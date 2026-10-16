//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

func TestIntegrateResourceWaitExpiryExitsTwo(t *testing.T) {
	t.Setenv("HOST_SLOTS_HELD", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	marker := filepath.Join(t.TempDir(), "gate-ran")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n  lockWait: 50ms\n  taskPattern: dev/*\n")
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@touch '"+marker+"'\nlint:\n\t@touch '"+marker+"'\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "expiry fixture")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	before := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop")
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
	if err := os.WriteFile(filepath.Join(dir, "integrate.lock.json"), []byte(`{"pid":1,"repository":"/fixture-holder/integration branch --target not ready","branch":"dev/holder"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(fx.Worktree)
	t.Run("check", func(t *testing.T) {
		restore := setIntegrateCheckGlobals(t)
		defer restore()
		wait := integrateCheckLockWait
		integrateCheckLockWait = 0
		defer func() { integrateCheckLockWait = wait }()
		var out bytes.Buffer
		oldOut, oldErr := integrateCheckCmd.OutOrStdout(), integrateCheckCmd.ErrOrStderr()
		integrateCheckCmd.SetOut(&out)
		integrateCheckCmd.SetErr(&out)
		defer func() { integrateCheckCmd.SetOut(oldOut); integrateCheckCmd.SetErr(oldErr) }()
		err := runIntegrateCheck(integrateCheckCmd, nil)
		if cliutil.ExitCodeForError(err) != 2 || !errors.Is(err, integrate.ErrHostLockWait) {
			t.Fatalf("exit=%d err=%v", cliutil.ExitCodeForError(err), err)
		}
		for _, want := range []string{"50ms", "repository config", "per acquisition", "/fixture-holder/integration branch --target not ready", "no gate measured", "resource wait"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("missing %q in report: %s", want, out.String())
			}
		}
	})
	t.Run("run JSON", func(t *testing.T) {
		restore := setIntegrateRunGlobals(t)
		defer restore()
		wait, asJSON := integrateRunLockWait, integrateRunJSON
		integrateRunLockWait, integrateRunJSON = time.Duration(0), true
		defer func() { integrateRunLockWait, integrateRunJSON = wait, asJSON }()
		var out, stderr bytes.Buffer
		oldOut, oldErr := integrateRunCmd.OutOrStdout(), integrateRunCmd.ErrOrStderr()
		integrateRunCmd.SetOut(&out)
		integrateRunCmd.SetErr(&stderr)
		defer func() { integrateRunCmd.SetOut(oldOut); integrateRunCmd.SetErr(oldErr) }()
		err := runIntegrateRun(integrateRunCmd, nil)
		if cliutil.ExitCodeForError(err) != 2 || !errors.Is(err, integrate.ErrHostLockWait) {
			t.Fatalf("exit=%d err=%v", cliutil.ExitCodeForError(err), err)
		}
		var p integrate.RunProtocol
		if err := json.Unmarshal(out.Bytes(), &p); err != nil {
			t.Fatalf("JSON %s: %v", out.String(), err)
		}
		if p.SchemaVersion != 1 || p.ExitCode != 2 || p.Integrated || p.SourcePushed || p.WorktreeRemoved || p.LocalBranchRemoved || p.RemoteBranchRemoved || p.ResourceWaitDurationMS <= 0 || p.ValidationDurationMS != 0 {
			t.Fatalf("expiry protocol: %+v", p)
		}
		if !strings.Contains(p.Error, "host measurement lock wait expired") || strings.Contains(p.Error, "pre-mutation outcome is not bound") || !strings.Contains(stderr.String(), "/fixture-holder/integration branch --target not ready") || !strings.Contains(stderr.String(), "no gate measured") {
			t.Fatalf("diagnostics: %+v stderr=%s", p, stderr.String())
		}
	})
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gate ran: %v", err)
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != before {
		t.Fatalf("target moved: %s -> %s", before, got)
	}
	if _, err := os.Stat(fx.Worktree); err != nil {
		t.Fatalf("task worktree changed: %v", err)
	}
}
