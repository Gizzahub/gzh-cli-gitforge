// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSpecLoader_Load(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantRepos int
		wantErr   bool
	}{
		{
			name: "valid config with single repo",
			yaml: `
strategy: reset
parallel: 4
maxRetries: 3
repositories:
  - name: test-repo
    url: https://github.com/test/repo.git
    path: ./repos/test-repo
`,
			wantRepos: 1,
			wantErr:   false,
		},
		{
			name: "valid config with multiple repos",
			yaml: `
strategy: pull
parallel: 8
maxRetries: 5
repositories:
  - name: repo1
    url: https://github.com/test/repo1.git
    path: ./repos/repo1
  - name: repo2
    url: git@github.com:test/repo2.git
    path: ./repos/repo2
`,
			wantRepos: 2,
			wantErr:   false,
		},
		{
			name: "valid config with additionalRemotes",
			yaml: `
strategy: fetch
repositories:
  - name: multi-remote
    url: https://github.com/test/repo.git
    additionalRemotes:
      upstream: git@github.com:original/repo.git
      backup: git@gitlab.com:test/repo.git
    path: ./repos/multi
`,
			wantRepos: 1,
			wantErr:   false,
		},
		{
			name: "valid config with per-repo strategy",
			yaml: `
strategy: reset
repositories:
  - name: repo1
    url: https://github.com/test/repo1.git
    path: ./repos/repo1
    strategy: pull
`,
			wantRepos: 1,
			wantErr:   false,
		},
		{
			name:      "invalid yaml syntax",
			yaml:      `strategy: [invalid`,
			wantRepos: 0,
			wantErr:   true,
		},
		{
			name: "invalid strategy value",
			yaml: `
strategy: invalid-strategy
repositories: []
`,
			wantRepos: 0,
			wantErr:   true,
		},
		{
			name: "invalid per-repo strategy",
			yaml: `
strategy: reset
repositories:
  - name: repo1
    url: https://github.com/test/repo.git
    path: ./repos/repo1
    strategy: not-valid
`,
			wantRepos: 0,
			wantErr:   true,
		},
		{
			name: "empty repositories",
			yaml: `
strategy: reset
repositories: []
`,
			wantRepos: 0,
			wantErr:   false,
		},
		{
			name: "with roots specified",
			yaml: `
strategy: reset
roots:
  - ~/repos
  - /opt/git
repositories:
  - name: repo1
    url: https://github.com/test/repo.git
    path: ./repos/repo1
`,
			wantRepos: 1,
			wantErr:   false,
		},
		{
			name: "missing path defaults to repo name",
			yaml: `
strategy: reset
repositories:
  - name: my-project
    url: https://github.com/test/my-project.git
  - name: another-repo
    url: https://github.com/test/another.git
`,
			wantRepos: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.yaml), 0o644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			loader := FileSpecLoader{}
			result, err := loader.Load(context.Background(), configPath)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Plan.Input.Repos) != tt.wantRepos {
				t.Errorf("got %d repos, want %d", len(result.Plan.Input.Repos), tt.wantRepos)
			}
		})
	}
}

func TestFileSpecLoader_Load_FileNotFound(t *testing.T) {
	loader := FileSpecLoader{}
	_, err := loader.Load(context.Background(), "/nonexistent/path/config.yaml")

	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestFileSpecLoader_Load_DefaultValues(t *testing.T) {
	yaml := `
repositories:
  - name: test
    url: https://github.com/test/repo.git
    path: ./test
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loader := FileSpecLoader{}
	result, err := loader.Load(context.Background(), configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check default parallel
	if result.Run.Parallel != 10 {
		t.Errorf("default parallel = %d, want 10", result.Run.Parallel)
	}

	// Check default maxRetries
	if result.Run.MaxRetries != 3 {
		t.Errorf("default maxRetries = %d, want 3", result.Run.MaxRetries)
	}
}

func TestFileSpecLoader_Load_MissingPath(t *testing.T) {
	yaml := `
strategy: reset
repositories:
  - name: my-project
    url: https://github.com/test/my-project.git
  - name: another-repo
    url: https://github.com/test/another.git
    path: ./custom/path
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loader := FileSpecLoader{}
	result, err := loader.Load(context.Background(), configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Plan.Input.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(result.Plan.Input.Repos))
	}

	// Check first repo defaults to name
	if result.Plan.Input.Repos[0].TargetPath != "my-project" {
		t.Errorf("first repo path = %q, want %q",
			result.Plan.Input.Repos[0].TargetPath, "my-project")
	}

	// Check second repo uses explicit path
	if result.Plan.Input.Repos[1].TargetPath != "./custom/path" {
		t.Errorf("second repo path = %q, want %q",
			result.Plan.Input.Repos[1].TargetPath, "./custom/path")
	}
}

