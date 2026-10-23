//go:build !windows

package gitcmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeGitLeavingChild writes a git stand-in that prints a line, leaves a
// background child holding stdout open, and then either exits 0 or blocks.
func fakeGitLeavingChild(t *testing.T, block bool) string {
	t.Helper()
	tail := "exit 0\n"
	if block {
		tail = "sleep 30\n"
	}
	script := "#!/bin/sh\necho done\nsleep 30 &\n" + tail
	path := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { // #nosec G306 -- test binary must be executable
		t.Fatal(err)
	}
	return path
}

func TestExecutorRunSucceedsWhenAChildKeepsThePipeOpen(t *testing.T) {
	e := NewExecutor(WithGitBinary(fakeGitLeavingChild(t, false)))
	started := time.Now()
	res, err := e.Run(context.Background(), t.TempDir(), "status")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.Error != nil || strings.TrimSpace(res.Stdout) != "done" {
		t.Fatalf("exit=%d err=%v stdout=%q", res.ExitCode, res.Error, res.Stdout)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("Run waited %s for the lingering child", elapsed)
	}
}

func TestExecutorRunReturnsAfterCancelWhenAChildKeepsThePipeOpen(t *testing.T) {
	e := NewExecutor(WithGitBinary(fakeGitLeavingChild(t, true)))
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	res, err := e.Run(ctx, t.TempDir(), "status")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode == 0 || res.Error == nil {
		t.Fatalf("canceled run reported success: exit=%d", res.ExitCode)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("Run waited %s after cancel", elapsed)
	}
}
