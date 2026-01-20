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

	if cmd.Use != "init" {
		t.Errorf("Use = %q, want %q", cmd.Use, "init")
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

func TestInitCmd_CreatesNewConfig(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Execute
	cmd.SetArgs([]string{})
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

	if !strings.Contains(string(content), "strategy: reset") {
		t.Error("config should contain default strategy")
	}

	if !strings.Contains(string(content), "repositories:") {
		t.Error("config should contain repositories section")
	}
}

func TestInitCmd_CreatesCustomNameConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-workspace.yaml")

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"-c", configPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("custom config file was not created")
	}
}

func TestInitCmd_ErrorOnExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "existing.yaml")

	// Create existing file
	if err := os.WriteFile(configPath, []byte("# existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	cmd.SetArgs([]string{"-c", configPath})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error when file already exists")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention file exists: %v", err)
	}
}

func TestInitCmd_ConfigFlag(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.newInitCmd()

	flag := cmd.Flags().Lookup("config")
	if flag == nil {
		t.Error("config flag should exist")
	}

	if flag.Shorthand != "c" {
		t.Errorf("config shorthand = %q, want %q", flag.Shorthand, "c")
	}
}
