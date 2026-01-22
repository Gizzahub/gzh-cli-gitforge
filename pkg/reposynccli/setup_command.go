// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitea"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/github"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/wizard"
)

// newSetupCmd creates the setup wizard command.
func (f CommandFactory) newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard for repository synchronization",
		Long: cliutil.QuickStartHelp(`  # Start the setup wizard
  gz-git sync setup

  # After setup, run the generated command:
  gz-git sync from-forge --provider gitlab --org myorg --path ~/repos`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.runSetup(cmd)
		},
	}

	return cmd
}

func (f CommandFactory) runSetup(cmd *cobra.Command) error {
	ctx := cmd.Context()

	// Run the wizard
	w := wizard.NewSyncSetupWizard()
	opts, err := w.Run(ctx)
	if err != nil {
		return fmt.Errorf("wizard cancelled: %w", err)
	}

	// Save config if requested
	if opts.SaveConfig {
		if err := saveWizardConfig(opts); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save config: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nConfiguration saved to: %s\n", opts.ConfigPath)
		}
	}

	// Show the equivalent command
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Equivalent command:")
	fmt.Fprintln(cmd.OutOrStdout(), "  "+w.BuildCommand())
	fmt.Fprintln(cmd.OutOrStdout())

	// Execute if requested
	if opts.ExecuteNow {
		fmt.Fprintln(cmd.OutOrStdout(), "Starting synchronization...")
		fmt.Fprintln(cmd.OutOrStdout())
		return f.executeSyncFromWizard(ctx, cmd, opts)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Setup complete. Run the command above to start synchronization.")
	return nil
}

func saveWizardConfig(opts *wizard.SyncSetupOptions) error {
	// Create a SyncConfig from wizard options
	config := map[string]interface{}{
		"provider":     opts.Provider,
		"organization": opts.Organization,
		"target":       opts.TargetPath,
		"cloneProto":   opts.CloneProto,
		"parallel":     opts.Parallel,
	}

	if opts.BaseURL != "" {
		config["baseURL"] = opts.BaseURL
	}

	if opts.Token != "" {
		config["token"] = opts.Token
	}

	if opts.SSHPort != 0 && opts.SSHPort != 22 {
		config["sshPort"] = opts.SSHPort
	}

	if opts.Provider == "gitlab" {
		config["includeSubgroups"] = opts.IncludeSubgroups
		config["subgroupMode"] = opts.SubgroupMode
	}

	config["includeArchived"] = opts.IncludeArchived
	config["includePrivate"] = opts.IncludePrivate
	config["includeForks"] = opts.IncludeForks

	// Ensure directory exists
	dir := filepath.Dir(opts.ConfigPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write config file
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(opts.ConfigPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (f CommandFactory) executeSyncFromWizard(ctx context.Context, cmd *cobra.Command, opts *wizard.SyncSetupOptions) error {
	// Create provider
	forgeProvider, err := createProviderFromWizard(opts)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create ForgePlanner config
	plannerConfig := reposync.ForgePlannerConfig{
		TargetPath:       opts.TargetPath,
		Organization:     opts.Organization,
		IncludeArchived:  opts.IncludeArchived,
		IncludeForks:     opts.IncludeForks,
		IncludePrivate:   opts.IncludePrivate,
		CloneProto:       opts.CloneProto,
		SSHPort:          opts.SSHPort,
		IncludeSubgroups: opts.IncludeSubgroups,
		SubgroupMode:     opts.SubgroupMode,
	}

	planner := reposync.NewForgePlanner(forgeProvider, plannerConfig)

	// Create orchestrator
	executor := reposync.GitExecutor{}
	orchestrator := reposync.NewOrchestrator(planner, executor, nil)

	// Build plan request
	strategy, _ := reposync.ParseStrategy("reset")
	planReq := reposync.PlanRequest{
		Options: reposync.PlanOptions{
			DefaultStrategy: strategy,
		},
	}

	// Build run options
	runOpts := reposync.RunOptions{
		Parallel:   opts.Parallel,
		MaxRetries: 3,
	}

	// Progress
	progress := ConsoleProgressSink{Out: cmd.OutOrStdout()}

	// Run
	result, err := orchestrator.Run(ctx, reposync.RunRequest{
		PlanRequest: planReq,
		RunOptions:  runOpts,
		Progress:    progress,
	})
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Print summary
	fmt.Fprintf(cmd.OutOrStdout(), "\nSync completed: %d succeeded, %d failed, %d skipped\n",
		len(result.Succeeded), len(result.Failed), len(result.Skipped))

	return nil
}

func createProviderFromWizard(opts *wizard.SyncSetupOptions) (reposync.ForgeProvider, error) {
	switch opts.Provider {
	case "github":
		return github.NewProvider(opts.Token), nil

	case "gitlab":
		p, err := gitlab.NewProviderWithOptions(gitlab.ProviderOptions{
			Token:   opts.Token,
			BaseURL: opts.BaseURL,
			SSHPort: opts.SSHPort,
		})
		if err != nil {
			return nil, err
		}
		return forgeProviderAdapter{p}, nil

	case "gitea":
		return forgeProviderAdapter{gitea.NewProvider(opts.Token, opts.BaseURL)}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", opts.Provider)
	}
}
