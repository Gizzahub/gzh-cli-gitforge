// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestFetchPrunePassesNoTags(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args")
	script := filepath.Join(dir, "git")
	const body = "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GIT_ARG_LOG\"\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	repo := newGitRepo(gitcmd.NewExecutor(
		gitcmd.WithGitBinary(script),
		gitcmd.WithEnv([]string{"GIT_ARG_LOG=" + logPath}),
	), dir)
	if err := repo.fetchPrune(context.Background(), "origin"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	const want = "fetch origin --prune --quiet --no-tags\n"
	if string(got) != want {
		t.Fatalf("fetch args = %q, want %q", got, want)
	}
}

func TestFetchPruneFailureNamesNoTags(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	const body = "#!/bin/sh\necho refused >&2\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	repo := newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(script)), dir)
	err := repo.fetchPrune(context.Background(), "origin")
	if err == nil {
		t.Fatal("expected fetch failure")
	}
	if !strings.Contains(err.Error(), "git fetch origin --prune --no-tags failed") {
		t.Fatalf("error = %q, want the --no-tags command", err)
	}
}
