// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// setCommitTestGlobals resets the package-level commit flags and returns a restore func.
func setCommitTestGlobals(t *testing.T) {
	t.Helper()
	origFlags, origQuiet, origVerbose := commitFlags, quiet, verbose
	origJSON, origYes, origEdit := commitJSON, commitYes, commitEdit
	commitFlags = BulkCommandFlags{Depth: 1, Parallel: 2, Format: "default"}
	quiet, verbose = true, false
	commitJSON, commitYes, commitEdit = "", true, false
	t.Cleanup(func() {
		commitFlags, quiet, verbose = origFlags, origQuiet, origVerbose
		commitJSON, commitYes, commitEdit = origJSON, origYes, origEdit
	})
}

// makeDirtyRepoUnder clones a committed repo to parent/name and dirties it.
func makeDirtyRepoUnder(t *testing.T, parent, name string) string {
	t.Helper()
	dest := filepath.Join(parent, name)
	runGit(t, ".", "clone", "--quiet", testutil.TempGitRepoWithCommit(t), dest)
	if err := os.WriteFile(filepath.Join(dest, "a.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dest
}

func headSubject(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output() //nolint:noctx // test helper
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitJSONUnmatchedKeyRejectedBeforeCommit(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	parent := t.TempDir()
	repo := makeDirtyRepoUnder(t, parent, "sbkube")
	before := headSubject(t, repo)

	commitJSON = `{"sbkub":"msg","zzz":"msg"}`
	err := runCommit(commitCmd, []string{parent})
	if err == nil {
		t.Fatal("runCommit with unmatched --json keys returned nil, want error")
	}
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitToolError {
		t.Errorf("exit code = %d, want %d", got, cliutil.ExitToolError)
	}
	for _, want := range []string{"sbkub, zzz", "sbkube"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if after := headSubject(t, repo); after != before {
		t.Errorf("a commit was created: HEAD %q, was %q", after, before)
	}
}

func TestCommitJSONMatchedKeyCommitsMessage(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	parent := t.TempDir()
	repo := makeDirtyRepoUnder(t, parent, "sbkube")

	commitJSON = `{"sbkube":"feat: explicit message"}`
	if err := runCommit(commitCmd, []string{parent}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	if got := headSubject(t, repo); got != "feat: explicit message" {
		t.Errorf("HEAD subject = %q, want the explicit message", got)
	}
}

// A nested key must match when the scan directory is relative, the usual case
// of running from the workspace root.
func TestCommitJSONNestedKeyFromRelativeDirectory(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)
	commitFlags.Depth = 3
	depthFlag := commitCmd.Flags().Lookup("scan-depth")
	origChanged := depthFlag.Changed
	depthFlag.Changed = true
	t.Cleanup(func() { depthFlag.Changed = origChanged })

	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "group"), 0o750); err != nil {
		t.Fatal(err)
	}
	repo := makeDirtyRepoUnder(t, filepath.Join(parent, "group"), "repo")
	t.Chdir(parent)

	commitJSON = `{"group/repo":"feat: explicit"}`
	if err := runCommit(commitCmd, []string{"."}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	if got := headSubject(t, repo); got != "feat: explicit" {
		t.Errorf("HEAD subject = %q, want the explicit message", got)
	}
}
