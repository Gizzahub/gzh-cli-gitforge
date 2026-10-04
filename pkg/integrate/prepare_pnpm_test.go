// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func pnpmFixture(t *testing.T) string {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeFile(t, fx.Worktree, ".gitignore", "node_modules/\n")
	writeFile(t, fx.Worktree, "package.json", "{\"name\":\"fixture\",\"private\":true}\n")
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "fixture")
	return fx.Worktree
}

func TestPreparePnpmFrozenLockfileInstallsIntoNodeModules(t *testing.T) {
	wt := pnpmFixture(t)
	// The fake only succeeds for the exact frozen install, in non-interactive
	// mode, and writes into a root and a nested workspace node_modules.
	bin := fakeCargo(t, `test "$1" = install
test "$2" = --frozen-lockfile
test "$CI" = true
mkdir -p node_modules/.pnpm packages/a/node_modules
: > node_modules/.pnpm/lock.yaml
: > packages/a/node_modules/.keep`)
	if err := os.Rename(filepath.Join(bin, "cargo"), filepath.Join(bin, "pnpm")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if err := runPrepareProfile(context.Background(), newGitRepo(gitcmd.NewExecutor(), wt), wt, pnpmFrozenLockfilePrepareV1); err != nil {
		t.Fatalf("pnpm preparation failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "node_modules", ".pnpm", "lock.yaml")); err != nil {
		t.Fatalf("install did not produce node_modules: %v", err)
	}
}

func TestPreparePnpmFrozenLockfileRejectsFailuresAndForbiddenOutput(t *testing.T) {
	for name, body := range map[string]string{
		"install fails":                  "exit 9",
		"tracked modification":           "echo x >> package.json",
		"untracked outside node_modules": ": > untracked.file",
	} {
		t.Run(name, func(t *testing.T) {
			wt := pnpmFixture(t)
			bin := fakeCargo(t, body)
			if err := os.Rename(filepath.Join(bin, "cargo"), filepath.Join(bin, "pnpm")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			err := runPrepareProfile(context.Background(), newGitRepo(gitcmd.NewExecutor(), wt), wt, pnpmFrozenLockfilePrepareV1)
			if err == nil {
				t.Fatal("preparation unexpectedly passed")
			}
		})
	}
}

func TestIsNodeModulesPath(t *testing.T) {
	for path, want := range map[string]bool{
		"node_modules/x":           true,
		"packages/a/node_modules/": true,
		"node_modules_extra/x":     false,
		"src/node_modules.ts":      false,
	} {
		if got := isNodeModulesPath(path); got != want {
			t.Errorf("isNodeModulesPath(%q) = %v, want %v", path, got, want)
		}
	}
}
