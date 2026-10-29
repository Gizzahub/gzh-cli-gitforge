// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetadataFileSurfaceRejectsDirectory(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	if err := os.Mkdir(config, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := safeExistingPath(root, "config", false); err == nil {
		t.Fatal("config directory accepted as a regular metadata file")
	}
	if info, err := os.Lstat(config); err != nil || !info.IsDir() {
		t.Fatalf("rejected metadata surface changed: %v", err)
	}
}
