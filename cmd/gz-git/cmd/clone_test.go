package cmd

import (
	"os"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		// HTTPS URLs
		{
			name: "HTTPS URL with .git suffix",
			url:  "https://github.com/user/repo.git",
			want: "repo",
		},
		{
			name: "HTTPS URL without .git suffix",
			url:  "https://github.com/user/repo",
			want: "repo",
		},
		{
			name: "HTTPS URL with organization",
			url:  "https://github.com/organization/project-name.git",
			want: "project-name",
		},

		// SSH URLs
		{
			name: "SSH URL with .git suffix",
			url:  "git@github.com:user/repo.git",
			want: "repo",
		},
		{
			name: "SSH URL without .git suffix",
			url:  "git@github.com:user/repo",
			want: "repo",
		},
		{
			name: "SSH URL with organization",
			url:  "git@github.com:organization/my-project.git",
			want: "my-project",
		},
		{
			name: "SSH URL with GitLab",
			url:  "git@gitlab.com:group/subgroup/repo.git",
			want: "repo",
		},

		// Git protocol URLs
		{
			name: "Git protocol URL",
			url:  "git://github.com/user/repo.git",
			want: "repo",
		},

		// File paths
		{
			name: "Unix file path",
			url:  "/home/user/projects/my-repo",
			want: "my-repo",
		},
		{
			name: "Unix file path with .git suffix",
			url:  "/path/to/repo.git",
			want: "repo",
		},
		// Note: Windows paths are not valid Git URLs
		// The utility function ExtractRepoNameFromURL is designed for Git URLs only

		// Edge cases
		{
			name: "Empty URL returns default",
			url:  "",
			want: "repository",
		},
		{
			name: "Only .git suffix",
			url:  ".git",
			want: "repository",
		},
		{
			name: "Simple name",
			url:  "repo",
			want: "repo",
		},
		{
			name: "URL with trailing slash should still work",
			url:  "https://github.com/user/repo.git",
			want: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := repository.ExtractRepoNameFromURL(tt.url)
			if got == "" {
				got = "repository"
			}
			if got != tt.want {
				t.Errorf("ExtractRepoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractRepoName_SSHVariants(t *testing.T) {
	// Specific test cases for SSH URL parsing
	tests := []struct {
		url  string
		want string
	}{
		// Standard SSH format
		{"git@github.com:user/repo.git", "repo"},
		{"git@github.com:user/repo", "repo"},

		// Custom SSH hosts
		{"git@myserver.com:projects/repo.git", "repo"},
		{"git@192.168.1.1:user/repo.git", "repo"},

		// Nested paths
		{"git@gitlab.com:group/subgroup/repo.git", "repo"},
		{"git@gitlab.com:a/b/c/deep-repo.git", "deep-repo"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got, _ := repository.ExtractRepoNameFromURL(tt.url)
			if got == "" {
				got = "repository"
			}
			if got != tt.want {
				t.Errorf("ExtractRepoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
// ============================================================================
// YAML Config Tests
// ============================================================================

func TestParseCloneConfig_ValidYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantErr  bool
		validate func(*testing.T, *CloneConfig)
	}{
		{
			name: "minimal config",
			yaml: `
repositories:
  - url: https://github.com/user/repo.git
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *CloneConfig) {
				if len(cfg.Repositories) != 1 {
					t.Errorf("expected 1 repository, got %d", len(cfg.Repositories))
				}
				if cfg.Repositories[0].URL != "https://github.com/user/repo.git" {
					t.Errorf("unexpected URL: %s", cfg.Repositories[0].URL)
				}
			},
		},
		{
			name: "full config with custom names",
			yaml: `
target: ~/projects
parallel: 5
structure: flat
update: true
repositories:
  - url: https://github.com/user/repo1.git
    name: custom-name-1
    branch: develop
    depth: 1
  - url: https://github.com/user/repo2.git
    name: custom-name-2
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *CloneConfig) {
				if cfg.Target != "~/projects" {
					t.Errorf("expected target ~/projects, got %s", cfg.Target)
				}
				if cfg.Parallel != 5 {
					t.Errorf("expected parallel 5, got %d", cfg.Parallel)
				}
				if cfg.Structure != "flat" {
					t.Errorf("expected structure flat, got %s", cfg.Structure)
				}
				if !cfg.Update {
					t.Error("expected update true")
				}
				if len(cfg.Repositories) != 2 {
					t.Fatalf("expected 2 repositories, got %d", len(cfg.Repositories))
				}
				if cfg.Repositories[0].Name != "custom-name-1" {
					t.Errorf("expected name custom-name-1, got %s", cfg.Repositories[0].Name)
				}
				if cfg.Repositories[0].Branch != "develop" {
					t.Errorf("expected branch develop, got %s", cfg.Repositories[0].Branch)
				}
				if cfg.Repositories[0].Depth != 1 {
					t.Errorf("expected depth 1, got %d", cfg.Repositories[0].Depth)
				}
			},
		},
		{
			name: "user structure",
			yaml: `
structure: user
repositories:
  - url: https://github.com/user/repo.git
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *CloneConfig) {
				if cfg.Structure != "user" {
					t.Errorf("expected structure user, got %s", cfg.Structure)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with YAML content
			tmpfile, err := os.CreateTemp("", "clone-config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.yaml)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			// Parse config
			cfg, err := parseCloneConfig(tmpfile.Name(), false)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCloneConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestParseCloneConfig_InvalidYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "invalid YAML syntax",
			yaml: `
repositories:
  - url: https://github.com/user/repo.git
    invalid syntax here
`,
			wantErr: "parse YAML",
		},
		{
			name: "no repositories",
			yaml: `
target: ~/projects
`,
			wantErr: "no repositories defined",
		},
		{
			name: "missing URL",
			yaml: `
repositories:
  - name: repo-name
`,
			wantErr: "missing URL",
		},
		{
			name: "invalid structure",
			yaml: `
structure: invalid-value
repositories:
  - url: https://github.com/user/repo.git
`,
			wantErr: "invalid structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "clone-config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.yaml)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			_, err = parseCloneConfig(tmpfile.Name(), false)
			if err == nil {
				t.Errorf("parseCloneConfig() expected error containing %q, got nil", tt.wantErr)
				return
			}

			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("parseCloneConfig() error = %q, want error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateCloneConfig_DuplicateNames(t *testing.T) {
	config := &CloneConfig{
		Repositories: []CloneRepoSpec{
			{
				URL:  "https://github.com/user/repo1.git",
				Name: "my-repo",
			},
			{
				URL:  "https://github.com/user/repo2.git",
				Name: "my-repo", // Duplicate!
			},
		},
	}

	err := validateCloneConfig(config)
	if err == nil {
		t.Error("validateCloneConfig() expected error for duplicate names, got nil")
	}

	if !containsString(err.Error(), "duplicate name") {
		t.Errorf("validateCloneConfig() error = %q, want error containing 'duplicate name'", err.Error())
	}
}

func TestValidateCloneConfig_DuplicateExtractedNames(t *testing.T) {
	// Two different URLs that extract to the same repo name
	config := &CloneConfig{
		Repositories: []CloneRepoSpec{
			{
				URL: "https://github.com/user1/repo.git",
				// Name not specified, will extract "repo"
			},
			{
				URL: "https://github.com/user2/repo.git",
				// Name not specified, will also extract "repo"
			},
		},
	}

	err := validateCloneConfig(config)
	if err == nil {
		t.Error("validateCloneConfig() expected error for duplicate extracted names, got nil")
	}

	if !containsString(err.Error(), "duplicate name") {
		t.Errorf("validateCloneConfig() error = %q, want error containing 'duplicate name'", err.Error())
	}
}

func TestParseCloneConfig_Stdin(t *testing.T) {
	yaml := `
repositories:
  - url: https://github.com/user/repo.git
    name: test-repo
`

	// Save current stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create pipe for stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Write YAML to stdin
	go func() {
		defer w.Close()
		w.Write([]byte(yaml))
	}()

	// Parse from stdin
	cfg, err := parseCloneConfig("", true)
	if err != nil {
		t.Errorf("parseCloneConfig() from stdin error = %v", err)
		return
	}

	if len(cfg.Repositories) != 1 {
		t.Errorf("expected 1 repository, got %d", len(cfg.Repositories))
	}
	if cfg.Repositories[0].Name != "test-repo" {
		t.Errorf("expected name test-repo, got %s", cfg.Repositories[0].Name)
	}
}

// Helper function for string contains check
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && s[:len(substr)] == substr || stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
