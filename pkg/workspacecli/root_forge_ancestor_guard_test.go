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
// still planned. The base URL is unreachable and the context is cancelled, so
// a regression fails without reaching the network.
func TestPlanForgeWorkspacesRefusesAncestorOfRootConfigDir(t *testing.T) {
	parent := t.TempDir()
	devbox := filepath.Join(parent, "devbox")
	if err := os.Mkdir(devbox, 0o750); err != nil {
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
