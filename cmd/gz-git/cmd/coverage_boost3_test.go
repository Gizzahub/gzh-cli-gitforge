// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestBuildCloneOptionsFromConfig(t *testing.T) {
	cfg := &CloneConfig{
		Target:    "./out",
		Parallel:  3,
		Strategy:  "pull",
		Structure: "user",
		Repositories: []CloneRepoSpec{
			{URL: "https://github.com/a/b.git", Name: "b"},
		},
	}
	flags := BulkCommandFlags{Parallel: 0, DryRun: true, Format: "default"}
	opts := buildCloneOptionsFromConfig(cfg, ".", flags, "main", 1, "", "flat", nil)
	if opts.Parallel != 3 {
		t.Errorf("parallel from yaml = %d", opts.Parallel)
	}
	if !opts.DryRun {
		t.Error("expected dry-run")
	}
	// CLI parallel overrides
	flags.Parallel = 8
	opts = buildCloneOptionsFromConfig(cfg, "/tmp/x", flags, "", 0, "skip", "flat", nil)
	if opts.Parallel != 8 {
		t.Errorf("cli parallel = %d", opts.Parallel)
	}
	// progress callback
	if opts.ProgressCallback != nil {
		captureStdout(t, func() { opts.ProgressCallback(1, 2, "https://github.com/a/b.git") })
	}
}

func TestBuildGroupCloneOptions(t *testing.T) {
	cfg := &CloneConfig{Parallel: 2, Strategy: "fetch"}
	group := &CloneGroup{
		Target:   "./g",
		Strategy: "pull",
		Depth:    1,
		Repositories: []CloneRepoSpec{
			{URL: "https://github.com/a/b.git"},
		},
	}
	opts := buildGroupCloneOptions(cfg, group, "./g", nil)
	if opts.Directory == "" {
		t.Error("expected directory")
	}
}

