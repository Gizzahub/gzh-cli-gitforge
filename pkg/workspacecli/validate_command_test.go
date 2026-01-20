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
strategy: reset
parallel: 4
maxRetries: 3
repositories:
  - name: test-repo
    url: https://github.com/test/repo.git
    targetPath: ./repos/test
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
