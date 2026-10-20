// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// TestEnsureChildConfigsSkipsWorkspaceAtConfigDir pins ISSUE-88: a workspace
// whose path is the config directory, spelled `.` or as a symlink to it, must
// not have its configLink applied, which would replace the devbox's own
// config. The config is loaded the way the CLI loads it before planning.
func TestEnsureChildConfigsSkipsWorkspaceAtConfigDir(t *testing.T) {
	cases := []struct {
		name string
		path func(t *testing.T, root string) string
	}{
		{name: "dot", path: func(*testing.T, string) string { return "." }},
		{name: "symlink to root", path: func(t *testing.T, root string) string {
			t.Helper()
			link := filepath.Join(t.TempDir(), "root-link")
			if err := os.Symlink(root, link); err != nil {
				t.Fatalf("symlink: %v", err)
			}
			return link
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "shared.yaml"), []byte("workspaces: {}\n"), 0o600); err != nil {
				t.Fatalf("write shared.yaml: %v", err)
			}
			original := "workspaces:\n  self:\n    path: " + tc.path(t, root) + "\n    configLink: shared.yaml\n"
			configFile := filepath.Join(root, DefaultConfigFile)
			if err := os.WriteFile(configFile, []byte(original), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			cfg, err := config.LoadConfigRecursive(root, DefaultConfigFile)
			if err != nil {
				t.Fatalf("LoadConfigRecursive: %v", err)
			}
			var out bytes.Buffer
			if err := ensureChildConfigsWithDryRun(&out, cfg, false); err != nil {
				t.Fatalf("ensureChildConfigsWithDryRun: %v", err)
			}

			info, err := os.Lstat(configFile)
			if err != nil {
				t.Fatalf("lstat config: %v", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				t.Fatalf("devbox config was replaced by a symlink; output:\n%s", out.String())
			}
			got, err := os.ReadFile(configFile)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			if string(got) != original {
				t.Fatalf("devbox config changed:\n%s", got)
			}
			if !strings.Contains(out.String(), "Skipping workspace 'self'") ||
				!strings.Contains(out.String(), "points to config directory") {
				t.Fatalf("missing self-sync warning; output:\n%s", out.String())
			}
		})
	}
}

// TestEnsureChildConfigsSkipsBootstrapAtConfigDir covers the bootstrap
// branch of ISSUE-88: a root loaded from a file other than .gz-git.yaml with
// a workspace at `path: .` and no configLink must not get a bootstrap
// .gz-git.yaml written into the devbox. Dry run must preview the skip, not
// the write.
func TestEnsureChildConfigsSkipsBootstrapAtConfigDir(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "root.yaml"), []byte("workspaces:\n  self:\n    path: .\n"), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		cfg, err := config.LoadConfigRecursive(root, "root.yaml")
		if err != nil {
			t.Fatalf("LoadConfigRecursive: %v", err)
		}
		var out bytes.Buffer
		if err := ensureChildConfigsWithDryRun(&out, cfg, dryRun); err != nil {
			t.Fatalf("dryRun=%v: ensureChildConfigsWithDryRun: %v", dryRun, err)
		}
		if _, err := os.Lstat(filepath.Join(root, DefaultConfigFile)); !os.IsNotExist(err) {
			t.Fatalf("dryRun=%v: bootstrap config written into the devbox (err=%v); output:\n%s", dryRun, err, out.String())
		}
		if !strings.Contains(out.String(), "points to config directory") || strings.Contains(out.String(), "bootstrap") {
			t.Fatalf("dryRun=%v: want only the self-sync warning; output:\n%s", dryRun, out.String())
		}
	}
}
