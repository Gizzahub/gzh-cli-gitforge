// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package runtask

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSourceBranchRejectsRemovedKeys covers the task runtime's .gz-git.yaml
// read, which decoded the declaration without the removed-key guard before
// it read through config.ReadConfigFile.
func TestSourceBranchRejectsRemovedKeys(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gz-git.yaml")
	if err := os.WriteFile(path, []byte("branch:\n  integrationBranch: develop\n  prepareHighLoad: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewService(root, nil, nil).sourceBranch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "branch.prepareHighLoad was removed") || !strings.Contains(err.Error(), "delete it from "+path) {
		t.Fatalf("sourceBranch() error = %v, want branch.prepareHighLoad rejected", err)
	}
}
