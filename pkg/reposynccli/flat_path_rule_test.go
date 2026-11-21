// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// TestFlatConfigPathRule pins the flat `repositories:` loader to the
// ISSUE-391 path rule shared with the workspaces format (config.ResolvePath):
// only "~/" expands, "$VAR" and a bare "~" stay literal, and relative paths
// join the config directory. It covers repo.path and roots.
func TestFlatConfigPathRule(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GZ_FLAT_VAR", "expanded")

	cases := []struct {
		name string
		path string // YAML scalar
		want func(cfgDir string) string
	}{
		{"home subdir expands", "~/x", func(string) string { return filepath.Join(home, "x") }},
		{"env var stays literal", "'$HOME/x'", func(d string) string { return filepath.Join(d, "$HOME", "x") }},
		{"braced env var stays literal", "'${GZ_FLAT_VAR}/x'", func(d string) string { return filepath.Join(d, "${GZ_FLAT_VAR}", "x") }},
		{"env var after home stays literal", "'~/$GZ_FLAT_VAR'", func(string) string { return filepath.Join(home, "$GZ_FLAT_VAR") }},
		{"bare tilde is relative", "'~'", func(d string) string { return filepath.Join(d, "~") }},
		{"tilde user is relative", "'~x/y'", func(d string) string { return filepath.Join(d, "~x", "y") }},
		{"plain relative joins config dir", "./sub/x", func(d string) string { return filepath.Join(d, "sub", "x") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, ".gz-git.yaml")
			writeFileAt(t, cfgPath, "roots:\n  - "+tc.path+"\ncleanupOrphans: true\nrepositories:\n  - url: https://github.com/o/r.git\n    path: "+tc.path+"\n")

			data, err := (FileSpecLoader{}).Load(context.Background(), cfgPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			want := tc.want(dir)
			if got := data.Plan.Input.Repos[0].TargetPath; got != want {
				t.Errorf("repo path = %q, want %q", got, want)
			}
			if !slices.Contains(data.Plan.Options.Roots, want) {
				t.Errorf("roots = %v, want %q", data.Plan.Options.Roots, want)
			}
		})
	}
}

// TestFlatParentPathParity pins that the flat loader reads `parent:` exactly
// where config.ResolveParentConfigFile (the workspaces/status rule) says, and
// that $VAR and a bare "~" are not expanded.
func TestFlatParentPathParity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GZ_PARENT_VAR", "expanded")

	cases := []struct {
		name   string
		parent string // YAML scalar
	}{
		{"home file", "~/up.yaml"},
		{"env var stays literal", "'$GZ_PARENT_VAR/p.yaml'"},
		{"bare tilde dir is relative", "'~'"},
		{"tilde user is relative", "'~x/p.yaml'"},
		{"directory reads .gz-git.yaml", "./up/"},
		{"relative file", "./up/p.yaml"},
	}
	const parentBody = "strategy: pull\nroots:\n  - /inherited\nrepositories:\n  - url: https://github.com/o/fromparent.git\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			parentVal := unquoteYAML(tc.parent)
			pdir, pfile, err := config.ResolveParentConfigFile(dir, parentVal)
			if err != nil {
				t.Fatal(err)
			}
			writeFileAt(t, filepath.Join(pdir, pfile), parentBody)
			cfgPath := filepath.Join(dir, ".gz-git.yaml")
			writeFileAt(t, cfgPath, "parent: "+tc.parent+"\n")

			data, err := (FileSpecLoader{}).Load(context.Background(), cfgPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(data.Plan.Input.Repos) != 1 || data.Plan.Input.Repos[0].Name != "fromparent" {
				t.Errorf("repos = %+v, want the parent's fromparent repo", data.Plan.Input.Repos)
			}
		})
	}
}
