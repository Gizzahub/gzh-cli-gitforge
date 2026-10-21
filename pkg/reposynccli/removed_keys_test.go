// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileSpecLoaderRejectsRemovedKeys covers a loader that decoded
// .gz-git.yaml without the removed-key guard before it read through
// config.ReadConfigFile.
func TestFileSpecLoaderRejectsRemovedKeys(t *testing.T) {
	for _, tc := range []struct{ body, key string }{
		{"branch:\n  readiness:\n    version: 1\n", "branch.readiness"},
		{"branch:\n  prepareHighLoad: true\n", "branch.prepareHighLoad"},
		{"workspaces:\n  child:\n    path: child\n    branch:\n      resourceHighLoad: true\n", "workspaces.child.branch.resourceHighLoad"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".gz-git.yaml")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := FileSpecLoader{}.Load(context.Background(), path)
			if err == nil || !strings.Contains(err.Error(), tc.key+" was removed") || !strings.Contains(err.Error(), "delete it from "+path) {
				t.Fatalf("Load() error = %v, want %s rejected", err, tc.key)
			}
		})
	}
}
