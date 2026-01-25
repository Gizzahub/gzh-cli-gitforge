// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewInitCmd(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	if cmd.Use != "init [path]" {
		t.Errorf("Use = %q, want %q", cmd.Use, "init [path]")
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

func TestInitCmd_NoArgs_ShowsGuide(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should show usage guide
	if !strings.Contains(output, "Workspace Init") {
		t.Error("should show workspace init header")
	}

	if !strings.Contains(output, "gz-git workspace init .") {
		t.Error("should show example usage")
	}

	if !strings.Contains(output, "--scan-depth") {
		t.Error("should show options")
	}
}

func TestInitCmd_Template_CreatesEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{tmpDir, "--template"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check file was created
	configPath := filepath.Join(tmpDir, DefaultConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Check content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	// Default kind is now 'workspace', so expect workspaces section
	if !strings.Contains(string(content), "kind: workspace") {
		t.Error("config should contain kind: workspace")
	}

	if !strings.Contains(string(content), "workspaces:") {
		t.Error("config should contain workspaces section (default kind is workspace)")
	}
}

func TestInitCmd_CustomOutput(t *testing.T) {
	tmpDir := t.TempDir()
	customName := "custom-workspace.yaml"

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{tmpDir, "--template", "-o", customName})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(tmpDir, customName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("custom config file was not created")
	}
}

func TestInitCmd_ExistingFile_ShowsGuidance(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, DefaultConfigFile)

	// Create existing file
	if err := os.WriteFile(configPath, []byte("# existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)

	cmd.SetArgs([]string{tmpDir, "--template"})
	err := cmd.Execute()
	// Should NOT return error, just show guidance
	if err != nil {
		t.Errorf("should not return error, got: %v", err)
	}

	output := outBuf.String()

	// Should show file exists message
	if !strings.Contains(output, "already exists") {
		t.Error("should mention file already exists")
	}

	// Should suggest --force
	if !strings.Contains(output, "--force") {
		t.Error("should suggest --force option")
	}

	// Verify file was NOT overwritten
	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "# existing") {
		t.Error("existing file should not have been modified")
	}
}

func TestInitCmd_Force_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, DefaultConfigFile)

	// Create existing file
	if err := os.WriteFile(configPath, []byte("# old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{tmpDir, "--template", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check file was overwritten
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(content), "old content") {
		t.Error("file should have been overwritten")
	}

	// Default kind is now 'workspace'
	if !strings.Contains(string(content), "workspaces:") {
		t.Error("new config should contain workspaces section (default kind is workspace)")
	}
}

func TestInitCmd_Scan_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should report no repos found
	if !strings.Contains(output, "No git repositories found") {
		t.Error("should report no repos found")
	}

	// Should suggest --template
	if !strings.Contains(output, "--template") {
		t.Error("should suggest --template option")
	}
}

func TestInitCmd_Flags(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	tests := []struct {
		name      string
		shorthand string
	}{
		{"output", "o"},
		{"scan-depth", "d"},
		{"force", "f"},
		{"exclude", ""},
		{"template", ""},
		{"no-gitignore", ""},
	}

	for _, tt := range tests {
		flag := cmd.Flags().Lookup(tt.name)
		if flag == nil {
			t.Errorf("flag %q should exist", tt.name)
			continue
		}

		if tt.shorthand != "" && flag.Shorthand != tt.shorthand {
			t.Errorf("flag %q shorthand = %q, want %q", tt.name, flag.Shorthand, tt.shorthand)
		}
	}
}

// TestNormalizeKind tests all kind normalization cases.
func TestNormalizeKind(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantKind    ConfigKind
		wantWarning bool
		wantError   bool
	}{
		// Valid kinds
		{
			name:     "workspace",
			input:    "workspace",
			wantKind: KindWorkspace,
		},
		{
			name:     "repositories",
			input:    "repositories",
			wantKind: KindRepositories,
		},
		// Deprecated aliases (should work with warning)
		{
			name:        "workspaces - deprecated alias",
			input:       "workspaces",
			wantKind:    KindWorkspace,
			wantWarning: true,
		},
		{
			name:        "repository - deprecated alias",
			input:       "repository",
			wantKind:    KindRepositories,
			wantWarning: true,
		},
		// Error cases
		{
			name:      "empty string",
			input:     "",
			wantKind:  "",
			wantError: true,
		},
		{
			name:      "invalid value",
			input:     "invalid",
			wantKind:  "",
			wantError: true,
		},
		{
			name:      "typo - repos",
			input:     "repos",
			wantKind:  "",
			wantError: true,
		},
		{
			name:      "typo - work",
			input:     "work",
			wantKind:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, warning, err := NormalizeKind(tt.input)

			// Check error expectation
			if tt.wantError {
				if err == nil {
					t.Errorf("NormalizeKind(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("NormalizeKind(%q) unexpected error: %v", tt.input, err)
				return
			}

			// Check kind
			if kind != tt.wantKind {
				t.Errorf("NormalizeKind(%q) kind = %q, want %q", tt.input, kind, tt.wantKind)
			}

			// Check warning
			if tt.wantWarning {
				if warning == "" {
					t.Errorf("NormalizeKind(%q) expected warning, got empty", tt.input)
				}
			} else {
				if warning != "" {
					t.Errorf("NormalizeKind(%q) unexpected warning: %q", tt.input, warning)
				}
			}
		})
	}
}

