// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

func siblingFixture(t *testing.T) (*testutil.WorktreeOrigin, config.SiblingDependency) {
	t.Helper()
	remote := "ssh://git@example.test/gitrump-ce.git"
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeFile(t, fx.Worktree, "payload.txt", "source\n")
	runGitInTest(t, fx.Worktree, "add", "payload.txt")
	runGitInTest(t, fx.Worktree, "commit", "-m", "payload")
	runGitInTest(t, fx.Worktree, "push", "origin", "HEAD:master")
	runGitInTest(t, fx.Worktree, "fetch", "origin")
	runGitInTest(t, fx.Clone, "fetch", "origin")
	runGitInTest(t, fx.Clone, "reset", "--hard", "origin/master")
	runGitInTest(t, fx.Clone, "remote", "set-url", "origin", remote)
	runGitInTest(t, fx.Clone, "remote", "set-url", "--push", "origin", remote)
	return fx, config.SiblingDependency{Name: "gitrump-ce", Remote: remote, Path: "../gitrump-ce", SourceBranch: "master", Revision: "source-tip"}
}

func TestProvisionSiblingReleasesOnlyUnchangedOwnedArchive(t *testing.T) {
	fx, dep := siblingFixture(t)
	root := filepath.Join(t.TempDir(), "gitrump-ee", "task")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	lease, err := ProvisionSibling(context.Background(), root, fx.Clone, dep)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(lease.Destination, "payload.txt")); err != nil || string(data) != "source\n" {
		t.Fatalf("payload=%q err=%v", data, err)
	}
	if err := ReleaseSibling(lease); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lease.Destination); !os.IsNotExist(err) {
		t.Fatalf("destination remains: %v", err)
	}
}

func TestProvisionSiblingRefusesCollisionAndTamperedRelease(t *testing.T) {
	fx, dep := siblingFixture(t)
	root := filepath.Join(t.TempDir(), "gitrump-ee", "task")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(filepath.Dir(root), "gitrump-ce")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ProvisionSibling(context.Background(), root, fx.Clone, dep); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("collision err=%v", err)
	}
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	lease, err := ProvisionSibling(context.Background(), root, fx.Clone, dep)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, lease.Destination, "payload.txt", "changed\n")
	if err := ReleaseSibling(lease); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("release err=%v", err)
	}
	if _, err := os.Stat(lease.Destination); err != nil {
		t.Fatalf("tampered destination removed: %v", err)
	}
}

func TestProvisionSiblingRefusesStaleSourceTip(t *testing.T) {
	fx, dep := siblingFixture(t)
	runGitInTest(t, fx.Clone, "commit", "--allow-empty", "-m", "unpublished")
	root := filepath.Join(t.TempDir(), "gitrump-ee", "task")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ProvisionSibling(context.Background(), root, fx.Clone, dep); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("stale tip err=%v", err)
	}
}
