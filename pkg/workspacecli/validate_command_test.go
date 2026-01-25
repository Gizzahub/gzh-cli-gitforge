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

func TestNewValidateCmd(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	if cmd.Use != "validate" {
		t.Errorf("Use = %q, want %q", cmd.Use, "validate")
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

func TestValidateCmd_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "valid.yaml")

	validConfig := `
version: 1
kind: repositories
strategy: reset
parallel: 4
maxRetries: 3
repositories:
  - name: test-repo
    url: https://github.com/test/repo.git
    path: ./repos/test
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"-c", configPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "valid") {
		t.Errorf("output should indicate success: %s", output)
	}
}

func TestValidateCmd_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: [yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid YAML")
	}

	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should mention validation failed: %v", err)
	}
}

func TestValidateCmd_InvalidStrategy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad-strategy.yaml")

	badConfig := `
strategy: invalid-strategy
repositories: []
`
	if err := os.WriteFile(configPath, []byte(badConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid strategy")
	}
}

func TestValidateCmd_FileNotFound(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", "/nonexistent/path/config.yaml"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestValidateCmd_AutoDetect(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create config in tmpDir
	configPath := filepath.Join(tmpDir, DefaultConfigFile)
	validConfig := `
version: 1
kind: repositories
strategy: reset
repositories: []
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Using config:") {
		t.Errorf("should show auto-detected config: %s", output)
	}
}

func TestValidateCmd_AutoDetectFails(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error when no config found")
	}

	if !strings.Contains(err.Error(), "auto-detection failed") {
		t.Errorf("error should mention auto-detection: %v", err)
	}
}

func TestValidateCmd_ConfigFlag(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	flag := cmd.Flags().Lookup("config")
	if flag == nil {
		t.Error("config flag should exist")
	}

	if flag.Shorthand != "c" {
		t.Errorf("config shorthand = %q, want %q", flag.Shorthand, "c")
	}
}

// TestValidateCmd_MissingKind tests validation when kind is missing.
func TestValidateCmd_MissingKind(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "no-kind.yaml")

	noKindConfig := `
version: 1
repositories:
  - name: test-repo
    url: https://github.com/test/repo.git
`
	if err := os.WriteFile(configPath, []byte(noKindConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for missing kind")
	}

	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should mention validation failed: %v", err)
	}
}

// TestValidateCmd_InvalidKind tests error for invalid kind values.
func TestValidateCmd_InvalidKind(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-kind.yaml")

	config := `
version: 1
kind: invalid
repositories: []
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid kind")
	}
}

// TestValidateCmd_KindMismatch tests warning when kind doesn't match content.
func TestValidateCmd_KindMismatch(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		wantWarn string
	}{
		{
			name: "workspace kind with repositories",
			config: `
version: 1
kind: workspace
repositories:
  - name: test
    url: https://github.com/test/repo.git
`,
			wantWarn: "repositories",
		},
		{
			name: "repositories kind with workspaces",
			config: `
version: 1
kind: repositories
workspaces:
  test:
    path: test
    type: git
`,
			wantWarn: "workspaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "mismatch.yaml")

			if err := os.WriteFile(configPath, []byte(tt.config), 0o644); err != nil {
				t.Fatal(err)
			}

			factory := CommandFactory{}
			cmd := factory.newValidateCmd()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			cmd.SetArgs([]string{"-c", configPath})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, tt.wantWarn) {
				t.Errorf("output should warn about %q: %s", tt.wantWarn, output)
			}
		})
	}
}

// TestValidateCmd_WorkspaceKind tests validation of workspace kind config.
func TestValidateCmd_WorkspaceKind(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "workspace.yaml")

	config := `
version: 1
kind: workspace
strategy: reset
workspaces:
  project1:
    path: project1
    type: git
    url: https://github.com/user/project1.git
  project2:
    path: project2
    type: config
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"-c", configPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "valid") {
		t.Errorf("output should indicate success: %s", output)
	}
}

