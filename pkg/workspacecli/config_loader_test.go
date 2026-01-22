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
    targetPath: ./repos/test-repo
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
    targetPath: ./repos/repo1
  - name: repo2
    url: git@github.com:test/repo2.git
    targetPath: ./repos/repo2
`,
			wantRepos: 2,
			wantErr:   false,
		},
		{
			name: "valid config with urls array",
			yaml: `
strategy: fetch
repositories:
  - name: multi-remote
    urls:
      - https://github.com/test/repo.git
      - git@github.com:test/repo.git
    targetPath: ./repos/multi
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
    targetPath: ./repos/repo1
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
    targetPath: ./repos/repo1
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
    targetPath: ./repos/repo1
`,
			wantRepos: 1,
			wantErr:   false,
		},
		{
			name: "missing targetPath defaults to repo name",
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
    targetPath: ./test
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
	if result.Run.Parallel != 4 {
		t.Errorf("default parallel = %d, want 4", result.Run.Parallel)
	}

	// Check default maxRetries
	if result.Run.MaxRetries != 3 {
		t.Errorf("default maxRetries = %d, want 3", result.Run.MaxRetries)
	}
}

func TestFileSpecLoader_Load_MissingTargetPath(t *testing.T) {
	yaml := `
strategy: reset
repositories:
  - name: my-project
    url: https://github.com/test/my-project.git
  - name: another-repo
    url: https://github.com/test/another.git
    targetPath: ./custom/path
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
		t.Errorf("first repo targetPath = %q, want %q",
			result.Plan.Input.Repos[0].TargetPath, "my-project")
	}

	// Check second repo uses explicit path
	if result.Plan.Input.Repos[1].TargetPath != "./custom/path" {
		t.Errorf("second repo targetPath = %q, want %q",
			result.Plan.Input.Repos[1].TargetPath, "./custom/path")
	}
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
