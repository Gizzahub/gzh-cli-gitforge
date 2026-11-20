// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

const parentProfileYAML = "profiles:\n  fromparent:\n    name: fromparent\n    provider: github\n"

func writeFileAt(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestStatusParentPathRule pins the `parent:` rule of the status loader and
// its parity with the sync loader (config.LoadConfigRecursive): one rule,
// config.ResolveParentConfigFile (ISSUE-391).
func TestStatusParentPathRule(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GZ_PARENT_VAR", "expanded")

	cases := []struct {
		name       string
		parent     string // YAML scalar
		parentFile func(cfgDir string) string
	}{
		{"tilde user is relative", "'~x/p.yaml'", func(d string) string { return filepath.Join(d, "~x", "p.yaml") }},
		{"bare tilde dir is relative", "'~'", func(d string) string { return filepath.Join(d, "~", ".gz-git.yaml") }},
		{"env var stays literal", "'$GZ_PARENT_VAR/p.yaml'", func(d string) string { return filepath.Join(d, "$GZ_PARENT_VAR", "p.yaml") }},
		{"home file", "~/up.yaml", func(string) string { return filepath.Join(home, "up.yaml") }},
		{"directory without slash reads .gz-git.yaml", "./up", func(d string) string { return filepath.Join(d, "up", ".gz-git.yaml") }},
		{"directory with slash reads .gz-git.yaml", "./up/", func(d string) string { return filepath.Join(d, "up", ".gz-git.yaml") }},
		{"home directory reads .gz-git.yaml", "~/", func(string) string { return filepath.Join(home, ".gz-git.yaml") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFileAt(t, tc.parentFile(dir), parentProfileYAML)
			cfgPath := filepath.Join(dir, ".gz-git.yaml")
			writeFileAt(t, cfgPath, "parent: "+tc.parent+"\nworkspaces: {}\n")

			// A parent directory such as "~" must exist for the stat rule.
			cfg := &workspacesConfig{}
			cfg.Parent = unquoteYAML(tc.parent)
			if err := mergeProfilesIntoConfig(cfg, cfgPath); err != nil {
				t.Fatalf("status mergeProfilesIntoConfig: %v", err)
			}
			if cfg.Profiles["fromparent"] == nil {
				t.Errorf("status did not read the parent profile; profiles = %v", cfg.Profiles)
			}

			syncCfg, err := config.LoadConfigRecursive(dir, ".gz-git.yaml")
			if err != nil {
				t.Fatalf("sync LoadConfigRecursive: %v", err)
			}
			if syncCfg.ParentConfig == nil || syncCfg.ParentConfig.Profiles["fromparent"] == nil {
				t.Errorf("sync did not read the parent profile; parent = %+v", syncCfg.ParentConfig)
			}
		})
	}
}

func unquoteYAML(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}

// TestStatusParentPathHomeUnavailable pins that a "~/" parent fails status,
// like sync, when no home directory can be determined.
func TestStatusParentPathHomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".gz-git.yaml")
	writeFileAt(t, cfgPath, "parent: ~/up.yaml\nworkspaces: {}\n")

	cfg := &workspacesConfig{}
	cfg.Parent = "~/up.yaml"
	if err := mergeProfilesIntoConfig(cfg, cfgPath); err == nil {
		t.Error("status: want error for ~/ parent without a home directory")
	}
	if _, err := config.LoadConfigRecursive(dir, ".gz-git.yaml"); err == nil {
		t.Error("sync: want error for ~/ parent without a home directory")
	}
}
