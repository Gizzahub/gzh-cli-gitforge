// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestInfraOpsCodesLayoutPreservesOnlyTrackedRegularBoundary(t *testing.T) {
	for _, kind := range []string{"tracked", "untracked", "symlink", "directory", "unknown tracked file", "existing child"} {
		t.Run(kind, func(t *testing.T) {
			fx := testutil.TempWorktreeWithBareOrigin(t)
			codes := filepath.Join(fx.Worktree, "codes")
			if err := os.MkdirAll(codes, 0o755); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(codes, ".envrc")
			switch kind {
			case "symlink":
				if err := os.Symlink(filepath.Join(fx.Worktree, "README.md"), marker); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(marker, 0o755); err != nil {
					t.Fatal(err)
				}
			default:
				writeFile(t, fx.Worktree, "codes/.envrc", "# preserved boundary\n")
			}
			if kind == "unknown tracked file" {
				writeFile(t, fx.Worktree, "codes/unknown", "not an owned preparation resource\n")
			}
			if kind != "untracked" && kind != "directory" {
				runGitInTest(t, fx.Worktree, "add", "codes")
				runGitInTest(t, fx.Worktree, "commit", "-m", "tracked layout")
			}
			if kind == "existing child" {
				if err := os.Mkdir(filepath.Join(codes, "sbkube"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			ownedRoot := filepath.Join(t.TempDir(), "owned")
			err := initializeInfraOpsResources(context.Background(), fx.Worktree, ownedRoot)
			if kind != "tracked" {
				if err == nil {
					t.Fatal("unsafe existing layout accepted")
				}
				if _, statErr := os.Lstat(ownedRoot); !os.IsNotExist(statErr) {
					t.Fatalf("unsafe layout created owned resources: %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("tracked boundary refused: %v", err)
			}
			contents, err := os.ReadFile(marker)
			if err != nil || string(contents) != "# preserved boundary\n" {
				t.Fatalf("tracked boundary changed: %q, %v", contents, err)
			}
		})
	}
}
