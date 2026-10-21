// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorkspaceLoadersRejectRemovedKeys covers the workspace loaders that
// decoded a config without the removed-key guard before they read through
// config.ReadConfigFile or config.RejectRemovedKeys.
func TestWorkspaceLoadersRejectRemovedKeys(t *testing.T) {
	const key = "branch.readiness"
	write := func(t *testing.T) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), ".gz-git.yaml")
		if err := os.WriteFile(path, []byte("branch:\n  readiness: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	check := func(t *testing.T, err error, path string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), key+" was removed") || !strings.Contains(err.Error(), "delete it from "+path) {
			t.Fatalf("error = %v, want %s rejected", err, key)
		}
	}
	t.Run("load", func(t *testing.T) {
		path := write(t)
		_, err := FileSpecLoader{}.Load(context.Background(), path)
		check(t, err, path)
	})
	t.Run("add", func(t *testing.T) {
		path := write(t)
		_, err := loadOrCreateConfig(path)
		check(t, err, path)
	})
	t.Run("validate", func(t *testing.T) {
		path := write(t)
		result, err := validateConfigFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Errors) != 1 {
			t.Fatalf("validate errors = %v, want the removed key", result.Errors)
		}
		check(t, errors.New(result.Errors[0]), path)
	})
}
