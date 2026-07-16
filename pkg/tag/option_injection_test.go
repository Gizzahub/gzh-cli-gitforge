// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package tag

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// TestManager_Create_RejectsOptionInjection verifies Create rejects a tag name
// git could read as an option, before the git process is spawned (AC4).
func TestManager_Create_RejectsOptionInjection(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)
	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	for _, name := range []string{"--upload-pack=/tmp/evil", "-x", "--force"} {
		err := m.Create(ctx, repo, CreateOptions{Name: name})
		if err == nil || !strings.Contains(err.Error(), "invalid tag name") {
			t.Fatalf("tag %q: expected invalid tag name error, got %v", name, err)
		}
	}
}
