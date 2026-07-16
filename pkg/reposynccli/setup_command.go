// Copyright (c) 2025 Gizzahub
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
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/wizard"
)

// newSetupCmd creates the setup wizard command.
func (f CommandFactory) newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard for repository synchronization",
		Long: cliutil.QuickStartHelp(`  # Start the setup wizard
  gz-git forge setup

  # After setup, run the generated command:
  gz-git forge from --provider gitlab --org myorg --path ~/repos`),
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
		return fmt.Errorf("wizard canceled: %w", err)
	}

	// Save config if requested
	if opts.SaveConfig {
		if err := saveWizardConfig(opts); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save config: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nConfiguration saved to: %s\n", opts.ConfigPath)
			if opts.Token != "" {
				envVar := tokenEnvVarForProvider(opts.Provider)
				fmt.Fprintf(cmd.OutOrStdout(),
					"Note: the token itself was not saved; the config references ${%s}.\nExport it before syncing: export %s=<your-token>\n",
					envVar, envVar)
			}
			if wizardFiltersDropped(opts) {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Note: repository filters (archived/private/forks) are not stored in the workspace config.\n"+
						"When syncing with 'gz-git sync -c %s', private repos are included and archived/forks excluded.\n"+
						"Use the equivalent 'gz-git forge from' command above to apply your chosen filters.\n",
					opts.ConfigPath)
			}
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

// tokenEnvVarForProvider returns the conventional token env var name for a provider.
func tokenEnvVarForProvider(provider string) string {
	switch provider {
	case "gitlab":
		return "GITLAB_TOKEN"
	case "gitea":
		return "GITEA_TOKEN"
	default:
		return "GITHUB_TOKEN"
	}
}

// wizardConfigFile wraps the hierarchical config with the version/kind header so
// the saved file is self-describing. detectConfigKind honors the explicit kind
// first, guaranteeing the file routes to the workspace loader rather than the
// gzh.yaml path that requires a repositories array.
type wizardConfigFile struct {
	Version int               `yaml:"version"`
	Kind    config.ConfigKind `yaml:"kind"`
	Config  config.Config     `yaml:",inline"`
}

// buildWizardConfig converts wizard options into the hierarchical pkg/config
// typed structure (kind: workspace, one forge workspace). The wizard previously
// wrote a flat map that no loader could consume — the sync loader detected it as
// a gzh.yaml and rejected it for having no repositories. Emitting a real
// config.Config makes the saved file loadable by `gz-git sync -c` and lets it
// pass config validation (the AC4 round-trip).
//
// The wizard's archived/private/forks filter choices have no field in the
// hierarchical schema (loadForgeWorkspace applies fixed behavior), so they are
// intentionally not persisted here; runSetup warns when that drops a choice.
func buildWizardConfig(opts *wizard.SyncSetupOptions) config.Config {
	src := &config.ForgeSource{
		Provider: opts.Provider,
		Org:      opts.Organization,
		BaseURL:  opts.BaseURL,
	}
	if opts.Token != "" {
		// Never persist raw tokens to disk; reference an env var instead.
		src.Token = "${" + tokenEnvVarForProvider(opts.Provider) + "}"
	}
	if opts.Provider == "gitlab" {
		src.IncludeSubgroups = opts.IncludeSubgroups
		src.SubgroupMode = opts.SubgroupMode
	}

	ws := &config.Workspace{
		Path:       opts.TargetPath,
		Type:       config.WorkspaceTypeForge,
		Source:     src,
		CloneProto: opts.CloneProto,
		Parallel:   opts.Parallel,
	}
	if opts.SSHPort != 0 && opts.SSHPort != 22 {
		ws.SSHPort = opts.SSHPort
	}

	return config.Config{
		Metadata: &config.Metadata{Name: opts.Organization},
		Workspaces: map[string]*config.Workspace{
			opts.Organization: ws,
		},
	}
}

// wizardFiltersDropped reports whether the wizard captured repo-filter choices
// that the hierarchical workspace config cannot express. loadForgeWorkspace
// fixes these (private included, archived/forks excluded), so a config saved
// from non-default filter choices would sync differently than the wizard's
// immediate run — we surface that rather than silently dropping the setting.
func wizardFiltersDropped(opts *wizard.SyncSetupOptions) bool {
	return opts.IncludeArchived || !opts.IncludePrivate || opts.IncludeForks
}

func saveWizardConfig(opts *wizard.SyncSetupOptions) error {
	cfg := buildWizardConfig(opts)

	// Route through the shared validator so we never persist a config that
	// `gz-git sync` / `config validate` would later reject.
	if err := config.NewValidator().ValidateConfig(&cfg); err != nil {
		return fmt.Errorf("generated config failed validation: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(opts.ConfigPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write config file with an explicit version/kind header.
	data, err := yaml.Marshal(wizardConfigFile{
		Version: 1,
		Kind:    config.KindWorkspace,
		Config:  cfg,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(opts.ConfigPath, data, 0o600); err != nil {
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

	// Create ForgePlanner config. Auth mirrors `forge from`: without it the
	// planner injects no token into HTTPS clone URLs, so private-repo clones
	// from the wizard's execute-now path would fail authentication.
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
		Auth: reposync.AuthConfig{
			Token:    opts.Token,
			Provider: opts.Provider,
			SSHPort:  opts.SSHPort,
		},
	}

	planner := reposync.NewForgePlanner(forgeProvider, plannerConfig)

	// Create orchestrator
	executor := reposync.GitExecutor{}
	orchestrator := reposync.NewOrchestrator(planner, executor, nil)

	// Build plan request
	strategy, err := reposync.ParseStrategy("reset")
	if err != nil {
		return fmt.Errorf("invalid strategy: %w", err)
	}
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
	return CreateForgeProviderRaw(opts.Provider, opts.Token, opts.BaseURL, opts.SSHPort)
}
