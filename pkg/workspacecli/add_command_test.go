// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/repo.git", "repo"},
		{"https://github.com/user/repo", "repo"},
		{"git@github.com:user/repo.git", "repo"},
		{"git@github.com:user/repo", "repo"},
		{"https://gitlab.com/group/subgroup/repo.git", "repo"},
		{"ssh://git@gitlab.com:2222/group/repo.git", "repo"},
		{"/local/path/to/repo.git", "repo"},
		{"/local/path/to/repo", "repo"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractRepoName(tt.url)
			if got != tt.want {
				t.Errorf("extractRepoName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestLoadOrCreateConfig_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "new-config.yaml")

	config, err := loadOrCreateConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check default values
	if config["strategy"] != "reset" {
		t.Errorf("strategy = %v, want reset", config["strategy"])
	}

	if config["parallel"] != 4 {
		t.Errorf("parallel = %v, want 4", config["parallel"])
	}

	repos, ok := config["repositories"].([]interface{})
	if !ok {
		t.Error("repositories should be a slice")
	}

	if len(repos) != 0 {
		t.Errorf("new config should have empty repositories, got %d", len(repos))
	}
}

func TestLoadOrCreateConfig_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "existing-config.yaml")

	existing := `
strategy: pull
parallel: 8
repositories:
  - name: existing-repo
    url: https://github.com/test/repo.git
    targetPath: ./repos/existing
`
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := loadOrCreateConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config["strategy"] != "pull" {
		t.Errorf("strategy = %v, want pull", config["strategy"])
	}

	if config["parallel"] != 8 {
		t.Errorf("parallel = %v, want 8", config["parallel"])
	}
}

func TestLoadOrCreateConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-config.yaml")

	if err := os.WriteFile(configPath, []byte(`invalid: [yaml`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadOrCreateConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestWriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "output.yaml")

	config := map[string]interface{}{
		"strategy": "reset",
		"parallel": 4,
		"repositories": []interface{}{
			map[string]interface{}{
				"name":       "test-repo",
				"url":        "https://github.com/test/repo.git",
				"targetPath": "./repos/test",
			},
		},
	}

	if err := writeConfig(configPath, config); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var loaded map[string]interface{}
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}

	if loaded["strategy"] != "reset" {
		t.Errorf("strategy = %v, want reset", loaded["strategy"])
	}
}

func TestAddOptions_DuplicateDetection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config with existing repo
	config := map[string]interface{}{
		"strategy": "reset",
		"parallel": 4,
		"repositories": []interface{}{
			map[string]interface{}{
				"name":       "existing",
				"url":        "https://github.com/test/existing.git",
				"targetPath": "./repos/existing",
			},
		},
	}

	if err := writeConfig(configPath, config); err != nil {
		t.Fatal(err)
	}

	// Load and check for duplicate
	loaded, err := loadOrCreateConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}

	repos := loaded["repositories"].([]interface{})
	newTargetPath := "./repos/existing"

	// Check duplicate detection logic
	for _, r := range repos {
		if rm, ok := r.(map[string]interface{}); ok {
			if rm["targetPath"] == newTargetPath {
				// This is the expected behavior
				return
			}
		}
	}

	t.Error("should have detected duplicate targetPath")
}
