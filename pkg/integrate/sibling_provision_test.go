// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
	t.Parallel()
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

func TestProvisionSiblingAdmitsOnlyUntrackedRuntimeMetadataOutsideArchive(t *testing.T) {
	t.Parallel()
	fx, dep := siblingFixture(t)
	metadata := []string{
		".ce/task-runtime.yaml",
		".ce/audit/events.jsonl",
		"modules/server/.ce/heartbeat/events.jsonl",
		"modules/server/.ce/heartbeat/sessions/session.json",
		".agent-mesh/checkpoints/session.checkpoint.json",
	}
	for _, name := range metadata {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(fx.Clone, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, fx.Clone, name, "runtime metadata\n")
	}
	root := filepath.Join(t.TempDir(), "gitrump-ee", "task")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	lease, err := ProvisionSibling(context.Background(), root, fx.Clone, dep)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range metadata {
		if _, err := os.Stat(filepath.Join(lease.Destination, name)); !os.IsNotExist(err) {
			t.Fatalf("runtime metadata entered committed archive: %s, err=%v", name, err)
		}
		if _, err := os.Stat(filepath.Join(fx.Clone, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := ReleaseSibling(lease); err != nil {
		t.Fatal(err)
	}
}

func TestSiblingSourceStatusCleanFailsClosed(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, status string
		clean        bool
	}{
		{"empty", "", true},
		{"audit", "?? server/.ce/audit/events.jsonl\x00", true},
		{"heartbeat", "?? .ce/heartbeat/state.json\x00", true},
		{"session", "?? server/.ce/heartbeat/sessions/session.json\x00", true},
		{"declaration", "?? .ce/task-runtime.yaml\x00", true},
		{"checkpoint", "?? .agent-mesh/checkpoints/session.checkpoint.json\x00", true},
		{"tracked metadata", " M .ce/task-runtime.yaml\x00", false},
		{"staged metadata", "A  .ce/task-runtime.yaml\x00", false},
		{"tracked source", " M payload.txt\x00", false},
		{"unknown", "?? hidden-source.rs\x00", false},
		{"nested declaration", "?? server/.ce/task-runtime.yaml\x00", false},
		{"nested checkpoint", "?? .agent-mesh/checkpoints/nested/session.checkpoint.json\x00", false},
		{"wrong suffix", "?? .agent-mesh/checkpoints/session.json\x00", false},
		{"empty checkpoint basename", "?? .agent-mesh/checkpoints/.checkpoint.json\x00", false},
		{"flow continuation", "?? .omo/run-continuation/session.json\x00", false},
		{"traversal", "?? ../.ce/audit/events.jsonl\x00", false},
		{"noncanonical", "?? server/../.ce/audit/events.jsonl\x00", false},
		{"absolute", "?? /.ce/audit/events.jsonl\x00", false},
		{"missing delimiter", "?? .ce/audit/events.jsonl", false},
		{"empty record", "?? .ce/audit/events.jsonl\x00\x00", false},
		{"rename", "R  .ce/audit/events.jsonl\x00payload.txt\x00", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := siblingSourceStatusClean(test.status); got != test.clean {
				t.Fatalf("clean=%t want=%t for %q", got, test.clean, test.status)
			}
		})
	}
	for _, name := range []string{".ce/task-runtime.yaml", ".agent-mesh/checkpoints/session.checkpoint.json"} {
		if statusCleanExceptUntracked("?? "+name+"\x00", isCELogRuntimeArtifactPath) {
			t.Fatalf("CE log runtime policy widened for %s", name)
		}
	}
}

func TestProvisionSiblingStillRejectsSourceChangesAndStagedMetadata(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"unknown untracked", "tracked source", "staged metadata"} {
		t.Run(name, func(t *testing.T) {
			fx, dep := siblingFixture(t)
			switch name {
			case "unknown untracked":
				writeFile(t, fx.Clone, "unexpected.txt", "not committed\n")
			case "tracked source":
				writeFile(t, fx.Clone, "payload.txt", "changed\n")
			case "staged metadata":
				if err := os.Mkdir(filepath.Join(fx.Clone, ".ce"), 0o755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, fx.Clone, ".ce/task-runtime.yaml", "changed\n")
				runGitInTest(t, fx.Clone, "add", "-f", ".ce/task-runtime.yaml")
			}
			if _, _, err := resolveSiblingSource(context.Background(), fx.Clone, dep); err == nil || !strings.Contains(err.Error(), "dirty") {
				t.Fatalf("source change admitted: %v", err)
			}
		})
	}
}

