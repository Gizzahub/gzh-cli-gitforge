// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:noctx // test helper
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// findCommand returns the descendant command reached by walking path names.
func findCommand(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	cur := root
	for _, name := range path {
		var next *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == name {
				next = c
				break
			}
		}
		if next == nil {
			t.Fatalf("command %q not found under %q", name, cur.CommandPath())
		}
		cur = next
	}
	return cur
}

// TestErrPartialFailure verifies the bulk exit-code helper: no failures => nil,
// any failure => an *ExitError carrying code 2.
func TestErrPartialFailure(t *testing.T) {
	if err := errPartialFailure(0, 5); err != nil {
		t.Errorf("errPartialFailure(0, 5) = %v, want nil", err)
	}

	err := errPartialFailure(3, 5)
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitPartialFailed {
		t.Errorf("exit code for 3/5 failed = %d, want %d", got, cliutil.ExitPartialFailed)
	}
}

// TestBulkCommandsHaveExitCodesHelp asserts every bulk command (and conflict
// detect) documents its exit-code contract in --help.
func TestBulkCommandsHaveExitCodesHelp(t *testing.T) {
	bulk := []string{
		"clone", "update", "pull", "fetch", "push", "status",
		"commit", "switch", "stash", "tag", "clean", "diff",
	}
	for _, name := range bulk {
		c := findCommand(t, rootCmd, name)
		if !strings.Contains(c.Long, "Exit Codes:") {
			t.Errorf("command %q help is missing an 'Exit Codes:' section", name)
		}
	}

	detect := findCommand(t, rootCmd, "conflict", "detect")
	if !strings.Contains(detect.Long, "Exit Codes:") {
		t.Errorf("'conflict detect' help is missing an 'Exit Codes:' section")
	}
}

// TestPullBadFormatExitsOne verifies an invalid --format value is a tool error
// (exit 1), not a partial-failure (exit 2).
func TestPullBadFormatExitsOne(t *testing.T) {
	restore := setBulkTestGlobals(t)
	defer restore()

	pullFlags.Format = "bogus-format"

	err := runPull(pullCmd, []string{t.TempDir()})
	if err == nil {
		t.Fatalf("runPull with bad format returned nil, want error")
	}
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitToolError {
		t.Errorf("exit code = %d, want %d (tool error)", got, cliutil.ExitToolError)
	}
}

// TestPullAllReposFailExitsTwo clones a source repo twice, breaks each clone's
// remote URL, and asserts a bulk pull where every repo fails exits with code 2.
func TestPullAllReposFailExitsTwo(t *testing.T) {
	restore := setBulkTestGlobals(t)
	defer restore()

	source := testutil.TempGitRepoWithCommit(t)
	parent := t.TempDir()
	bogus := filepath.Join(t.TempDir(), "does-not-exist.git")

	for _, name := range []string{"repo1", "repo2"} {
		dest := filepath.Join(parent, name)
		runGit(t, ".", "clone", "--quiet", source, dest)
		// Break the remote so the actual pull fails (upstream config stays intact).
		runGit(t, dest, "remote", "set-url", "origin", bogus)
	}

	pullFlags.Depth = 1

	// Swallow any stray stdout from the command.
	_ = captureStdout(t, func() {
		if err := runPull(pullCmd, []string{parent}); err != nil {
			if got := cliutil.ExitCodeForError(err); got != cliutil.ExitPartialFailed {
				t.Errorf("exit code = %d, want %d (partial failure)", got, cliutil.ExitPartialFailed)
			}
		} else {
			t.Errorf("runPull returned nil, want partial-failure error")
		}
	})
}

// TestConflictDetectExitCodes covers the grep-style contract: 0 clean, 1 found,
// 2 execution error.
func TestConflictDetectExitCodes(t *testing.T) {
	origQuiet := quiet
	quiet = true
	t.Cleanup(func() { quiet = origQuiet })

	t.Run("no conflict exits 0", func(t *testing.T) {
		dir := newTwoBranchRepo(t, false)
		t.Chdir(dir)
		if err := runConflictDetect(detectCmd, []string{"source", "target"}); err != nil {
			t.Errorf("expected nil (exit 0), got %v (exit %d)", err, cliutil.ExitCodeForError(err))
		}
	})

	t.Run("conflict found exits 1", func(t *testing.T) {
		dir := newTwoBranchRepo(t, true)
		t.Chdir(dir)
		err := runConflictDetect(detectCmd, []string{"source", "target"})
		if got := cliutil.ExitCodeForError(err); got != 1 {
			t.Errorf("exit code = %d, want 1 (conflict found); err=%v", got, err)
		}
	})

	t.Run("execution error exits 2", func(t *testing.T) {
		t.Chdir(t.TempDir()) // not a git repository
		err := runConflictDetect(detectCmd, []string{"source", "target"})
		if got := cliutil.ExitCodeForError(err); got != 2 {
			t.Errorf("exit code = %d, want 2 (execution error); err=%v", got, err)
		}
	})
}

// newTwoBranchRepo builds a repo with `source` and `target` branches diverged
// from a common base. When conflicting is true both branches modify the same
// file (a real conflict); otherwise only source adds a new file (clean merge).
func newTwoBranchRepo(t *testing.T, conflicting bool) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "base.txt", "base\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "base")

	runGit(t, dir, "branch", "target")
	runGit(t, dir, "branch", "source")

	runGit(t, dir, "checkout", "--quiet", "source")
	if conflicting {
		writeFile(t, dir, "base.txt", "source change\n")
	} else {
		writeFile(t, dir, "new.txt", "new\n")
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "source change")

	if conflicting {
		runGit(t, dir, "checkout", "--quiet", "target")
		writeFile(t, dir, "base.txt", "target change\n")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "--quiet", "-m", "target change")
	}

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// setBulkTestGlobals resets the shared command globals to deterministic values
// and returns a restore func.
func setBulkTestGlobals(t *testing.T) func() {
	t.Helper()
	origFlags := pullFlags
	origQuiet := quiet
	origVerbose := verbose
	origStrategy := pullStrategy

	pullFlags = BulkCommandFlags{Depth: 1, Parallel: 4, Format: "default"}
	quiet = true
	verbose = false
	pullStrategy = "merge"

	return func() {
		pullFlags = origFlags
		quiet = origQuiet
		verbose = origVerbose
		pullStrategy = origStrategy
	}
}