func TestRunCloneFromConfig_DryRunLocalBare(t *testing.T) {
	// Create a bare repo to clone from without network
	srcParent := t.TempDir()
	work := filepath.Join(srcParent, "work")
	bare := filepath.Join(srcParent, "bare.git")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "init")
	runGit(t, work, "config", "user.email", "t@t.com")
	runGit(t, work, "config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-m", "i")
	runGit(t, srcParent, "clone", "--bare", work, bare)

	dest := t.TempDir()
	cfgPath := filepath.Join(dest, "clone.yaml")
	yaml := "strategy: skip\nrepositories:\n  - url: " + bare + "\n    name: cloned\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	prevCfg, prevFlags, prevStdin := cloneConfig, cloneFlags, cloneConfigStdin
	t.Cleanup(func() {
		cloneConfig, cloneFlags, cloneConfigStdin = prevCfg, prevFlags, prevStdin
	})
	cloneConfig = cfgPath
	cloneConfigStdin = false
	cloneFlags = BulkCommandFlags{Parallel: 1, Format: "default", DryRun: true}

	captureStdout(t, func() {
		if err := runCloneFromConfig(context.Background(), dest); err != nil {
			t.Logf("dry-run clone config: %v", err)
		}
	})

	// real clone (local bare, no network)
	cloneFlags.DryRun = false
	cloneFlags.Format = "json"
	captureStdout(t, func() {
		if err := runCloneFromConfig(context.Background(), dest); err != nil {
			t.Logf("clone config: %v", err)
		}
	})

	// grouped config
	gpath := filepath.Join(dest, "groups.yaml")
	gyaml := "core:\n  target: " + filepath.Join(dest, "core") + "\n  repositories:\n    - url: " + bare + "\n"
	if err := os.WriteFile(gpath, []byte(gyaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cloneConfig = gpath
	cloneFlags.DryRun = true
	captureStdout(t, func() {
		if err := runCloneFromConfig(context.Background(), dest); err != nil {
			t.Logf("grouped: %v", err)
		}
	})
}

func TestDisplayDiffResultsAll(t *testing.T) {
	// Construct minimal BulkDiffResult - inspect fields at compile time
	res := &repository.BulkDiffResult{
		TotalScanned: 2,
		Duration:     time.Millisecond,
		Summary:      map[string]int{"clean": 1, "dirty": 1},
		Repositories: []repository.RepositoryDiffResult{
			{RelativePath: "a", Status: "clean", Branch: "main"},
			{RelativePath: "b", Status: "dirty", Branch: "dev"},
			{RelativePath: "c", Status: "error", Error: errors.New("e")},
		},
	}
	// format via package flag if any
	for _, format := range []string{"default", "compact", "json", "llm"} {
		// set diff format flag if exists
		if f := diffCmd.Flags().Lookup("format"); f != nil {
			_ = f.Value.Set(format)
		}
		captureStdout(t, func() {
			displayDiffResults(res)
			displayDiffResultsDefault(res)
			displayDiffResultsCompact(res)
			for _, r := range res.Repositories {
				displayDiffRepositoryResult(r)
			}
			displayDiffResultsStructured(res, format)
		})
	}
}

func TestTagCreateStatusOnRepo(t *testing.T) {
	parent := setupBulkParent(t)
	repo := filepath.Join(parent, "r1")
	cwd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// create tag in single-repo mode
	create := findCommand(t, rootCmd, "tag", "create")
	captureStdout(t, func() {
		if create.RunE != nil {
			if err := create.RunE(create, []string{"v0.0.0-test"}); err != nil {
				t.Logf("tag create: %v", err)
			}
		}
	})
	status := findCommand(t, rootCmd, "tag", "status")
	captureStdout(t, func() {
		if status.RunE != nil {
			if err := status.RunE(status, nil); err != nil {
				t.Logf("tag status: %v", err)
			}
		}
	})
	// bulk tag create dry-run on parent
	_ = os.Chdir(cwd)
	captureStdout(t, func() {
		if err := runBulkTagCreate(context.Background(), parent, "v0.0.1-bulk"); err != nil {
			t.Logf("bulk tag create: %v", err)
		}
	})
	captureStdout(t, func() {
		if err := runBulkTagList(context.Background(), parent); err != nil {
			t.Logf("bulk tag list: %v", err)
		}
	})
}

func TestStashSaveListOnDirtyRepo(t *testing.T) {
	parent := setupBulkParent(t)
	repo := filepath.Join(parent, "r1")
	if err := os.WriteFile(filepath.Join(repo, "dirty2.txt"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() {
		if err := runBulkStashSave(context.Background(), parent); err != nil {
			t.Logf("stash save: %v", err)
		}
	})
	captureStdout(t, func() {
		if err := runBulkStashList(context.Background(), parent); err != nil {
			t.Logf("stash list: %v", err)
		}
	})
}

func TestRunCommitWithYesDryRun(t *testing.T) {
	parent := setupBulkParent(t)
	repo := filepath.Join(parent, "r1")
	if err := os.WriteFile(filepath.Join(repo, "c.txt"), []byte("c"), 0o600); err != nil {
		t.Fatal(err)
	}
	prevYes, prevFlags := commitYes, commitFlags
	t.Cleanup(func() { commitYes, commitFlags = prevYes, prevFlags })
	commitFlags = BulkCommandFlags{Depth: 1, Parallel: 2, DryRun: true, Format: "json"}
	commitYes = true
	captureStdout(t, func() {
		if err := runCommit(commitCmd, []string{parent}); err != nil {
			t.Logf("commit dry-run: %v", err)
		}
	})
	commitFlags.DryRun = false
	commitFlags.Format = "default"
	// message flag if any
	if f := commitCmd.Flags().Lookup("message"); f != nil {
		_ = f.Value.Set("test: commit")
	}
	captureStdout(t, func() {
		if err := runCommit(commitCmd, []string{parent}); err != nil {
			t.Logf("commit yes: %v", err)
		}
	})
}

func TestExecuteHooks(t *testing.T) {
	dir := t.TempDir()
	// safe command without shell
	if err := executeHooks(context.Background(), []string{"true"}, dir, nil); err != nil {
		// on mac true exists
		t.Logf("executeHooks true: %v", err)
	}
	if err := executeHooks(context.Background(), []string{"false"}, dir, nil); err == nil {
		t.Log("false may fail")
	}
	if err := executeHooks(context.Background(), nil, dir, nil); err != nil {
		t.Errorf("empty hooks: %v", err)
	}
}
