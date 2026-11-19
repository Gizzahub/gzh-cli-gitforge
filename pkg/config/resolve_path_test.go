// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolvePathEdgeCases pins the base-directory rule every loader shares
// (ISSUE-391): sync, the status loaders and the config loaders must not
// diverge on these inputs.
func TestResolvePathEdgeCases(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	base := filepath.FromSlash("/cfg/dir")

	tests := []struct {
		name string
		path string
		want string
	}{
		{"relative", "child", filepath.Join(base, "child")},
		{"dot slash", "./child", filepath.Join(base, "child")},
		{"dot", ".", base},
		{"empty", "", base},
		{"parent", "../sib", filepath.Join(filepath.Dir(base), "sib")},
		{"unclean relative", "a//b/../c", filepath.Join(base, "a", "c")},
		{"absolute unchanged", filepath.FromSlash("/abs/x"), filepath.FromSlash("/abs/x")},
		{"absolute not cleaned", filepath.FromSlash("/abs/x/../y"), filepath.FromSlash("/abs/x/../y")},
		{"home", "~/dev", filepath.Join(home, "dev")},
		{"bare tilde is relative", "~", filepath.Join(base, "~")},
		{"tilde user is relative", "~user/x", filepath.Join(base, "~user", "x")},
		{"env var is not expanded", "$HOME/x", filepath.Join(base, "$HOME", "x")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePath(base, tt.path)
			if err != nil {
				t.Fatalf("ResolvePath(%q, %q): %v", base, tt.path, err)
			}
			if got != tt.want {
				t.Errorf("ResolvePath(%q, %q) = %q, want %q", base, tt.path, got, tt.want)
			}
		})
	}
}
