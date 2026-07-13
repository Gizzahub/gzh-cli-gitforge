// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyncCmd_AutoDetectsParentConfigFromSubdir is the AC1 integration test:
// running `workspace sync` from a workspace subdirectory must discover the
// parent .gz-git.yaml (not re-init a new one) and surface it via "Using config:".
// An empty repositories list keeps the run side-effect-free — planning yields no
// actions, so no git/network work happens. The sync/quicksync/status/add/validate
// commands all auto-detect through the same detectConfigFile(".") call, so this
// exercises the shared discovery path.
func TestSyncCmd_AutoDetectsParentConfigFromSubdir(t *testing.T) {
	// Resolve symlinks so the cwd-derived path matches the written config path
	// on macOS (/var vs /private/var).
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	parent := filepath.Join(home, "workspace")
	sub := filepath.Join(parent, "sub", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	parentConfig := filepath.Join(parent, DefaultConfigFile)
	cfg := "version: 1\nkind: repositories\nstrategy: reset\nrepositories: []\n"
	if err := os.WriteFile(parentConfig, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(sub)

	factory := CommandFactory{}
	cmd := factory.newSyncCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workspace sync from subdir failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Using config:") {
		t.Errorf("expected 'Using config:' to surface parent discovery, got: %q", output)
	}
	if !strings.Contains(output, parentConfig) {
		t.Errorf("expected parent config path %s in output, got: %q", parentConfig, output)
	}
}
