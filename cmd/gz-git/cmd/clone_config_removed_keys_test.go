// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseCloneConfigRejectsRemovedKeys covers the clone --config loader,
// which decoded its file without the removed-key guard before it called
// config.RejectRemovedKeys.
func TestParseCloneConfigRejectsRemovedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clone.yaml")
	if err := os.WriteFile(path, []byte("workspaces:\n  child:\n    integration:\n      readinessApproval: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := parseCloneConfig(path, false)
	const key = "workspaces.child.integration.readinessApproval"
	if err == nil || !strings.Contains(err.Error(), key+" was removed") || !strings.Contains(err.Error(), "delete it from "+path) {
		t.Fatalf("parseCloneConfig() error = %v, want %s rejected", err, key)
	}
}