func TestFileSpecLoader_Load_NameExtractedFromURL(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantRepos    int
		wantNames    []string
		wantPaths    []string
		wantErr      bool
		wantErrMatch string
	}{
		{
			name: "name omitted - extract from HTTPS URL",
			yaml: `
repositories:
  - url: https://github.com/test/my-repo.git
`,
			wantRepos: 1,
			wantNames: []string{"my-repo"},
			wantPaths: []string{"my-repo"},
			wantErr:   false,
		},
		{
			name: "name omitted - extract from SSH URL",
			yaml: `
repositories:
  - url: git@github.com:test/another-repo.git
`,
			wantRepos: 1,
			wantNames: []string{"another-repo"},
			wantPaths: []string{"another-repo"},
			wantErr:   false,
		},
		{
			name: "name omitted with custom path",
			yaml: `
repositories:
  - url: https://github.com/test/my-repo.git
    path: ./custom/location
`,
			wantRepos: 1,
			wantNames: []string{"my-repo"},
			wantPaths: []string{"./custom/location"},
			wantErr:   false,
		},
		{
			name: "name provided - use explicit name",
			yaml: `
repositories:
  - name: custom-name
    url: https://github.com/test/original-name.git
`,
			wantRepos: 1,
			wantNames: []string{"custom-name"},
			wantPaths: []string{"custom-name"},
			wantErr:   false,
		},
		{
			name: "mixed - some with name, some without",
			yaml: `
repositories:
  - url: https://github.com/test/auto-extracted.git
  - name: explicit-name
    url: https://github.com/test/other-repo.git
`,
			wantRepos: 2,
			wantNames: []string{"auto-extracted", "explicit-name"},
			wantPaths: []string{"auto-extracted", "explicit-name"},
			wantErr:   false,
		},
		{
			name: "missing URL - error",
			yaml: `
repositories:
  - name: no-url-repo
`,
			wantErr:      true,
			wantErrMatch: "missing URL",
		},
		{
			name: "SSH URL with port",
			yaml: `
repositories:
  - url: ssh://git@gitlab.example.com:2224/group/project-name.git
`,
			wantRepos: 1,
			wantNames: []string{"project-name"},
			wantPaths: []string{"project-name"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.yaml), 0o644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			loader := FileSpecLoader{}
			result, err := loader.Load(context.Background(), configPath)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.wantErrMatch != "" && !contains(err.Error(), tt.wantErrMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrMatch)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Plan.Input.Repos) != tt.wantRepos {
				t.Errorf("got %d repos, want %d", len(result.Plan.Input.Repos), tt.wantRepos)
				return
			}

			for i, wantName := range tt.wantNames {
				if result.Plan.Input.Repos[i].Name != wantName {
					t.Errorf("repo[%d].Name = %q, want %q",
						i, result.Plan.Input.Repos[i].Name, wantName)
				}
			}

			for i, wantPath := range tt.wantPaths {
				if result.Plan.Input.Repos[i].TargetPath != wantPath {
					t.Errorf("repo[%d].TargetPath = %q, want %q",
						i, result.Plan.Input.Repos[i].TargetPath, wantPath)
				}
			}
		})
	}
}

// contains checks if substr is in s (helper for error matching)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDetectConfigFile(t *testing.T) {
	t.Run("finds .gz-git.yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".gz-git.yaml")

		if err := os.WriteFile(configPath, []byte("# test"), 0o644); err != nil {
			t.Fatal(err)
		}

		found, err := detectConfigFile(tmpDir)
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

		found, err := detectConfigFile(tmpDir)
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

		found, err := detectConfigFile(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found != yamlPath {
			t.Errorf("got %s, want %s", found, yamlPath)
		}
	})

	t.Run("returns error when no config found", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := detectConfigFile(tmpDir)
		if err == nil {
			t.Error("expected error when no config file found")
		}
	})
}
