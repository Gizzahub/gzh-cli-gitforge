// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeGitWorkspaceConfig writes a config with one git workspace "w" whose
// path is the given YAML scalar, and returns the config path.
func writeGitWorkspaceConfig(t *testing.T, dir, yamlPath string) string {
	t.Helper()
	body := "workspaces:\n  w:\n    type: git\n    url: https://example.invalid/team/w.git\n    path: " + yamlPath + "\n"
	p := filepath.Join(dir, ".gz-git.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestStatusWorkspacePathRule pins the status loader's workspace path rule,
// the one `workspace sync` uses (config.ResolvePath, ISSUE-391): only "~/" is
// special, and nothing is expanded from the environment.
func TestStatusWorkspacePathRule(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GZ_TEST_VAR", "expanded")

	cases := []struct {
		name string
		path string // YAML scalar
		want func(cfgDir string) string
	}{
		{"home subdir", "~/x", func(string) string { return filepath.Join(home, "x") }},
		{"bare tilde is relative", "'~'", func(d string) string { return filepath.Join(d, "~") }},
		{"tilde user is relative", "'~x/y'", func(d string) string { return filepath.Join(d, "~x", "y") }},
		{"env var stays literal", "'$GZ_TEST_VAR/x'", func(d string) string { return filepath.Join(d, "$GZ_TEST_VAR", "x") }},
		{"braced env var stays literal", "'${GZ_TEST_VAR}'", func(d string) string { return filepath.Join(d, "${GZ_TEST_VAR}") }},
		{"env var after home stays literal", "'~/$GZ_TEST_VAR'", func(string) string { return filepath.Join(home, "$GZ_TEST_VAR") }},
		{"plain relative", "./sub/x", func(d string) string { return filepath.Join(d, "sub", "x") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			data, err := (FileSpecLoader{}).Load(context.Background(), writeGitWorkspaceConfig(t, dir, tc.path))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			want := tc.want(dir)
			if !slices.Contains(data.Plan.Options.Roots, want) {
				t.Errorf("roots = %v, want %q", data.Plan.Options.Roots, want)
			}
			var target string
			for _, repo := range data.Plan.Input.Repos {
				if repo.Name == "w" {
					target = repo.TargetPath
				}
			}
			if target != want {
				t.Errorf("target = %q, want %q", target, want)
			}
		})
	}
}

// TestStatusWorkspacePathHomeUnavailable pins that a "~/" workspace path
// fails status with a clear error when no home directory can be determined,
// as sync does, while paths that need no home still load.
func TestStatusWorkspacePathHomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	dir := t.TempDir()
	_, err := (FileSpecLoader{}).Load(context.Background(), writeGitWorkspaceConfig(t, dir, "~/x"))
	if err == nil {
		t.Fatal("Load with ~/ and no home: want error, got nil")
	}

	dir = t.TempDir()
	if _, err := (FileSpecLoader{}).Load(context.Background(), writeGitWorkspaceConfig(t, dir, "'~x'")); err != nil {
		t.Errorf("Load with ~x and no home: %v", err)
	}
}