// TestValidateCmd_WorkspaceEntryMissingPath tests error for workspace without path.
func TestValidateCmd_WorkspaceEntryMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "missing-path.yaml")

	config := `
version: 1
kind: workspace
workspaces:
  project1:
    type: git
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()
	// Should show warning but not fail
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "path") {
		t.Errorf("output should warn about missing path: %s", output)
	}
}

// ============================================================================
// Clone Config Validation Tests
// ============================================================================

// TestValidateCmd_CloneConfigDetection tests auto-detection of clone config.
func TestValidateCmd_CloneConfigDetection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clone-config.yaml")

	config := `
strategy: pull
parallel: 10

core:
  target: "."
  repositories:
    - https://github.com/test/repo1.git
    - https://github.com/test/repo2.git

plugins:
  target: "./plugins"
  repositories:
    - url: https://github.com/test/plugin1.git
      branch: main
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"-c", configPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Config type: clone") {
		t.Errorf("should detect clone config type: %s", output)
	}
	if !strings.Contains(output, "valid") {
		t.Errorf("should be valid: %s", output)
	}
}

// TestValidateCmd_CloneConfigWithKind tests clone config with explicit kind.
func TestValidateCmd_CloneConfigWithKind(t *testing.T) {
	tests := []struct {
		name string
		kind string
	}{
		{
			name: "groups",
			kind: "groups",
		},
		{
			name: "flat",
			kind: "flat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "clone.yaml")

			config := `
version: 1
kind: ` + tt.kind + `
strategy: pull

core:
  target: "."
  repositories:
    - https://github.com/test/repo.git
`
			if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}

			factory := CommandFactory{}
			cmd := factory.newValidateCmd()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			cmd.SetArgs([]string{"-c", configPath})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, "valid") {
				t.Errorf("config should be valid, output: %s", output)
			}
		})
	}
}

// TestValidateCmd_DeprecatedKinds tests deprecated kind aliases with warnings.
func TestValidateCmd_DeprecatedKinds(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		configType  string // "workspace" or "clone"
		wantWarning string
	}{
		// Workspace deprecated aliases
		{
			name:        "workspaces - deprecated workspace alias",
			kind:        "workspaces",
			configType:  "workspace",
			wantWarning: "'kind: workspaces' is deprecated",
		},
		{
			name:        "repository - deprecated repositories alias",
			kind:        "repository",
			configType:  "workspace",
			wantWarning: "'kind: repository' is deprecated",
		},
		// Clone deprecated alias
		{
			name:        "group - deprecated groups alias",
			kind:        "group",
			configType:  "clone",
			wantWarning: "'kind: group' is deprecated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "deprecated.yaml")

			var config string
			if tt.configType == "workspace" {
				config = `
version: 1
kind: ` + tt.kind + `
strategy: reset
repositories:
  - url: https://github.com/test/repo.git
`
			} else {
				config = `
version: 1
kind: ` + tt.kind + `
strategy: pull

core:
  target: "."
  repositories:
    - https://github.com/test/repo.git
`
			}

			if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}

			factory := CommandFactory{}
			cmd := factory.newValidateCmd()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			cmd.SetArgs([]string{"-c", configPath})
			// Should NOT error - deprecated aliases are accepted
			if err := cmd.Execute(); err != nil {
				t.Fatalf("deprecated kind should not cause error: %v", err)
			}

			output := buf.String()
			// Should show deprecation warning
			if !strings.Contains(output, tt.wantWarning) {
				t.Errorf("expected deprecation warning %q, output: %s", tt.wantWarning, output)
			}
			// Should still be valid
			if !strings.Contains(output, "valid") {
				t.Errorf("config should be valid despite deprecation warning, output: %s", output)
			}
		})
	}
}

// TestValidateCmd_CloneConfigInvalidStrategy tests invalid strategy in clone config.
func TestValidateCmd_CloneConfigInvalidStrategy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-strategy.yaml")

	config := `
kind: groups
strategy: invalid

core:
  target: "."
  repositories:
    - https://github.com/test/repo.git
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid strategy")
	}
}

// TestValidateCmd_CloneConfigMissingRepositories tests group without repositories.
func TestValidateCmd_CloneConfigMissingRepositories(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "missing-repos.yaml")

	config := `