// TestIsValidStrategy tests strategy validation.
func TestIsValidStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		want     bool
	}{
		{"reset", true},
		{"pull", true},
		{"fetch", true},
		{"skip", true},
		{"", false},
		{"invalid", false},
		{"rebase", false},
		{"merge", false},
	}

	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			got := isValidStrategy(tt.strategy)
			if got != tt.want {
				t.Errorf("isValidStrategy(%q) = %v, want %v", tt.strategy, got, tt.want)
			}
		})
	}
}

// TestInitCmd_KindFlag tests --kind flag behavior.
func TestInitCmd_KindFlag(t *testing.T) {
	tests := []struct {
		name         string
		kind         string
		expectInFile string
	}{
		{
			name:         "workspace kind",
			kind:         "workspace",
			expectInFile: "workspaces:",
		},
		{
			name:         "repositories kind",
			kind:         "repositories",
			expectInFile: "repositories:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			factory := CommandFactory{}
			cmd := factory.newInitCmd()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			cmd.SetArgs([]string{tmpDir, "--template", "--kind", tt.kind})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			configPath := filepath.Join(tmpDir, DefaultConfigFile)
			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(string(content), tt.expectInFile) {
				t.Errorf("config should contain %q, got:\n%s", tt.expectInFile, string(content))
			}
		})
	}
}

