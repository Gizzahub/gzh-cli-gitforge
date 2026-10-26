// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
)

func worktreeSyncTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	result, err := gitcmd.NewExecutor().Run(context.Background(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("git %v: %s", args, result.Stderr)
	}
}

func TestWorktreeSyncCommandJSONAndPrimaryConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "Sync Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "sync@example.test")
	t.Setenv("GIT_COMMITTER_NAME", "Sync Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "sync@example.test")
	main := filepath.Join(root, "main")
	linked := filepath.Join(root, "linked")
	worktreeSyncTestGit(t, root, "init", "--initial-branch=master", main)
	configText := "branch:\n  worktreeSync:\n    rules:\n      - match: feature/*\n        parent: master\n        strategy: rebase\n"
	if err := os.WriteFile(filepath.Join(main, ".gz-git.yaml"), []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	worktreeSyncTestGit(t, main, "add", ".gz-git.yaml")
	worktreeSyncTestGit(t, main, "commit", "-m", "config")
	worktreeSyncTestGit(t, main, "worktree", "add", "-b", "feature/api", linked)
	// Primary config wins over an edited configuration in the task worktree.
	if err := os.WriteFile(filepath.Join(linked, ".gz-git.yaml"), []byte("branch:\n  worktreeSync: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, configRoot, err := worktreeSyncLocation(context.Background(), linked)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadWorktreeSyncConfig(configRoot)
	if err != nil || cfg == nil || len(cfg.Rules) != 1 {
		t.Fatalf("primary policy not loaded: %+v, %v", cfg, err)
	}
	var output bytes.Buffer
	command := newWorktreeSyncCommand()
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{linked, "--dry-run", "--format", "json"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected dirty/upstream blockers")
	}
	var plans []branch.WorktreeSyncPlan
	if err := json.Unmarshal(output.Bytes(), &plans); err != nil {
		t.Fatalf("invalid JSON %q: %v", output.String(), err)
	}
	if len(plans) != 1 || len(plans[0].Results) != 2 || plans[0].Fetched || !plans[0].DryRun {
		t.Fatalf("unexpected report: %+v", plans)
	}
	paths, err := worktreeSyncRepositories(context.Background(), root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 2 {
		t.Fatalf("expected main and linked discovery: %v", paths)
	}
	output.Reset()
	command = newWorktreeSyncCommand()
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{root, "--dry-run", "--format", "json"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected blockers")
	}
	if err := json.Unmarshal(output.Bytes(), &plans); err != nil || len(plans) != 1 {
		t.Fatalf("common repository was not deduplicated: %q, %v", output.String(), err)
	}
}

func TestWorktreeSyncCommandRejectsInvalidFlags(t *testing.T) {
	for _, args := range [][]string{{"--format", "yaml"}, {"--scan-depth", "0"}} {
		cmd := newWorktreeSyncCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