kind: groups

core:
  target: "."
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for missing repositories in group")
	}
}

// TestValidateCmd_CloneConfigEmptyURL tests empty URL in clone config.
func TestValidateCmd_CloneConfigEmptyURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty-url.yaml")

	config := `
kind: groups

core:
  target: "."
  repositories:
    - url: ""
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for empty URL")
	}
}

// TestValidateCmd_CloneConfigFlatFormat tests flat format clone config.
func TestValidateCmd_CloneConfigFlatFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "flat.yaml")

	config := `
version: 1
kind: flat
strategy: pull
target: "."
repositories:
  - https://github.com/test/repo1.git
  - url: https://github.com/test/repo2.git
    branch: main
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newValidateCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"-c", configPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "valid") {
		t.Errorf("should be valid: %s", output)
	}
}

// TestDetectConfigType tests config type detection.
func TestDetectConfigType(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		wantType ConfigType
	}{
		{
			name: "workspace kind",
			config: map[string]interface{}{
				"kind":       "workspace",
				"workspaces": map[string]interface{}{},
			},
			wantType: ConfigTypeWorkspace,
		},
		{
			name: "repositories kind",
			config: map[string]interface{}{
				"kind":         "repositories",
				"repositories": []interface{}{},
			},
			wantType: ConfigTypeWorkspace,
		},
		{
			name: "groups kind",
			config: map[string]interface{}{
				"kind": "groups",
			},
			wantType: ConfigTypeClone,
		},
		{
			name: "flat kind",
			config: map[string]interface{}{
				"kind": "flat",
			},
			wantType: ConfigTypeClone,
		},
		{
			name: "heuristic - named groups",
			config: map[string]interface{}{
				"core": map[string]interface{}{
					"target":       ".",
					"repositories": []interface{}{},
				},
			},
			wantType: ConfigTypeClone,
		},
		{
			name: "heuristic - workspaces map",
			config: map[string]interface{}{
				"workspaces": map[string]interface{}{},
			},
			wantType: ConfigTypeWorkspace,
		},
		{
			name: "heuristic - repositories array",
			config: map[string]interface{}{
				"repositories": []interface{}{},
			},
			wantType: ConfigTypeWorkspace,
		},
		{
			name:     "unknown - empty",
			config:   map[string]interface{}{},
			wantType: ConfigTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectConfigType(tt.config)
			if got != tt.wantType {
				t.Errorf("detectConfigType() = %v, want %v", got, tt.wantType)
			}
		})
	}
}

// ============================================================================
// ValidationResult Tests
// ============================================================================

// TestValidationResult tests ValidationResult methods.
func TestValidationResult(t *testing.T) {
	t.Run("empty result is valid", func(t *testing.T) {
		result := &ValidationResult{}
		if !result.IsValid() {
			t.Error("empty result should be valid")
		}
		if result.HasIssues() {
			t.Error("empty result should have no issues")
		}
	})

	t.Run("result with errors is invalid", func(t *testing.T) {
		result := &ValidationResult{
			Errors: []string{"some error"},
		}
		if result.IsValid() {
			t.Error("result with errors should not be valid")
		}
		if !result.HasIssues() {
			t.Error("result with errors should have issues")
		}
	})

	t.Run("result with only warnings is valid but has issues", func(t *testing.T) {
		result := &ValidationResult{
			Warnings: []string{"some warning"},
		}
		if !result.IsValid() {
			t.Error("result with only warnings should be valid")
		}
		if !result.HasIssues() {
			t.Error("result with warnings should have issues")
		}
	})

	t.Run("result with only suggestions is valid but has issues", func(t *testing.T) {
		result := &ValidationResult{
			Suggestions: []string{"some suggestion"},
		}
		if !result.IsValid() {
			t.Error("result with only suggestions should be valid")
		}
		if !result.HasIssues() {
			t.Error("result with suggestions should have issues")
		}
	})
}