// TestInitCmd_StrategyFlag tests --strategy flag behavior.
// Note: Strategy is only included in repositories-sample template, not workspace-workstation.
func TestInitCmd_StrategyFlag(t *testing.T) {
	tmpDir := t.TempDir()

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Use --kind repositories to get repositories-sample template which includes strategy
	cmd.SetArgs([]string{tmpDir, "--template", "--kind", "repositories", "--strategy", "pull"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(tmpDir, DefaultConfigFile)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	// repositories-sample.yaml has strategy field
	if !strings.Contains(string(content), "strategy:") {
		t.Errorf("config should contain 'strategy:', got:\n%s", string(content))
	}
}

// TestInitCmd_InvalidStrategy tests invalid strategy handling.
func TestInitCmd_InvalidStrategy(t *testing.T) {
	tmpDir := t.TempDir()

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{tmpDir, "--template", "--strategy", "invalid"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid strategy")
	}

	if !strings.Contains(err.Error(), "invalid strategy") {
		t.Errorf("error should mention invalid strategy: %v", err)
	}
}

// TestInitCmd_ScanWithGitRepos tests init with actual git repos.
func TestInitCmd_ScanWithGitRepos(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock git repos
	for _, name := range []string{"repo1", "repo2"} {
		repoPath := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		// Create minimal git config
		gitConfig := filepath.Join(repoPath, ".git", "config")
		configContent := `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = https://github.com/test/` + name + `.git
`
		if err := os.WriteFile(gitConfig, []byte(configContent), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name         string
		kind         string
		expectInFile string
	}{
		{
			name:         "workspace kind scan",
			kind:         "workspace",
			expectInFile: "workspaces:",
		},
		{
			name:         "repositories kind scan",
			kind:         "repositories",
			expectInFile: "repositories:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanDir := t.TempDir()

			// Create mock git repos in scan directory
			for _, name := range []string{"repo1", "repo2"} {
				repoPath := filepath.Join(scanDir, name)
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
				gitConfig := filepath.Join(repoPath, ".git", "config")
				configContent := `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = https://github.com/test/` + name + `.git
`
				if err := os.WriteFile(gitConfig, []byte(configContent), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			factory := CommandFactory{}
			cmd := factory.newInitCmd()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			cmd.SetArgs([]string{scanDir, "--kind", tt.kind, "-d", "1"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			configPath := filepath.Join(scanDir, DefaultConfigFile)
			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("failed to read config: %v", err)
			}

			if !strings.Contains(string(content), tt.expectInFile) {
				t.Errorf("config should contain %q, got:\n%s", tt.expectInFile, string(content))
			}

			// Verify both repos are in config
			if !strings.Contains(string(content), "repo1") {
				t.Error("config should contain repo1")
			}
			if !strings.Contains(string(content), "repo2") {
				t.Error("config should contain repo2")
			}
		})
	}
}

func TestInitCmd_WorkspaceScan_OmitsRootDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a git repo at root
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitConfig := filepath.Join(tmpDir, ".git", "config")
	configContent := `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = https://github.com/test/root.git
`
	if err := os.WriteFile(gitConfig, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{tmpDir, "--kind", "workspace", "-d", "0"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(tmpDir, DefaultConfigFile)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if strings.Contains(string(content), "path: .") {
		t.Errorf("config should omit default path '.' for root repo\nGot:\n%s", string(content))
	}
	if strings.Contains(string(content), "type: git") {
		t.Errorf("config should omit default type 'git'\nGot:\n%s", string(content))
	}
}

// TestExtractSSHPortFromURL tests SSH port extraction from URLs.
func TestExtractSSHPortFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want int
	}{
		// Non-standard SSH ports
		{
			name: "ssh URL with port 2224",
			url:  "ssh://git@gitlab.polypia.net:2224/org/repo.git",
			want: 2224,
		},
		{
			name: "ssh URL with port 443",
			url:  "ssh://git@example.com:443/org/repo.git",
			want: 443,
		},
		// Standard SSH port (should return 0)
		{
			name: "ssh URL with standard port 22",
			url:  "ssh://git@example.com:22/org/repo.git",
			want: 0,
		},
		// No port specified
		{
			name: "ssh URL without port",
			url:  "ssh://git@github.com/org/repo.git",
			want: 0,
		},
		// Non-SSH URLs (should return 0)
		{
			name: "https URL",
			url:  "https://github.com/org/repo.git",
			want: 0,
		},
		{
			name: "git@ URL (not ssh://)",
			url:  "git@github.com:org/repo.git",
			want: 0,
		},
		// Edge cases
		{
			name: "empty URL",
			url:  "",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSSHPortFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractSSHPortFromURL(%q) = %d, want %d", tt.url, got, tt.want)
			}
		})
	}
}

// TestExtractSSHPortFromURLs tests SSH port extraction from multiple URLs.
func TestExtractSSHPortFromURLs(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		want int
	}{
		{
			name: "all same port",
			urls: []string{
				"ssh://git@gitlab.polypia.net:2224/org/repo1.git",
				"ssh://git@gitlab.polypia.net:2224/org/repo2.git",
			},
			want: 2224,
		},
		{
			name: "mixed with no-port URLs",
			urls: []string{
				"ssh://git@gitlab.polypia.net:2224/org/repo1.git",
				"https://github.com/org/repo2.git",
			},
			want: 2224,
		},
		{
			name: "inconsistent ports - returns 0",
			urls: []string{
				"ssh://git@gitlab.polypia.net:2224/org/repo1.git",
				"ssh://git@other.com:443/org/repo2.git",
			},
			want: 0,
		},
		{
			name: "no custom ports",
			urls: []string{
				"https://github.com/org/repo1.git",
				"git@github.com:org/repo2.git",
			},
			want: 0,
		},
		{
			name: "empty list",
			urls: []string{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSSHPortFromURLs(tt.urls)
			if got != tt.want {
				t.Errorf("extractSSHPortFromURLs(%v) = %d, want %d", tt.urls, got, tt.want)
			}
		})
	}
}

func TestInitCmd_WorkspaceScan_ExplainDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a git repo in a subdirectory (not root, which is now skipped)
	subRepo := filepath.Join(tmpDir, "subrepo")
	if err := os.MkdirAll(filepath.Join(subRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitConfig := filepath.Join(subRepo, ".git", "config")
	configContent := `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = https://github.com/test/subrepo.git
`
	if err := os.WriteFile(gitConfig, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{tmpDir, "--kind", "workspace", "-d", "1", "--explain-defaults"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(tmpDir, DefaultConfigFile)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// When path equals name, path is omitted (compact output)
	// So we check for commented default type instead
	if !strings.Contains(string(content), "# type: git") {
		t.Errorf("config should include commented default type\nGot:\n%s", string(content))
	}
	// Verify workspace entry exists
	if !strings.Contains(string(content), "subrepo:") {
		t.Errorf("config should include subrepo workspace\nGot:\n%s", string(content))
	}
}
