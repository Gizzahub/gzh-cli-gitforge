// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"testing"
)

func TestCommandFactory_NewRootCmd_Defaults(t *testing.T) {
	factory := CommandFactory{}

	cmd := factory.NewRootCmd()

	if cmd.Use != "git-sync" {
		t.Errorf("Use = %q, want %q", cmd.Use, "git-sync")
	}

	if cmd.Short != "Git repository synchronization" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Git repository synchronization")
	}

	if !cmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}

	if !cmd.SilenceErrors {
		t.Error("SilenceErrors should be true")
	}
}

func TestCommandFactory_NewRootCmd_CustomUse(t *testing.T) {
	factory := CommandFactory{
		Use:   "custom-sync",
		Short: "Custom description",
	}

	cmd := factory.NewRootCmd()

	if cmd.Use != "custom-sync" {
		t.Errorf("Use = %q, want %q", cmd.Use, "custom-sync")
	}

	if cmd.Short != "Custom description" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Custom description")
	}
}

func TestCommandFactory_NewRootCmd_HasSubcommands(t *testing.T) {
	factory := CommandFactory{}

	cmd := factory.NewRootCmd()

	// Check that subcommands are registered
	subcommands := cmd.Commands()
	if len(subcommands) == 0 {
		t.Error("Expected subcommands to be registered")
	}

	// Check for expected subcommand names
	cmdNames := make(map[string]bool)
	for _, sub := range subcommands {
		cmdNames[sub.Name()] = true
	}

	expectedCmds := []string{"from", "config", "status", "setup"}
	for _, name := range expectedCmds {
		if !cmdNames[name] {
			t.Errorf("Expected subcommand %q not found", name)
		}
	}
}

func TestCommandFactory_NewRootCmd_HasGroups(t *testing.T) {
	factory := CommandFactory{}

	cmd := factory.NewRootCmd()

	groups := cmd.Groups()
	if len(groups) == 0 {
		t.Error("Expected command groups to be registered")
	}

	// Check for expected group IDs
	groupIDs := make(map[string]bool)
	for _, g := range groups {
		groupIDs[g.ID] = true
	}

	expectedGroups := []string{"sync", "config", "diag"}
	for _, id := range expectedGroups {
		if !groupIDs[id] {
			t.Errorf("Expected group %q not found", id)
		}
	}
}

func TestCommandFactory_orchestrator_NotConfigured(t *testing.T) {
	factory := CommandFactory{}

	_, err := factory.orchestrator()
	if err == nil {
		t.Error("Expected error when orchestrator not configured")
	}
}
