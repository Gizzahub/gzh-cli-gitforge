// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// useCommitEditor points $EDITOR at a shell script with the given body; "$1"
// is the message file.
func useCommitEditor(t *testing.T, body string) {
	t.Helper()
	editor := filepath.Join(t.TempDir(), "editor")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", editor)
}

// Canceling the editor must leave every repository uncommitted, with or
// without --yes (ISSUE-103).
func TestCommitEditCanceledCommitsNothing(t *testing.T) {
	for _, yes := range []bool{false, true} {
		t.Run(map[bool]string{false: "preview", true: "yes"}[yes], func(t *testing.T) {
			setCommitTestGlobals(t)
			isolateConfigHome(t)
			commitYes, commitEdit = yes, true
			useCommitEditor(t, `: > "$1"`)

			parent := t.TempDir()
			repo := makeDirtyRepoUnder(t, parent, "r1")
			before := headSubject(t, repo)

			commitJSON = `{"r1":"feat: x"}`
			if err := runCommit(commitCmd, []string{parent}); err != nil {
				t.Fatalf("runCommit: %v", err)
			}
			if after := headSubject(t, repo); after != before {
				t.Errorf("a commit was created: HEAD %q, was %q", after, before)
			}
		})
	}
}

// The edited message is the one committed, exactly once.
func TestCommitEditCommitsEditedMessage(t *testing.T) {
	setCommitTestGlobals(t)
	isolateConfigHome(t)
	commitYes, commitEdit = false, true
	useCommitEditor(t, `printf 'r1: feat: edited\n' > "$1"`)

	parent := t.TempDir()
	repo := makeDirtyRepoUnder(t, parent, "r1")
	before := headSubject(t, repo)

	commitJSON = `{"r1":"feat: x"}`
	if err := runCommit(commitCmd, []string{parent}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	if got := headSubject(t, repo); got != "feat: edited" {
		t.Fatalf("HEAD subject = %q, want the edited message", got)
	}
	parentSubject := gitOutputForIntegrateRun(t, repo, "log", "-1", "--skip=1", "--format=%s")
	if parentSubject != before {
		t.Errorf("parent of HEAD = %q, want %q: more than one commit was made", parentSubject, before)
	}
}
