// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultConfig verifies that DefaultConfig returns the expected default values.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Provider tokens start empty
	if cfg.GitHub.Token != "" {
		t.Errorf("GitHub.Token = %q, want empty", cfg.GitHub.Token)
	}
	if cfg.GitLab.Token != "" {
		t.Errorf("GitLab.Token = %q, want empty", cfg.GitLab.Token)
	}
	if cfg.Gitea.Token != "" {
		t.Errorf("Gitea.Token = %q, want empty", cfg.Gitea.Token)
	}

	// BaseURLs start empty
	if cfg.GitHub.BaseURL != "" {
		t.Errorf("GitHub.BaseURL = %q, want empty", cfg.GitHub.BaseURL)
	}
	if cfg.GitLab.BaseURL != "" {
		t.Errorf("GitLab.BaseURL = %q, want empty", cfg.GitLab.BaseURL)
	}
	if cfg.Gitea.BaseURL != "" {
		t.Errorf("Gitea.BaseURL = %q, want empty", cfg.Gitea.BaseURL)
	}

	// Sync defaults
	if cfg.Sync.TargetPath != "." {
		t.Errorf("Sync.TargetPath = %q, want %q", cfg.Sync.TargetPath, ".")
	}
	if cfg.Sync.Parallel != 4 {
		t.Errorf("Sync.Parallel = %d, want 4", cfg.Sync.Parallel)
	}
	if cfg.Sync.IncludeArchived {
		t.Error("Sync.IncludeArchived = true, want false")
	}
	if cfg.Sync.IncludeForks {
		t.Error("Sync.IncludeForks = true, want false")
	}
	if !cfg.Sync.IncludePrivate {
		t.Error("Sync.IncludePrivate = false, want true")
	}
}

// TestLoad_HappyPath tests that all YAML fields are correctly loaded.
func TestLoad_HappyPath(t *testing.T) {
	// Neutralize any provider tokens that may exist in the test environment
	// so that we can assert on the values loaded from the config file.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITEA_TOKEN", "")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := []byte(`
github:
  token: gh-token-123
  base_url: https://api.github.com
gitlab:
  token: gl-token-456
  base_url: https://gitlab.example.com
gitea:
  token: gt-token-789
  base_url: https://gitea.example.com
sync:
  target_path: /repos
  parallel: 8
  include_archived: true
  include_forks: true
  include_private: false
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GitHub.Token != "gh-token-123" {
		t.Errorf("GitHub.Token = %q, want %q", cfg.GitHub.Token, "gh-token-123")
	}
	if cfg.GitHub.BaseURL != "https://api.github.com" {
		t.Errorf("GitHub.BaseURL = %q, want %q", cfg.GitHub.BaseURL, "https://api.github.com")
	}
	if cfg.GitLab.Token != "gl-token-456" {
		t.Errorf("GitLab.Token = %q, want %q", cfg.GitLab.Token, "gl-token-456")
	}
	if cfg.GitLab.BaseURL != "https://gitlab.example.com" {
		t.Errorf("GitLab.BaseURL = %q, want %q", cfg.GitLab.BaseURL, "https://gitlab.example.com")
	}
	if cfg.Gitea.Token != "gt-token-789" {
		t.Errorf("Gitea.Token = %q, want %q", cfg.Gitea.Token, "gt-token-789")
	}
	if cfg.Gitea.BaseURL != "https://gitea.example.com" {
		t.Errorf("Gitea.BaseURL = %q, want %q", cfg.Gitea.BaseURL, "https://gitea.example.com")
	}
	if cfg.Sync.TargetPath != "/repos" {
		t.Errorf("Sync.TargetPath = %q, want %q", cfg.Sync.TargetPath, "/repos")
	}
	if cfg.Sync.Parallel != 8 {
		t.Errorf("Sync.Parallel = %d, want 8", cfg.Sync.Parallel)
	}
	if !cfg.Sync.IncludeArchived {
		t.Error("Sync.IncludeArchived = false, want true")
	}
	if !cfg.Sync.IncludeForks {
		t.Error("Sync.IncludeForks = false, want true")
	}
	if cfg.Sync.IncludePrivate {
		t.Error("Sync.IncludePrivate = true, want false")
	}
}

// TestLoad_FileNotFound tests that Load returns an error for a missing file.
func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Error("Load() expected error for missing file, got nil")
	}
}

// TestLoad_InvalidYAML tests that Load returns an error for malformed YAML.
func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")

	if err := os.WriteFile(configPath, []byte("github: [\ninvalid yaml"), 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for invalid YAML, got nil")
	}
}

