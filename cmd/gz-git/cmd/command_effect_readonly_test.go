// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// readOnlyExempt lists read-only declarations the behavior test cannot run
// in a sandbox, with the reason. Everything else declared read-only must
// appear in readOnlyRuns.
var readOnlyExempt = map[string]string{
	"watch":            "runs until interrupted",
	"config token get": "reads the real OS keychain",
}

// readOnlyRuns gives each read-only command arguments that reach its real
// work inside the sandbox repository.
var readOnlyRuns = map[string][]string{
	"":                      {},
	"help":                  {"help", "stash"},
	"version":               {"version"},
	"capability":            {"capability", contextReferenceObserveV1Capability},
	"schema":                {"schema"},
	"completion bash":       {"completion", "bash"},
	"completion fish":       {"completion", "fish"},
	"completion powershell": {"completion", "powershell"},
	"completion zsh":        {"completion", "zsh"},
	"branch list":           {"branch", "list", "."},
	"branch name":           {"branch", "name", "demo"},
	"config hierarchy":      {"config", "hierarchy"},
	"config show":           {"config", "show"},
	"config profile list":   {"config", "profile", "list"},
	"config profile show":   {"config", "profile", "show", "default"},
	"conflict detect":       {"conflict", "detect", "feature", "main"},
	"diff":                  {"diff", "."},
	"doctor":                {"doctor"},
	"handoff check":         {"handoff", "check", "."},
	"history blame":         {"history", "blame", "README.md"},
	"history contributors":  {"history", "contributors"},
	"history file":          {"history", "file", "README.md"},
	"history stats":         {"history", "stats"},
	"info":                  {"info", "."},
	"stash list":            {"stash", "list", "."},
	"tag list":              {"tag", "list", "."},
	"tag status":            {"tag", "status", "."},
	"workspace validate":    {"workspace", "validate"},
	"worktree list":         {"worktree", "list", "."},
}

func TestReadOnlyRunsCoverDeclarations(t *testing.T) {
	for key, targets := range commandEffects {
		_, run := readOnlyRuns[key]
		_, exempt := readOnlyExempt[key]
		switch {
		case len(targets) == 0 && !run && !exempt:
			t.Errorf("read-only %q has no behavior run and no exemption", key)
		case len(targets) != 0 && (run || exempt):
			t.Errorf("%q is declared mutating but listed as a read-only run", key)
		case run && exempt:
			t.Errorf("%q is both run and exempt", key)
		}
	}
}

// TestReadOnlyCommandsWriteNothing runs every read-only command against a
// repository whose remote is ahead of its tracking ref, so an undeclared
// fetch, pull, or config write shows up as a changed file.
func TestReadOnlyCommandsWriteNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	home := filepath.Join(base, "home")
	work := filepath.Join(base, "work")
	sandboxRepo(t, base, home, work)

	root := fullCommandTree()
	for key, args := range readOnlyRuns {
		t.Run(key, func(t *testing.T) {
			t.Chdir(work)
			before := snapshotFiles(t, base)
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(args)
			err := executeCommand(root)
			root.SetOut(nil)
			root.SetErr(nil)
			root.SetArgs(nil)
			if err != nil {
				t.Logf("%v returned %v (writes are still checked)", args, err)
			}
			diffSnapshots(t, before, snapshotFiles(t, base))
		})
	}
}

func sandboxRepo(t *testing.T, base, home, work string) {
	t.Helper()
	origin := filepath.Join(base, "origin.git")
	peer := filepath.Join(base, "peer")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	gitConfig := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitConfig, []byte("[user]\n\tname = t\n\temail = t@example.invalid\n[init]\n\tdefaultBranch = main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	git := func(dir string, args ...string) {
		t.Helper()
		c := exec.CommandContext(t.Context(), "git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git(base, "init", "--bare", origin)
	git(base, "clone", origin, work)
	readme := filepath.Join(work, "README.md")
	if err := os.WriteFile(readme, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Age the file so the index never holds a racily-clean entry that a
	// status refresh would rewrite.
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(readme, past, past); err != nil {
		t.Fatal(err)
	}
	git(work, "add", "README.md")
	git(work, "commit", "-m", "init")
	git(work, "branch", "feature")
	git(work, "push", "origin", "main", "feature")

	// The remote moves ahead of work's tracking refs.
	git(base, "clone", origin, peer)
	git(peer, "commit", "--allow-empty", "-m", "ahead")
	git(peer, "push", "origin", "main")
	if err := os.RemoveAll(peer); err != nil {
		t.Fatal(err)
	}

	// Config the read-only commands load, so they reach their real work
	// instead of stopping at "not found".
	t.Setenv("GIT_WORK_HOST", "sandbox")
	branchDecl := "branch:\n  integrationBranch:\n    - main\n  taskPattern:\n    - dev/*/*/*\n"
	taskDecl := "schema-version: 1\nrepository-id: sandbox\nworktree-roots:\n  sandbox: " + filepath.Join(base, "wt", "sandbox") + "\nintegration-provider: gz-git\n"
	for path, body := range map[string]string{
		filepath.Join(work, ".gz-git.yaml"):            branchDecl,
		filepath.Join(home, ".gz-git.yaml"):            branchDecl,
		filepath.Join(work, ".gz-git-task.yaml"):       taskDecl,
		filepath.Join(work, ".git", "info", "exclude"): ".gz-git.yaml\n.gz-git-task.yaml\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mgr, err := config.NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Initialize(); err != nil {
		t.Fatalf("initialize sandbox profile: %v", err)
	}
	git(work, "status")
}

// TestReadOnlySandboxDetectsWrites is the negative control: a declared
// tracking-refs command must change the sandbox, or the read-only test
// above proves nothing.
func TestReadOnlySandboxDetectsWrites(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	work := filepath.Join(base, "work")
	sandboxRepo(t, base, filepath.Join(base, "home"), work)
	t.Chdir(work)

	before := snapshotFiles(t, base)
	root := fullCommandTree()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"fetch", "."})
	err := executeCommand(root)
	root.SetOut(nil)
	root.SetErr(nil)
	root.SetArgs(nil)
	if err != nil {
		t.Fatalf("fetch: %v\n%s", err, out.String())
	}
	after := snapshotFiles(t, base)
	changed := len(before) != len(after)
	for path, sum := range after {
		if before[path] != sum {
			changed = true
		}
	}
	if !changed {
		t.Fatal("fetch against a remote that is ahead changed no file; the sandbox cannot detect writes")
	}
}

// snapshotFiles hashes every regular file under root. Directories are left
// out: creating gz-git's own empty config directories stays read-only.
func snapshotFiles(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	snap := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		snap[rel] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snap
}

func diffSnapshots(t *testing.T, before, after map[string][32]byte) {
	t.Helper()
	for path, sum := range after {
		if old, ok := before[path]; !ok {
			t.Errorf("created %s", path)
		} else if old != sum {
			t.Errorf("modified %s", path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			t.Errorf("removed %s", path)
		}
	}
}
