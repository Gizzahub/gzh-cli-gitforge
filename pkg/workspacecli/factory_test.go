// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func TestCommandFactory_NewRootCmd(t *testing.T) {
	factory := CommandFactory{}
	cmd := factory.NewRootCmd()

	if cmd.Use != "workspace" {
		t.Errorf("Use = %q, want %q", cmd.Use, "workspace")
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Check subcommands are registered
	subCmds := map[string]bool{
		"init":     false,
		"scan":     false,
		"sync":     false,
		"status":   false,
		"add":      false,
		"validate": false,
	}

	for _, sub := range cmd.Commands() {
		// Use Name() which returns just the command name without arguments
		if _, ok := subCmds[sub.Name()]; ok {
			subCmds[sub.Name()] = true
		}
	}

	for name, found := range subCmds {
		if !found {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}

func TestCommandFactory_WithOrchestrator(t *testing.T) {
	mockOrchestrator := &mockRunner{}

	factory := CommandFactory{
		Orchestrator: mockOrchestrator,
	}

	cmd := factory.NewRootCmd()

	if cmd == nil {
		t.Error("NewRootCmd should not return nil")
	}
}

func TestDefaultConfigFile(t *testing.T) {
	if DefaultConfigFile != ".gz-git.yaml" {
		t.Errorf("DefaultConfigFile = %q, want %q", DefaultConfigFile, ".gz-git.yaml")
	}
}

// mockRunner implements reposync.Runner for testing
type mockRunner struct{}

func (m *mockRunner) Run(_ context.Context, _ reposync.RunRequest) (reposync.ExecutionResult, error) {
	return reposync.ExecutionResult{}, nil
}
