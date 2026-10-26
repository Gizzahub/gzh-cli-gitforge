// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// runWorkspaceSyncDryRun runs `workspace sync --dry-run` on root's config and
// returns the command error with its output.
func runWorkspaceSyncDryRun(t *testing.T, root string) (string, error) {
	t.Helper()
	t.Chdir(root)

	var output bytes.Buffer
	cmd := CommandFactory{}.newSyncCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"-c", filepath.Join(root, DefaultConfigFile), "--dry-run"})
	err := cmd.Execute()
	return output.String(), err
}

func writeSyncKindConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, DefaultConfigFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestWorkspaceSyncRefusesConfigWorkspaceWithSource pins ISSUE-104: sync
// fails on `type: config` with a source, at the top level and in the child
// config of a recursive config workspace, instead of warning and planning on.
func TestWorkspaceSyncRefusesConfigWorkspaceWithSource(t *testing.T) {
	conflict := "workspaces:\n  mixed:\n    type: config\n    source:\n      provider: gitlab\n      org: example\n      baseURL: http://127.0.0.1:1\n"

	t.Run("top level", func(t *testing.T) {
		root := t.TempDir()
		writeSyncKindConfig(t, root, conflict)

		out, err := runWorkspaceSyncDryRun(t, root)
		if !errors.Is(err, config.ErrWorkspaceTypeConflict) {
			t.Fatalf("sync error = %v, want ErrWorkspaceTypeConflict\n%s", err, out)
		}
	})

	t.Run("nested", func(t *testing.T) {
		root := t.TempDir()
		writeSyncKindConfig(t, root, "workspaces:\n  deep:\n    type: config\n    sync:\n      recursive: true\n")
		writeSyncKindConfig(t, filepath.Join(root, "deep"), conflict)

		out, err := runWorkspaceSyncDryRun(t, root)
		if !errors.Is(err, config.ErrWorkspaceTypeConflict) {
			t.Fatalf("sync error = %v, want ErrWorkspaceTypeConflict\n%s", err, out)
		}
	})
}

// TestWorkspaceSyncRefusesConfigWorkspaceCycle pins the shared cycle rule:
// a config workspace leading back to an enclosing config fails sync, as it
// fails status, instead of dropping the tree behind a load warning.
func TestWorkspaceSyncRefusesConfigWorkspaceCycle(t *testing.T) {
	root := t.TempDir()
	writeSyncKindConfig(t, root, "workspaces:\n  a:\n    type: config\n    path: a\n")
	writeSyncKindConfig(t, filepath.Join(root, "a"), "workspaces:\n  back:\n    type: config\n    path: ..\n")

	out, err := runWorkspaceSyncDryRun(t, root)
	if !errors.Is(err, config.ErrConfigCycle) {
		t.Fatalf("sync error = %v, want ErrConfigCycle\n%s", err, out)
	}
}

// TestWorkspaceSyncInferredConfigWorkspace pins ISSUE-104's reproduction on
// the sync side: an untyped, url-less recursive workspace plans its child's
// repositories, and a broken child config still fails sync instead of
// dropping every workspace behind a load warning.
func TestWorkspaceSyncInferredConfigWorkspace(t *testing.T) {
	inferred := "workspaces:\n  child:\n    sync:\n      recursive: true\n"

	t.Run("plans child repositories", func(t *testing.T) {
		root := t.TempDir()
		writeSyncKindConfig(t, root, inferred)
		writeSyncKindConfig(t, filepath.Join(root, "child"), "repositories:\n  - name: lib\n    url: https://example.invalid/lib.git\n")

		out, err := runWorkspaceSyncDryRun(t, root)
		if err != nil {
			t.Fatalf("sync: %v\n%s", err, out)
		}
		if !strings.Contains(out, "Planning config workspace 'child'") || !strings.Contains(out, "lib") {
			t.Fatalf("child repositories not planned:\n%s", out)
		}
	})

	t.Run("broken child config fails", func(t *testing.T) {
		root := t.TempDir()
		writeSyncKindConfig(t, root, inferred)
		writeSyncKindConfig(t, filepath.Join(root, "child"), "repositories: [\n")

		out, err := runWorkspaceSyncDryRun(t, root)
		if err == nil || !strings.Contains(err.Error(), "workspace 'child'") {
			t.Fatalf("sync error = %v, want the child config failure\n%s", err, out)
		}
	})
}