func TestProvisionSiblingRefusesCollisionAndTamperedRelease(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestDiscoverSiblingPrimaryUsesDeclaredNestedWorkspace(t *testing.T) {
	t.Parallel()
	dependency, dep := siblingFixture(t)
	primary := testutil.TempWorktreeWithBareOrigin(t)
	source := filepath.Join(primary.Clone, dep.Name)
	runGit(t, primary.Clone, "clone", dependency.Origin, source)
	runGit(t, source, "checkout", "-B", "master", "origin/master")
	runGit(t, source, "remote", "set-url", "origin", dep.Remote)
	runGit(t, source, "remote", "set-url", "--push", "origin", dep.Remote)
	writeFile(t, primary.Clone, ".gz-git.yaml", "workspaces:\n  "+dep.Name+":\n    url: "+dep.Remote+"\n")

	got, err := DiscoverSiblingPrimary(context.Background(), primary.Worktree, dep)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("source = %s, want %s", got, want)
	}
}

func TestDiscoverSiblingPrimaryKeepsAdjacentCompatibility(t *testing.T) {
	t.Parallel()
	dependency, dep := siblingFixture(t)
	primary := testutil.TempWorktreeWithBareOrigin(t)
	source := filepath.Join(filepath.Dir(primary.Clone), dep.Name)
	runGit(t, primary.Clone, "clone", dependency.Origin, source)
	runGit(t, source, "checkout", "-B", "master", "origin/master")
	runGit(t, source, "remote", "set-url", "origin", dep.Remote)
	runGit(t, source, "remote", "set-url", "--push", "origin", dep.Remote)

	got, err := DiscoverSiblingPrimary(context.Background(), primary.Worktree, dep)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("source = %s, want %s", got, want)
	}
}

func TestDiscoverSiblingPrimaryDoesNotFallbackFromPresentAdjacentPath(t *testing.T) {
	t.Parallel()
	dependency, dep := siblingFixture(t)
	primary := testutil.TempWorktreeWithBareOrigin(t)
	nested := filepath.Join(primary.Clone, dep.Name)
	runGit(t, primary.Clone, "clone", dependency.Origin, nested)
	runGit(t, nested, "checkout", "-B", "master", "origin/master")
	runGit(t, nested, "remote", "set-url", "origin", dep.Remote)
	runGit(t, nested, "remote", "set-url", "--push", "origin", dep.Remote)
	writeFile(t, primary.Clone, ".gz-git.yaml", "workspaces:\n  "+dep.Name+":\n    url: "+dep.Remote+"\n")
	if err := os.Mkdir(filepath.Join(filepath.Dir(primary.Clone), dep.Name), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverSiblingPrimary(context.Background(), primary.Worktree, dep)
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("DiscoverSiblingPrimary error = %v, want adjacent refusal", err)
	}
}

func TestDiscoverSiblingPrimaryRejectsMismatchedOrSymlinkedNestedWorkspace(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup is platform-dependent")
	}
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, primary, source string, dep config.SiblingDependency)
		want  string
	}{
		{
			name: "mismatched remote",
			setup: func(t *testing.T, primary, source string, dep config.SiblingDependency) {
				t.Helper()
				writeFile(t, primary, ".gz-git.yaml", "workspaces:\n  "+dep.Name+":\n    url: ssh://git@example.test/other.git\n")
			},
			want: "does not match sibling remote",
		},
		{
			name: "symlink",
			setup: func(t *testing.T, primary, source string, dep config.SiblingDependency) {
				t.Helper()
				writeFile(t, primary, ".gz-git.yaml", "workspaces:\n  "+dep.Name+":\n    url: "+dep.Remote+"\n")
				if err := os.Symlink(source, filepath.Join(primary, dep.Name)); err != nil {
					t.Fatal(err)
				}
			},
			want: "symlink",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dependency, dep := siblingFixture(t)
			primary := testutil.TempWorktreeWithBareOrigin(t)
			tc.setup(t, primary.Clone, dependency.Clone, dep)

			_, err := DiscoverSiblingPrimary(context.Background(), primary.Worktree, dep)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DiscoverSiblingPrimary error = %v, want %q", err, tc.want)
			}
		})
	}
}