// TestLoad_EmptyFile tests that an empty YAML file yields defaults unchanged.
func TestLoad_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yaml")

	if err := os.WriteFile(configPath, []byte{}, 0o600); err != nil {
		t.Fatalf("failed to write empty config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Defaults must survive an empty file
	if cfg.Sync.Parallel != 4 {
		t.Errorf("Sync.Parallel = %d, want default 4", cfg.Sync.Parallel)
	}
	if cfg.Sync.TargetPath != "." {
		t.Errorf("Sync.TargetPath = %q, want default %q", cfg.Sync.TargetPath, ".")
	}
	if !cfg.Sync.IncludePrivate {
		t.Error("Sync.IncludePrivate = false, want default true")
	}
}

// TestLoad_PartialConfig tests that unset YAML fields keep their DefaultConfig values.
func TestLoad_PartialConfig(t *testing.T) {
	// Neutralize any provider tokens from the environment.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITEA_TOKEN", "")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.yaml")

	content := []byte(`
github:
  token: my-token
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write partial config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GitHub.Token != "my-token" {
		t.Errorf("GitHub.Token = %q, want %q", cfg.GitHub.Token, "my-token")
	}
	// Unspecified fields must retain defaults
	if cfg.Sync.Parallel != 4 {
		t.Errorf("Sync.Parallel = %d, want default 4", cfg.Sync.Parallel)
	}
	if cfg.Sync.TargetPath != "." {
		t.Errorf("Sync.TargetPath = %q, want default %q", cfg.Sync.TargetPath, ".")
	}
	if !cfg.Sync.IncludePrivate {
		t.Error("Sync.IncludePrivate = false, want default true")
	}
}

// TestLoad_EnvOverrideTokens tests that GITHUB/GITLAB/GITEA_TOKEN env vars take
// precedence over the values in the config file.
func TestLoad_EnvOverrideTokens(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := []byte(`
github:
  token: file-gh-token
gitlab:
  token: file-gl-token
gitea:
  token: file-gt-token
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Setenv("GITHUB_TOKEN", "env-gh-token")
	t.Setenv("GITLAB_TOKEN", "env-gl-token")
	t.Setenv("GITEA_TOKEN", "env-gt-token")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GitHub.Token != "env-gh-token" {
		t.Errorf("GitHub.Token = %q, want env-gh-token (env should override file)", cfg.GitHub.Token)
	}
	if cfg.GitLab.Token != "env-gl-token" {
		t.Errorf("GitLab.Token = %q, want env-gl-token (env should override file)", cfg.GitLab.Token)
	}
	if cfg.Gitea.Token != "env-gt-token" {
		t.Errorf("Gitea.Token = %q, want env-gt-token (env should override file)", cfg.Gitea.Token)
	}
}

// TestLoad_EmptyEnvDoesNotClearFileToken tests that setting an env var to the
// empty string does not wipe the token that was read from the config file.
func TestLoad_EmptyEnvDoesNotClearFileToken(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := []byte(`
github:
  token: file-gh-token
gitlab:
  token: file-gl-token
gitea:
  token: file-gt-token
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITEA_TOKEN", "")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GitHub.Token != "file-gh-token" {
		t.Errorf("GitHub.Token = %q, want file-gh-token (empty env must not wipe file token)", cfg.GitHub.Token)
	}
	if cfg.GitLab.Token != "file-gl-token" {
		t.Errorf("GitLab.Token = %q, want file-gl-token (empty env must not wipe file token)", cfg.GitLab.Token)
	}
	if cfg.Gitea.Token != "file-gt-token" {
		t.Errorf("Gitea.Token = %q, want file-gt-token (empty env must not wipe file token)", cfg.Gitea.Token)
	}
}

// TestApplyEnvOverrides tests each token env var in isolation.
func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		getVal  func(*Config) string
		wantVal string
	}{
		{
			name:    "GITHUB_TOKEN",
			envKey:  "GITHUB_TOKEN",
			envVal:  "gh-only",
			getVal:  func(c *Config) string { return c.GitHub.Token },
			wantVal: "gh-only",
		},
		{
			name:    "GITLAB_TOKEN",
			envKey:  "GITLAB_TOKEN",
			envVal:  "gl-only",
			getVal:  func(c *Config) string { return c.GitLab.Token },
			wantVal: "gl-only",
		},
		{
			name:    "GITEA_TOKEN",
			envKey:  "GITEA_TOKEN",
			envVal:  "gt-only",
			getVal:  func(c *Config) string { return c.Gitea.Token },
			wantVal: "gt-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)

			cfg := DefaultConfig()
			cfg.applyEnvOverrides()

			if got := tt.getVal(cfg); got != tt.wantVal {
				t.Errorf("after %s=%s: token = %q, want %q", tt.envKey, tt.envVal, got, tt.wantVal)
			}
		})
	}
}

// TestApplyEnvOverrides_EmptyVarNoOp tests that an empty env var leaves the
// existing token value intact.
func TestApplyEnvOverrides_EmptyVarNoOp(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITEA_TOKEN", "")

	cfg := DefaultConfig()
	cfg.GitHub.Token = "preset-gh"
	cfg.GitLab.Token = "preset-gl"
	cfg.Gitea.Token = "preset-gt"
	cfg.applyEnvOverrides()

	if cfg.GitHub.Token != "preset-gh" {
		t.Errorf("GitHub.Token = %q, want preset-gh after empty env", cfg.GitHub.Token)
	}
	if cfg.GitLab.Token != "preset-gl" {
		t.Errorf("GitLab.Token = %q, want preset-gl after empty env", cfg.GitLab.Token)
	}
	if cfg.Gitea.Token != "preset-gt" {
		t.Errorf("Gitea.Token = %q, want preset-gt after empty env", cfg.Gitea.Token)
	}
}

// TestLoadDefault_NoConfigFile tests that LoadDefault returns defaults when no
// config file exists in any search location.
func TestLoadDefault_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}

	if cfg.Sync.Parallel != 4 {
		t.Errorf("Sync.Parallel = %d, want default 4", cfg.Sync.Parallel)
	}
	if cfg.Sync.TargetPath != "." {
		t.Errorf("Sync.TargetPath = %q, want default %q", cfg.Sync.TargetPath, ".")
	}
}

// TestLoadDefault_ForgeYaml tests that LoadDefault picks up forge.yaml in the
// current working directory.
func TestLoadDefault_ForgeYaml(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)

	content := []byte("sync:\n  parallel: 12\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "forge.yaml"), content, 0o600); err != nil {
		t.Fatalf("failed to write forge.yaml: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}

	if cfg.Sync.Parallel != 12 {
		t.Errorf("Sync.Parallel = %d, want 12 (from forge.yaml)", cfg.Sync.Parallel)
	}
}

// TestLoadDefault_DotForgeYaml tests that LoadDefault picks up .forge.yaml when
// forge.yaml is absent.
func TestLoadDefault_DotForgeYaml(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)

	content := []byte("sync:\n  parallel: 7\n")
	if err := os.WriteFile(filepath.Join(tmpDir, ".forge.yaml"), content, 0o600); err != nil {
		t.Fatalf("failed to write .forge.yaml: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}

	if cfg.Sync.Parallel != 7 {
		t.Errorf("Sync.Parallel = %d, want 7 (from .forge.yaml)", cfg.Sync.Parallel)
	}
}

// TestLoadDefault_HomeConfig tests that LoadDefault finds the config under
// $HOME/.config/gzh-forge/config.yaml when no local file is present.
func TestLoadDefault_HomeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)
	// Neutralize any provider tokens from the environment.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITEA_TOKEN", "")

	homeConfigDir := filepath.Join(tmpDir, ".config", "gzh-forge")
	if err := os.MkdirAll(homeConfigDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	content := []byte("github:\n  token: home-config-token\nsync:\n  parallel: 6\n")
	if err := os.WriteFile(filepath.Join(homeConfigDir, "config.yaml"), content, 0o600); err != nil {
		t.Fatalf("failed to write home config: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}

	if cfg.GitHub.Token != "home-config-token" {
		t.Errorf("GitHub.Token = %q, want home-config-token", cfg.GitHub.Token)
	}
	if cfg.Sync.Parallel != 6 {
		t.Errorf("Sync.Parallel = %d, want 6 (from home config)", cfg.Sync.Parallel)
	}
}

// TestLoadDefault_PrecedenceOrder tests that forge.yaml wins over .forge.yaml
// when both exist in the current directory.
func TestLoadDefault_PrecedenceOrder(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "forge.yaml"), []byte("sync:\n  parallel: 10\n"), 0o600); err != nil {
		t.Fatalf("failed to write forge.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".forge.yaml"), []byte("sync:\n  parallel: 20\n"), 0o600); err != nil {
		t.Fatalf("failed to write .forge.yaml: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}

	// forge.yaml is first in the search list; it must win
	if cfg.Sync.Parallel != 10 {
		t.Errorf("Sync.Parallel = %d, want 10 (forge.yaml has priority)", cfg.Sync.Parallel)
	}
}

// TestLoadDefault_EnvOverrideAppliedWithNoFile tests that env vars are applied
// even when LoadDefault falls back to the built-in defaults (no file found).
func TestLoadDefault_EnvOverrideAppliedWithNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)
	t.Setenv("GITHUB_TOKEN", "env-only-token")

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}

	if cfg.GitHub.Token != "env-only-token" {
		t.Errorf("GitHub.Token = %q, want env-only-token", cfg.GitHub.Token)
	}
}
