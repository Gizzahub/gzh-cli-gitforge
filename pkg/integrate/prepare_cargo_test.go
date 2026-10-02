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

// fakeCargo installs a fake cargo binary whose behavior is scripted per
// subcommand, mirroring the fakeGo helper in prepare_test.go. Script traces
// are written under target/ so the status validation reads them as build
// output, never as tree changes.
func fakeCargo(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cargo")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func cargoFixture(t *testing.T) string {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeFile(t, fx.Worktree, ".gitignore", "target/\n")
	writeFile(t, fx.Worktree, "Cargo.toml", "[workspace]\nmembers = []\n")
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "fixture")
	return fx.Worktree
}

func TestPrepareCargoWorkspaceRunsFetchThenCheckAndKeepsTargetArtifacts(t *testing.T) {
	wt := cargoFixture(t)
	// check only runs after fetch (marker under target/), writes its build
	// artifact under target/, and never touches tracked paths. If the
	// executor skipped fetch, ran the steps out of order, dropped the check
	// step, or rejected the target/ output, this script fails and so does
	// the test.
	bin := fakeCargo(t, `mkdir -p target
if [ "$1" = fetch ]; then : > target/.fetched; exit 0; fi
if [ "$1" = check ] && [ "$2" = --workspace ]; then test -e target/.fetched; : > target/artifact; exit 0; fi
exit 97`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if err := runPrepareProfile(context.Background(), newGitRepo(gitcmd.NewExecutor(), wt), wt, cargoWorkspacePrepareV1); err != nil {
		t.Fatalf("cargo preparation failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "target", "artifact")); err != nil {
		t.Fatalf("check step did not produce its build artifact: %v", err)
	}
}

func TestPrepareCargoWorkspaceRejectsFailuresAndForbiddenOutput(t *testing.T) {
	for name, body := range map[string]string{
		"fetch fails":              "exit 9",
		"check fails":              "if [ \"$1\" = fetch ]; then exit 0; fi\nexit 8",
		"tracked modification":     "if [ \"$1\" = fetch ]; then exit 0; fi\necho x >> Cargo.toml",
		"untracked outside target": "if [ \"$1\" = fetch ]; then exit 0; fi\n: > untracked.file",
	} {
		t.Run(name, func(t *testing.T) {
			wt := cargoFixture(t)
			t.Setenv("PATH", fakeCargo(t, body)+":"+os.Getenv("PATH"))
			err := runPrepareProfile(context.Background(), newGitRepo(gitcmd.NewExecutor(), wt), wt, cargoWorkspacePrepareV1)
			if err == nil {
				t.Fatal("preparation unexpectedly passed")
			}
		})
	}
}
