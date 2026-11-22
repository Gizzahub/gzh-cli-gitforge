// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output() //nolint:noctx // test helper
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func TestCommitMessageMapMissingRepoRefused(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	parent := t.TempDir()
	alpha := makeDirtyRepoUnder(t, parent, "alpha")
	beta := makeDirtyRepoUnder(t, parent, "beta")
	alphaBefore, betaBefore := headSubject(t, alpha), headSubject(t, beta)

	commitJSON = `{"alpha":"feat: alpha"}`
	err := runCommit(commitCmd, []string{parent})
	if err == nil || !strings.Contains(err.Error(), "no commit message for beta") {
		t.Fatalf("runCommit error = %v, want refusal naming beta", err)
	}
	if headSubject(t, alpha) != alphaBefore || headSubject(t, beta) != betaBefore {
		t.Fatal("a commit was created despite the refusal")
	}

	commitAutoMessage = true
	if err := runCommit(commitCmd, []string{parent}); err != nil {
		t.Fatalf("runCommit --auto-message: %v", err)
	}
	if got := headSubject(t, alpha); got != "feat: alpha" {
		t.Errorf("alpha HEAD = %q, want the explicit message", got)
	}
	if headSubject(t, beta) == betaBefore {
		t.Error("beta was not committed with a generated message")
	}
}

func TestCommitAutoMessageWithoutMapRefused(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)
	commitAutoMessage = true
	if err := runCommit(commitCmd, []string{t.TempDir()}); err == nil {
		t.Fatal("--auto-message without a message map returned nil, want error")
	}
}

func TestCommitFileMessageMap(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	parent := t.TempDir()
	repo := makeDirtyRepoUnder(t, parent, "svc")
	mapDir := t.TempDir()
	writeTestFile(t, mapDir, "messages.yaml", "svc: \"fix: from file\"\n")

	commitFile = filepath.Join(mapDir, "messages.yaml")
	if err := runCommit(commitCmd, []string{parent}); err != nil {
		t.Fatalf("runCommit --file: %v", err)
	}
	if got := headSubject(t, repo); got != "fix: from file" {
		t.Errorf("HEAD = %q, want the message from --file", got)
	}
}

func TestCommitFileRejectsEmptyAndConflictingSources(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	parent := t.TempDir()
	repo := makeDirtyRepoUnder(t, parent, "svc")
	before := headSubject(t, repo)
	mapDir := t.TempDir()
	writeTestFile(t, mapDir, "empty.json", "  \n")

	commitFile = filepath.Join(mapDir, "empty.json")
	if err := runCommit(commitCmd, []string{parent}); err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Errorf("empty --file error = %v, want an empty-file refusal", err)
	}

	commitJSON = `{"svc":"x"}`
	if err := runCommit(commitCmd, []string{parent}); err == nil || !strings.Contains(err.Error(), "give only one") {
		t.Errorf("--file with --json error = %v, want a single-source refusal", err)
	}
	if headSubject(t, repo) != before {
		t.Error("a commit was created despite the refusal")
	}
}

// The scan directory itself is addressed as ".".
func TestCommitRootRepositoryDotKey(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	repo := makeDirtyRepoUnder(t, t.TempDir(), "root")
	commitJSON = `{".":"feat: root"}`
	if err := runCommit(commitCmd, []string{repo}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	if got := headSubject(t, repo); got != "feat: root" {
		t.Errorf("HEAD = %q, want the message for \".\"", got)
	}
}

func TestCommitStageStagedCommitsIndexOnly(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	parent := t.TempDir()
	repo := makeDirtyRepoUnder(t, parent, "svc") // a.txt is untracked
	writeTestFile(t, repo, "staged.txt", "s\n")
	runGit(t, repo, "add", "staged.txt")

	commitStage = "staged"
	commitJSON = `{"svc":"feat: staged only"}`
	if err := runCommit(commitCmd, []string{parent}); err != nil {
		t.Fatalf("runCommit --stage staged: %v", err)
	}
	if got := headSubject(t, repo); got != "feat: staged only" {
		t.Fatalf("HEAD = %q, want the staged commit", got)
	}
	files := gitOutput(t, repo, "show", "--name-only", "--format=", "HEAD")
	if strings.TrimSpace(files) != "staged.txt" {
		t.Errorf("commit recorded %q, want only staged.txt", files)
	}
	if status := gitOutput(t, repo, "status", "--porcelain"); !strings.Contains(status, "?? a.txt") {
		t.Errorf("untracked a.txt was not left alone: %q", status)
	}
}

func TestCommitStageTrackedLeavesUntracked(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	parent := t.TempDir()
	repo := makeDirtyRepoUnder(t, parent, "svc") // a.txt is untracked
	tracked := strings.Fields(gitOutput(t, repo, "ls-files"))[0]
	writeTestFile(t, repo, tracked, "changed\n")

	commitStage = "tracked"
	commitJSON = `{"svc":"fix: tracked only"}`
	if err := runCommit(commitCmd, []string{parent}); err != nil {
		t.Fatalf("runCommit --stage tracked: %v", err)
	}
	files := gitOutput(t, repo, "show", "--name-only", "--format=", "HEAD")
	if strings.TrimSpace(files) != tracked {
		t.Errorf("commit recorded %q, want only %s", files, tracked)
	}
}

func TestCommitInvalidStageRefused(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)
	commitStage = "everything"
	if err := runCommit(commitCmd, []string{t.TempDir()}); err == nil || !strings.Contains(err.Error(), "--stage") {
		t.Fatalf("invalid --stage error = %v, want a --stage refusal", err)
	}
}

// An embedded repository that is not a submodule must not become a gitlink in
// the enclosing repository's commit.
func TestCommitLeavesNestedRepositoryOut(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)

	outer := makeDirtyRepoUnder(t, t.TempDir(), "outer")
	inner := filepath.Join(outer, "inner")
	runGit(t, outer, "init", "--quiet", inner)
	runGit(t, inner, "config", "user.name", "Test")
	runGit(t, inner, "config", "user.email", "test@example.com")
	writeTestFile(t, inner, "x.txt", "x\n")

	// The scan may also find inner as a repository of its own; it gets a
	// generated message, which is beside the point here.
	commitAutoMessage = true
	commitJSON = `{".":"feat: outer"}`
	if err := runCommit(commitCmd, []string{outer}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	if got := headSubject(t, outer); got != "feat: outer" {
		t.Fatalf("HEAD = %q, want the outer commit", got)
	}
	tree := gitOutput(t, outer, "ls-tree", "-r", "--name-only", "HEAD")
	if strings.Contains(tree, "inner") {
		t.Errorf("nested repository recorded in the outer commit:\n%s", tree)
	}
}
