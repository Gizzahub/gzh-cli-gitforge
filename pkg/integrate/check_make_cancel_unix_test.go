//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// ISSUE-82: canceling Check while the baseline make probe runs must still
// unregister the baseline worktree; cleanup cannot reuse the canceled context.
func TestCheckCancelDuringBaselineUnregistersBaselineWorktree(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marker := filepath.Join(t.TempDir(), "baseline-started")
	fx := makeTimeoutFixture(t, "@: > '"+marker+"'; sleep 30", "@exit 1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Check(ctx, gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
		done <- err
	}()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("Check returned before the baseline probe started: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("baseline probe never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Check did not return after cancellation")
	}
	cmd := exec.CommandContext(t.Context(), "git", "worktree", "list", "--porcelain")
	cmd.Dir = fx.Worktree
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "gz-git-integrate-base-") {
		t.Fatalf("baseline worktree still registered after cancellation:\n%s", out)
	}
}
