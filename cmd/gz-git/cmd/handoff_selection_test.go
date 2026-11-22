// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestRepoPathSelection(t *testing.T) {
	root := t.TempDir()

	got, err := repoPathSelection(root, []string{root, filepath.Join(root, "a"), filepath.Join(root, "group", "b")})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Listed || !slices.Equal(got.Paths, []string{".", "a", "group/b"}) {
		t.Fatalf("got %+v", got)
	}

	// A planned repository outside the scan root must fail, not widen or drop.
	if _, err := repoPathSelection(root, []string{filepath.Join(filepath.Dir(root), "other")}); err == nil {
		t.Fatal("repository outside the root must be refused")
	}
}
