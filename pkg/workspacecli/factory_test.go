// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"testing"
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

func TestDefaultConfigFile(t *testing.T) {
	if DefaultConfigFile != ".gz-git.yaml" {
		t.Errorf("DefaultConfigFile = %q, want %q", DefaultConfigFile, ".gz-git.yaml")
	}
}
