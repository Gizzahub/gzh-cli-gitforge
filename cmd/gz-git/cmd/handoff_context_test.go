// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// resetHandoffFlagState snapshots every global the three handoff commands read
// and restores it when the test finishes, so one subtest's flags cannot leak
// into the next test in the package.
func resetHandoffFlagState(t *testing.T) {
	t.Helper()
	prevQuiet, prevVerbose := quiet, verbose
	prevCheck, prevStart, prevEnd := handoffCheckFlags, handoffStartFlags, handoffEndFlags
	prevMessage, prevForce := handoffEndMessage, handoffEndForce
	prevNoPush, prevNoTrailers := handoffEndNoPush, handoffEndNoTrailers
	t.Cleanup(func() {
		quiet, verbose = prevQuiet, prevVerbose
		handoffCheckFlags, handoffStartFlags, handoffEndFlags = prevCheck, prevStart, prevEnd
		handoffEndMessage, handoffEndForce = prevMessage, prevForce
		handoffEndNoPush, handoffEndNoTrailers = prevNoPush, prevNoTrailers
	})

	quiet = false
	verbose = false
	handoffCheckFlags = BulkCommandFlags{Depth: 1, Parallel: 2, Format: "default"}
	handoffStartFlags = BulkCommandFlags{Depth: 1, Parallel: 2, Format: "default"}
	handoffEndFlags = BulkCommandFlags{Depth: 1, Parallel: 2, Format: "default"}
	handoffEndMessage = defaultCheckpointMessage
	handoffEndForce = false
	handoffEndNoPush = false
	handoffEndNoTrailers = false
}

// isolateHandoffConfig points the config loader at an empty home, so the
// effective config the commands resolve comes from defaults rather than from
// whatever the developer running the tests has configured.
func isolateHandoffConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

// injectCancelledContext points cmd at an already-canceled context, the state
// a command is in when the process was asked to stop before it began, and
// restores the previous command context when the test finishes. No signal is
// ever raised: the canceled context is the thing the commands must honor.
func injectCancelledContext(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	prev := cmd.Context()
	t.Cleanup(func() { cmd.SetContext(prev) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cancel()
	cmd.SetContext(ctx)
}

// gitRevParse returns the resolved object name of ref in dir.
func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("git -C %s rev-parse %s failed: %v", dir, ref, err)
	}
	return strings.TrimSpace(string(out))
}

// gitStatusPorcelain returns the machine-readable worktree state of dir.
func gitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git -C %s status --porcelain failed: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// requireCancellation asserts the command surfaced the canceled context as the
// exit-2 "could not run" failure rather than as a verdict, a partial report, or
// nil success.
func requireCancellation(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("command returned nil with a canceled context; want a cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v; want it to wrap context.Canceled", err)
	}
	var exitErr *cliutil.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v; want a *cliutil.ExitError", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("exit code = %d; want 2 (the operation itself could not run)", exitErr.Code)
	}
}

// TestHandoffCommandCancellation proves a canceled command context reaches
// each handoff command before it can touch a repository: check reports the
// cancellation instead of a verdict, start rebases nothing, and end commits
// nothing and pushes nothing — not even a checkpoint.
func TestHandoffCommandCancellation(t *testing.T) {
	t.Run("check reports the cancellation instead of a verdict", func(t *testing.T) {
		isolateHandoffConfig(t)
		resetHandoffFlagState(t)
		parent := setupBulkParent(t)
		injectCancelledContext(t, handoffCheckCmd)

		var err error
		captureStdout(t, func() {
			err = runHandoffCheck(handoffCheckCmd, []string{parent})
		})
		requireCancellation(t, err)
	})

	t.Run("start leaves the clone and the bare origin untouched", func(t *testing.T) {
		isolateHandoffConfig(t)
		resetHandoffFlagState(t)
		fx := testutil.TempWorktreeWithBareOrigin(t)

		// Give the remote a commit the clone has not pulled, from the linked
		// worktree: the clone is then clean and behind, exactly the state an
		// arrival is supposed to act on.
		readme := filepath.Join(fx.Worktree, "README.md")
		if err := os.WriteFile(readme, []byte("advanced on the remote\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, fx.Worktree, "add", ".")
		runGit(t, fx.Worktree, "commit", "-m", "advance")
		runGit(t, fx.Worktree, "push", "origin", "HEAD:main")

		beforeHead := gitRevParse(t, fx.Clone, "HEAD")
		beforeOrigin := gitRevParse(t, fx.Origin, "main")
		injectCancelledContext(t, handoffStartCmd)

		var err error
		captureStdout(t, func() {
			err = runHandoffStart(handoffStartCmd, []string{fx.Clone})
		})
		requireCancellation(t, err)

		if after := gitRevParse(t, fx.Clone, "HEAD"); after != beforeHead {
			t.Errorf("clone HEAD moved %s -> %s; the canceled arrival must not rebase", beforeHead, after)
		}
		if status := gitStatusPorcelain(t, fx.Clone); status != "" {
			t.Errorf("clone checkout changed: %q; the canceled arrival must not touch it", status)
		}
		if after := gitRevParse(t, fx.Origin, "main"); after != beforeOrigin {
			t.Errorf("bare origin main moved %s -> %s; start never pushes", beforeOrigin, after)
		}
	})

	t.Run("end leaves the dirty repository and the remote untouched", func(t *testing.T) {
		isolateHandoffConfig(t)
		resetHandoffFlagState(t)
		fx := testutil.TempWorktreeWithBareOrigin(t)

		// Make the clone dirty: uncommitted changes to a tracked file are
		// exactly the work a departure exists to checkpoint and push.
		readme := filepath.Join(fx.Clone, "README.md")
		if err := os.WriteFile(readme, []byte("work in progress\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		beforeHead := gitRevParse(t, fx.Clone, "HEAD")
		beforeStatus := gitStatusPorcelain(t, fx.Clone)
		beforeContent, err := os.ReadFile(readme)
		if err != nil {
			t.Fatal(err)
		}
		beforeOrigin := gitRevParse(t, fx.Origin, "main")
		injectCancelledContext(t, handoffEndCmd)

		captureStdout(t, func() {
			err = runHandoffEnd(handoffEndCmd, []string{fx.Clone})
		})
		requireCancellation(t, err)

		if after := gitRevParse(t, fx.Clone, "HEAD"); after != beforeHead {
			t.Errorf("HEAD moved %s -> %s; the canceled departure must not create a checkpoint", beforeHead, after)
		}
		if after := gitStatusPorcelain(t, fx.Clone); after != beforeStatus {
			t.Errorf("worktree state changed %q -> %q; the canceled departure must not stage or revert", beforeStatus, after)
		}
		afterContent, err := os.ReadFile(readme)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(afterContent, beforeContent) {
			t.Error("README.md was modified; the canceled departure must not touch the files")
		}
		if after := gitRevParse(t, fx.Origin, "main"); after != beforeOrigin {
			t.Errorf("bare origin main moved %s -> %s; the canceled departure must not push", beforeOrigin, after)
		}
	})
}
