// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// TestLightGateDoesNotWaitForHighLoadSlot proves the opt-in boundary: a
// confirmed heavy gate occupies capacity, while a separate unclassified
// repository still completes its mandatory gate.
func TestLightGateDoesNotWaitForHighLoadSlot(t *testing.T) {
	fastHostLockPolling(t)
	state := t.TempDir()
	started := filepath.Join(state, "heavy-started")
	release := filepath.Join(state, "release-heavy")
	heavy := measurementLockFixture(t, "touch "+strconv.Quote(started)+"; while [ ! -f "+strconv.Quote(release)+" ]; do sleep 0.05; done", "1m")
	light := readyTaskFixture(t)

	heavyDone := make(chan checkResult, 1)
	go func() {
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: heavy.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
		heavyDone <- checkResult{report: report, err: err}
	}()
	t.Cleanup(func() {
		_ = os.WriteFile(release, []byte("release\n"), 0o600)
		select {
		case <-heavyDone:
		case <-time.After(10 * time.Second):
		}
	})
	waitForText(t, "heavy gate", func() string {
		if _, err := os.Stat(started); err == nil {
			return "started"
		}
		return ""
	}, "started")

	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{RepoPath: light.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard}})
	if err != nil {
		t.Fatalf("light Run: %v", err)
	}
	if !report.Integrated {
		t.Fatalf("light Run did not integrate:\n%s", FormatRun(report))
	}
	select {
	case result := <-heavyDone:
		t.Fatalf("heavy Check completed before light integration: %#v, %v", result.report, result.err)
	default:
	}
	if err := os.WriteFile(release, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-heavyDone:
		if result.err != nil || !result.report.Ready {
			t.Fatalf("heavy Check = %#v, %v", result.report, result.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("heavy Check did not finish")
	}
}

func TestTargetLockBlocksSameDestinationBeforeFetchOrGate(t *testing.T) {
	fastHostLockPolling(t)
	fx := readyTaskFixture(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	lock, err := acquireTargetLock(context.Background(), g, "refs/heads/develop", io.Discard)
	if err != nil {
		t.Fatalf("acquire target lock: %v", err)
	}
	t.Cleanup(lock.release)

	// Preflight reads repository configuration before it reaches the target
	// lock. Give that local setup room under a race run; the short polling
	// interval above still makes the locked acquisition observable promptly.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = Check(ctx, gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "wait for target lock") {
		t.Fatalf("Check error = %v, want target lock wait", err)
	}
}
