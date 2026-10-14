// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

//go:build !windows

package integrate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A write/execute-only parent permits temp creation and rename but refuses
// the read-only directory open used by the final durability sync.
func TestAtomicStateWriteFailureAfterRenameLeavesNewDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Errorf("restore state directory: %v", err)
		}
	})
	if err := os.Chmod(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	err := atomicStateWrite(path, []byte("new"))
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected final directory-open failure, got %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new" {
		t.Fatalf("rename must already have published the new document: %q %v", data, err)
	}
}
