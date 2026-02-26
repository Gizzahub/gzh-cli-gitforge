// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectConfigFile(t *testing.T) {
	t.Run("finds .gz-git.yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".gz-git.yaml")

		if err := os.WriteFile(configPath, []byte("# test"), 0o644); err != nil {
			t.Fatal(err)
		}

		found, err := DetectConfigFile(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found != configPath {
			t.Errorf("got %s, want %s", found, configPath)
		}
	})

	t.Run("finds .gz-git.yml fallback", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".gz-git.yml")

		if err := os.WriteFile(configPath, []byte("# test"), 0o644); err != nil {
			t.Fatal(err)
		}

		found, err := DetectConfigFile(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found != configPath {
			t.Errorf("got %s, want %s", found, configPath)
		}
	})

	t.Run("prefers .gz-git.yaml over .yml", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ".gz-git.yaml")
		ymlPath := filepath.Join(tmpDir, ".gz-git.yml")

		if err := os.WriteFile(yamlPath, []byte("# yaml"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ymlPath, []byte("# yml"), 0o644); err != nil {
			t.Fatal(err)
		}

		found, err := DetectConfigFile(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found != yamlPath {
			t.Errorf("got %s, want %s", found, yamlPath)
		}
	})

	t.Run("returns error when no config found", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := DetectConfigFile(tmpDir)
		if err == nil {
			t.Error("expected error when no config file found")
		}
	})

	t.Run("uses constant for default filename", func(t *testing.T) {
		if ProjectConfigFileName != ".gz-git" {
			t.Errorf("ProjectConfigFileName = %q, want %q", ProjectConfigFileName, ".gz-git")
		}
	})
}

func TestNewPaths(t *testing.T) {
	// Ensure HOME is set for the test
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	paths, err := NewPaths()
	if err != nil {
		t.Fatalf("NewPaths() error: %v", err)
	}

	if paths.ConfigDir == "" {
		t.Error("ConfigDir should not be empty")
	}

	if paths.ProfilesDir == "" {
		t.Error("ProfilesDir should not be empty")
	}

	if paths.StateDir == "" {
		t.Error("StateDir should not be empty")
	}
}

func TestPaths_ProfilePath(t *testing.T) {
	// Ensure HOME is set for the test
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	paths, err := NewPaths()
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(paths.ProfilesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(paths.ProfilesDir, "work.yaml"), []byte(""), 0o644)

	profilePath := paths.ProfilePath("work")

	if filepath.Base(profilePath) != "work.yaml" {
		t.Errorf("ProfilePath(work) should end with work.yaml, got %s", profilePath)
	}
}

func TestPaths_EnsureDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	paths := &Paths{
		ConfigDir:         filepath.Join(tmpDir, "config"),
		ProfilesDir:       filepath.Join(tmpDir, "config", "profiles"),
		StateDir:          filepath.Join(tmpDir, "config", "state"),
		GlobalConfigFile:  filepath.Join(tmpDir, "config", "config.yaml"),
		ActiveProfileFile: filepath.Join(tmpDir, "config", "state", "active-profile.txt"),
	}

	if err := paths.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories() error: %v", err)
	}

	// Verify directories exist
	if _, err := os.Stat(paths.ConfigDir); os.IsNotExist(err) {
		t.Error("ConfigDir was not created")
	}

	if _, err := os.Stat(paths.ProfilesDir); os.IsNotExist(err) {
		t.Error("ProfilesDir was not created")
	}

	if _, err := os.Stat(paths.StateDir); os.IsNotExist(err) {
		t.Error("StateDir was not created")
	}
}

func TestPaths_Exists(t *testing.T) {
	t.Run("returns false for non-existent", func(t *testing.T) {
		paths := &Paths{
			ConfigDir: "/nonexistent/path",
		}

		if paths.Exists() {
			t.Error("Exists() should return false for non-existent directory")
		}
	})

	t.Run("returns true for existing", func(t *testing.T) {
		tmpDir := t.TempDir()
		paths := &Paths{
			ConfigDir: tmpDir,
		}

		if !paths.Exists() {
			t.Error("Exists() should return true for existing directory")
		}
	})
}

func TestPaths_ProfileExists(t *testing.T) {
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	os.MkdirAll(profilesDir, 0o755)

	paths := &Paths{
		ProfilesDir: profilesDir,
	}

	// Create a profile
	profilePath := filepath.Join(profilesDir, "work.yaml")
	os.WriteFile(profilePath, []byte("# test"), 0o644)

	t.Run("returns true for existing profile", func(t *testing.T) {
		if !paths.ProfileExists("work") {
			t.Error("ProfileExists should return true")
		}
	})

	t.Run("returns false for non-existing profile", func(t *testing.T) {
		if paths.ProfileExists("nonexistent") {
			t.Error("ProfileExists should return false")
		}
	})
}

func TestPaths_ListProfiles(t *testing.T) {
	t.Run("lists profiles", func(t *testing.T) {
		tmpDir := t.TempDir()
		profilesDir := filepath.Join(tmpDir, "profiles")
		os.MkdirAll(profilesDir, 0o755)

		// Create profiles
		os.WriteFile(filepath.Join(profilesDir, "work.yaml"), []byte("# work"), 0o644)
		os.WriteFile(filepath.Join(profilesDir, "personal.yaml"), []byte("# personal"), 0o644)
		os.WriteFile(filepath.Join(profilesDir, "not-yaml.txt"), []byte("# txt"), 0o644)

		paths := &Paths{ProfilesDir: profilesDir}
		profiles, err := paths.ListProfiles()
		if err != nil {
			t.Fatal(err)
		}

		if len(profiles) != 2 {
			t.Errorf("expected 2 profiles, got %d", len(profiles))
		}
	})

	t.Run("returns empty for non-existent dir", func(t *testing.T) {
		paths := &Paths{ProfilesDir: "/nonexistent"}
		profiles, err := paths.ListProfiles()
		if err != nil {
			t.Fatal(err)
		}

		if len(profiles) != 0 {
			t.Errorf("expected 0 profiles, got %d", len(profiles))
		}
	})
}
