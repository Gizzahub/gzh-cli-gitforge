// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// DefaultConfigFile is the default workspace config filename.
const DefaultConfigFile = ".gz-git.yaml"

// CommandFactory builds workspace CLI commands.
type CommandFactory struct {
	Orchestrator reposync.Runner
}

// NewRootCmd returns the workspace root command.
func (f CommandFactory) NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "workspace",
		Short: "Manage local workspace with config-based repository sync",
		Long: `Manage local workspace with config-based repository synchronization.

Workspace commands handle local config file operations for managing
multiple git repositories as a workspace.

Config File:
  Default: .gz-git.yaml (in current directory)
  Custom:  Use -c/--config flag to specify different file

Examples:
  # Initialize workspace (scan and create config)
  gz-git workspace init .
  gz-git workspace init ~/mydevbox

  # Sync repositories
  gz-git workspace sync

  # Check workspace status
  gz-git workspace status

  # Use custom config file
  gz-git workspace sync -c myworkspace.yaml`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Define Groups
	// Define Groups
	// ANSI color codes
	const (
		colorCyanBold   = "\033[1;36m"
		colorYellowBold = "\033[1;33m"
		colorReset      = "\033[0m"
	)

	mgmtGroup := &cobra.Group{ID: "mgmt", Title: colorYellowBold + "Management" + colorReset}
	opsGroup := &cobra.Group{ID: "ops", Title: colorYellowBold + "Operations" + colorReset}
	diagGroup := &cobra.Group{ID: "diag", Title: colorYellowBold + "Diagnostics" + colorReset}

	root.AddGroup(mgmtGroup, opsGroup, diagGroup)

	// Management
	initCmd := f.newInitCmd()
	initCmd.GroupID = mgmtGroup.ID
	root.AddCommand(initCmd)

	addCmd := f.newAddCmd()
	addCmd.GroupID = mgmtGroup.ID
	root.AddCommand(addCmd)

	generateCmd := f.newGenerateCmd()
	generateCmd.GroupID = mgmtGroup.ID
	root.AddCommand(generateCmd)

	// Operations
	syncCmd := f.newSyncCmd()
	syncCmd.GroupID = opsGroup.ID
	root.AddCommand(syncCmd)

	// Diagnostics
	statusCmd := f.newStatusCmd()
	statusCmd.GroupID = diagGroup.ID
	root.AddCommand(statusCmd)

	validateCmd := f.newValidateCmd()
	validateCmd.GroupID = diagGroup.ID
	root.AddCommand(validateCmd)

	return root
}
