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

	if !strings.Contains(string(content), "strategy:") {
		t.Error("config should contain strategy field")
	}

	if !strings.Contains(string(content), "repositories:") {
		t.Error("config should contain repositories section")
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

	if !strings.Contains(string(content), "repositories:") {
		t.Error("new config should contain repositories section")
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
