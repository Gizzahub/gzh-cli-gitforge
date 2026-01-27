// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// CommandFactory builds a Cobra command tree that can be embedded into other CLIs.
type CommandFactory struct {
	Use   string
	Short string

	Orchestrator reposync.Runner
	SpecLoader   SpecLoader

	Version   string
	Commit    string
	BuildDate string
}

// NewRootCmd returns a root command suitable for standalone binary usage.
func (f CommandFactory) NewRootCmd() *cobra.Command {
	use := f.Use
	if use == "" {
		use = "git-sync"
	}

	short := f.Short
	if short == "" {
		short = "Git repository synchronization"
	}

	root := &cobra.Command{
		Use:           use,
		Short:         short,
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Git forge operations (GitHub, GitLab, Gitea).
Use this command to interact directly with Forge APIs. For local config-based operations, use 'gz-git workspace'.

` + cliutil.QuickStartHelp(`  # 1. Generate config from Forge
  gz-git forge config generate --provider gitlab --org myteam -o .gz-git.yaml

  # 2. Sync directly from Forge (One-off)
  gz-git forge from-forge --provider gitlab --org myteam --path ~/repos

  # 3. Check repository health
  gz-git forge status --path ~/repos

See 'gz-git workspace' for managing synced repositories via config file.`),
	}

	// Define Groups
	// ANSI color codes
	const (
		colorCyanBold   = "\033[1;36m"
		colorYellowBold = "\033[1;33m"
		colorReset      = "\033[0m"
	)

	syncGroup := &cobra.Group{ID: "sync", Title: colorYellowBold + "Sync Operations" + colorReset}
	configGroup := &cobra.Group{ID: "config", Title: colorYellowBold + "Configuration" + colorReset}
	diagGroup := &cobra.Group{ID: "diag", Title: colorYellowBold + "Diagnostics" + colorReset}

	root.AddGroup(syncGroup, configGroup, diagGroup)

	// Sync Operations
	fromForgeCmd := f.newFromForgeCmd()
	fromForgeCmd.GroupID = syncGroup.ID
	root.AddCommand(fromForgeCmd)

	setupCmd := f.newSetupCmd()
	setupCmd.GroupID = syncGroup.ID
	root.AddCommand(setupCmd)

	// Configuration
	configCmd := f.newConfigCmd()

	configCmd.GroupID = configGroup.ID
	root.AddCommand(configCmd)

	// Diagnostics
	statusCmd := f.newStatusCmd()
	statusCmd.GroupID = diagGroup.ID
	root.AddCommand(statusCmd)

	return root
}

func (f CommandFactory) orchestrator() (reposync.Runner, error) {
	if f.Orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not configured")
	}
	return f.Orchestrator, nil
}
