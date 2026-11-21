// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// TestFlatInheritedPathsJoinChildDir pins the flat-specific rule that a
// relative repo path and root inherited from a parent join the child config's
// directory (not the parent's). It differs from pkg/config/symlink.go, which
// resolves inherited paths against the parent's directory.
func TestFlatInheritedPathsJoinChildDir(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, filepath.Join(dir, "up", "p.yaml"),
		"cleanupOrphans: true\nroots:\n  - rel-root\nrepositories:\n  - url: https://github.com/o/r.git\n    path: rel/repo\n")
	cfgPath := filepath.Join(dir, ".gz-git.yaml")
	writeFileAt(t, cfgPath, "parent: ./up/p.yaml\n")

	data, err := (FileSpecLoader{}).Load(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := data.Plan.Input.Repos[0].TargetPath, filepath.Join(dir, "rel", "repo"); got != want {
		t.Errorf("inherited repo path = %q, want %q (child dir)", got, want)
	}
	if want := filepath.Join(dir, "rel-root"); !slices.Contains(data.Plan.Options.Roots, want) {
		t.Errorf("roots = %v, want %q (child dir)", data.Plan.Options.Roots, want)
	}
}

// TestFlatGrandparentPathRule pins grandparent resolution: a parent's own
// `parent:` resolves against the parent's directory with the shared rule.
func TestFlatGrandparentPathRule(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GZ_GP_VAR", "expanded")

	cases := []struct {
		name        string
		grandparent string // YAML scalar in the parent config
		file        func(upDir string) string
	}{
		{"relative to parent dir", "./gp/g.yaml", func(u string) string { return filepath.Join(u, "gp", "g.yaml") }},
		{"directory reads .gz-git.yaml", "./gp/", func(u string) string { return filepath.Join(u, "gp", ".gz-git.yaml") }},
		{"env var stays literal", "'$GZ_GP_VAR/g.yaml'", func(u string) string { return filepath.Join(u, "$GZ_GP_VAR", "g.yaml") }},
		{"home file", "~/g.yaml", func(string) string { return filepath.Join(home, "g.yaml") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			upDir := filepath.Join(dir, "up")
			writeFileAt(t, tc.file(upDir), "repositories:\n  - url: https://github.com/o/fromgrand.git\n")
			writeFileAt(t, filepath.Join(upDir, "p.yaml"), "parent: "+tc.grandparent+"\nstrategy: pull\n")
			cfgPath := filepath.Join(dir, ".gz-git.yaml")
			writeFileAt(t, cfgPath, "parent: ./up/p.yaml\n")

			data, err := (FileSpecLoader{}).Load(context.Background(), cfgPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(data.Plan.Input.Repos) != 1 || data.Plan.Input.Repos[0].Name != "fromgrand" {
				t.Errorf("repos = %+v, want the grandparent's fromgrand repo", data.Plan.Input.Repos)
			}
		})
	}
}

// TestFlatHomeUnavailable pins that a "~/" repo path or root fails the load
// when no home directory can be determined.
func TestFlatHomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if _, err := os.UserHomeDir(); err == nil {
		t.Skip("home directory still resolvable on this platform")
	}

	for name, body := range map[string]string{
		"repo path": "repositories:\n  - url: https://github.com/o/r.git\n    path: ~/x\n",
		"root":      "cleanupOrphans: true\nroots:\n  - ~/x\nrepositories:\n  - url: https://github.com/o/r.git\n",
	} {
		t.Run(name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), ".gz-git.yaml")
			writeFileAt(t, cfgPath, body)
			_, err := (FileSpecLoader{}).Load(context.Background(), cfgPath)
			if err == nil || !strings.Contains(err.Error(), "home directory") {
				t.Errorf("want a home directory error, got %v", err)
			}
		})
	}
}
