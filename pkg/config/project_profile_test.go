// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ISSUE-111: a project naming a profile defined inline in its parent config
// failed every command because the loader read only the profile files.

// setupProfileHome isolates HOME, writes each file relative to it, and changes
// into home/devbox, the project. It returns the home directory.
func setupProfileHome(t *testing.T, files map[string]string) string {
	t.Helper()
	// Resolve symlinks: on macOS os.Getwd() returns the /private/var form.
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	// Linux reads XDG_CONFIG_HOME before HOME, and CI runners set it; without
	// this the profiles and active profile other tests leave behind leak in.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	for rel, content := range files {
		path := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dir := filepath.Join(home, "devbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return home
}

func loadEffective(t *testing.T) (*EffectiveConfig, error) {
	t.Helper()
	loader, err := NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	if err := loader.Load(); err != nil {
		return nil, err
	}
	return loader.ResolveConfig(nil)
}

func TestLoadUsesInlineProfileFromParentChain(t *testing.T) {
	setupProfileHome(t, map[string]string{
		"workstation/.gz-git.yaml": `profiles:
  polypia:
    provider: gitlab
    parallel: 7
`,
		"devbox/.gz-git.yaml": `parent: ../workstation/.gz-git.yaml
profile: polypia
`,
	})

	effective, err := loadEffective(t)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if effective.Parallel != 7 {
		t.Errorf("Parallel = %d, want 7 from the parent's inline profile", effective.Parallel)
	}
	if effective.Provider != "gitlab" {
		t.Errorf("Provider = %q, want gitlab from the parent's inline profile", effective.Provider)
	}
}

func TestLoadUsesInlineProfileFromProjectConfig(t *testing.T) {
	setupProfileHome(t, map[string]string{
		"devbox/.gz-git.yaml": `profile: local
profiles:
  local:
    parallel: 3
`,
	})

	effective, err := loadEffective(t)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if effective.Parallel != 3 {
		t.Errorf("Parallel = %d, want 3 from the project's inline profile", effective.Parallel)
	}
}

func TestLoadPrefersInlineProfileOverProfileFile(t *testing.T) {
	setupProfileHome(t, map[string]string{
		"devbox/.gz-git.yaml": `profile: work
profiles:
  work:
    parallel: 4
`,
	})
	mgr, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveProfile(&Profile{Name: "work", Parallel: 9}); err != nil {
		t.Fatal(err)
	}

	effective, err := loadEffective(t)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if effective.Parallel != 4 {
		t.Errorf("Parallel = %d, want 4: the inline profile precedes the profile file", effective.Parallel)
	}
}

func TestLoadFallsBackToProfileFile(t *testing.T) {
	setupProfileHome(t, map[string]string{
		"devbox/.gz-git.yaml": "profile: work\n",
	})
	mgr, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveProfile(&Profile{Name: "work", Parallel: 9}); err != nil {
		t.Fatal(err)
	}

	effective, err := loadEffective(t)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if effective.Parallel != 9 {
		t.Errorf("Parallel = %d, want 9 from the profile file", effective.Parallel)
	}
}

func TestLoadMissingProfileNamesBothLocations(t *testing.T) {
	home := setupProfileHome(t, map[string]string{
		"devbox/.gz-git.yaml": "profile: absent\n",
	})

	_, err := loadEffective(t)
	if err == nil {
		t.Fatal("Load() error = nil, want profile not found")
	}
	msg := err.Error()
	for _, want := range []string{"'absent' not found", filepath.Join(home, "devbox", ".gz-git.yaml"), "profiles"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
}

func TestLoadFailsOnBrokenParent(t *testing.T) {
	setupProfileHome(t, map[string]string{
		"devbox/.gz-git.yaml": `parent: ../missing/.gz-git.yaml
profile: polypia
`,
	})

	if _, err := loadEffective(t); err == nil || !strings.Contains(err.Error(), "parent") {
		t.Fatalf("Load() error = %v, want the missing parent named", err)
	}
}
