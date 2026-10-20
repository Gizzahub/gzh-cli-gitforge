// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// TestPlanForgeWorkspacesRefusesAncestorOfRootConfigDir pins ISSUE-87: the
// root config's forge planner skips a workspace at an ancestor of the devbox,
// whose repositories or orphans would include the devbox, while a sibling is
// still planned. The base URL is unreachable and the context is canceled, so
// a regression fails without reaching the network.
func TestPlanForgeWorkspacesRefusesAncestorOfRootConfigDir(t *testing.T) {
	// The grandparent is owned by the test, not the shared temp directory.
	parent := filepath.Join(t.TempDir(), "outer", "parent")
	devbox := filepath.Join(parent, "devbox")
	if err := os.MkdirAll(devbox, 0o750); err != nil {
		t.Fatal(err)
	}
	linkToParent := filepath.Join(t.TempDir(), "parent-link")
	if err := os.Symlink(parent, linkToParent); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, path string
		refused    bool
	}{
		{"parent", "..", true},
		{"grandparent", filepath.Dir(parent), true},
		{"symlink to parent", linkToParent, true},
		{"sibling", "../sibling", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "workspaces:\n  org:\n    path: " + tc.path + "\n    source:\n      provider: gitlab\n      org: self\n      baseURL: http://127.0.0.1:1\n    sync:\n      cleanupOrphans: true\n"
			if err := os.WriteFile(filepath.Join(devbox, DefaultConfigFile), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.LoadConfigRecursive(devbox, DefaultConfigFile)
			if err != nil {
				t.Fatalf("LoadConfigRecursive: %v", err)
			}

			var out bytes.Buffer
			// A refused workspace never uses the context; the sibling fails fast.
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			actions, err := planForgeWorkspaces(ctx, cfg, nil, &out, "", false, false)
			skipped := strings.Contains(out.String(), "Skipping workspace 'org'") &&
				strings.Contains(out.String(), "contains the config directory")
			if skipped != tc.refused {
				t.Fatalf("refused=%v, want %v; err=%v; output:\n%s", skipped, tc.refused, err, out.String())
			}
			if planned := strings.Contains(out.String(), "Planning nested workspace 'org'"); planned == tc.refused {
				t.Fatalf("planned=%v, want %v; output:\n%s", planned, !tc.refused, out.String())
			}
			if tc.refused && (err != nil || len(actions) != 0) {
				t.Fatalf("refused workspace still planned: actions=%d err=%v; output:\n%s", len(actions), err, out.String())
			}
		})
	}
}

// TestConfigPlannerRefusesChildForgeWorkspaceBetweenDevboxAndChild covers the
// child side of ISSUE-87: a child's forge workspace at a directory between the
// devbox and the child does not reach the devbox, but contains the child's own
// config directory, so it is refused rather than planned.
func TestConfigPlannerRefusesChildForgeWorkspaceBetweenDevboxAndChild(t *testing.T) {
	configDir := t.TempDir()
	child := filepath.Join(configDir, "a", "child")
	if err := os.MkdirAll(child, 0o750); err != nil {
		t.Fatal(err)
	}
	childConfig := "workspaces:\n  up:\n    path: ..\n    source:\n      provider: gitlab\n      org: self\n      baseURL: http://127.0.0.1:1\n"
	if err := os.WriteFile(filepath.Join(child, DefaultConfigFile), []byte(childConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: filepath.Join(configDir, DefaultConfigFile), Workspaces: map[string]*config.Workspace{
		"child": {Path: filepath.Join("a", "child"), Sync: &config.SyncConfig{Recursive: true}},
	}}

	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	actions, err := planConfigWorkspacesWithDryRun(ctx, cfg, configDir, &out, "", true)
	if err != nil || len(actions) != 0 {
		t.Fatalf("actions=%d err=%v; output:\n%s", len(actions), err, out.String())
	}
	if !strings.Contains(out.String(), "Skipping workspace 'up'") ||
		!strings.Contains(out.String(), "contains the config directory") {
		t.Fatalf("intermediate forge workspace not refused; output:\n%s", out.String())
	}
}
