// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func (f CommandFactory) newFromConfigCmd() *cobra.Command {
	var (
		configPath string
		strategy   string
		parallel   int
		maxRetries int
		resume     bool
		dryRun     bool
		stateFile  string
	)

	cmd := &cobra.Command{
		Use:   "from-config",
		Short: "Sync repositories from a YAML configuration file",
		Long: `Sync repositories defined in a YAML configuration file.

This command reads a config file specifying repositories to sync,
then clones or updates each repository according to the sync strategy.

Config File Format (YAML):
  strategy: reset          # default strategy: reset|pull|fetch
  parallel: 4              # concurrent operations
  maxRetries: 3            # retry attempts per repo
  cleanupOrphans: false    # remove dirs not in config
  cloneProto: ssh          # ssh or https
  sshPort: 0               # custom SSH port (0 = auto)
  roots:                   # required if cleanupOrphans=true
    - ./repos
  repositories:
    - name: my-project
      url: https://github.com/owner/my-project.git
      targetPath: ./repos/my-project
      strategy: pull       # per-repo override (optional)
      cloneProto: ssh      # per-repo override (optional)
    - name: another-repo
      url: git@github.com:owner/another-repo.git
      targetPath: ./repos/another-repo

Examples:
  # Sync from config file
  gz-git sync from-config -c config.yaml

  # Preview without making changes
  gz-git sync from-config -c config.yaml --dry-run

  # Override strategy for all repos
  gz-git sync from-config -c config.yaml --strategy pull

  # Resume interrupted sync
  gz-git sync from-config -c config.yaml --resume --state-file state.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			loader := f.SpecLoader
			if loader == nil {
				loader = FileSpecLoader{}
			}

			cfg, err := loader.Load(ctx, configPath)
			if err != nil {
				return err
			}

			planReq := cfg.Plan
			runOpts := cfg.Run

			if cmd.Flags().Changed("strategy") {
				parsed, err := reposync.ParseStrategy(strategy)
				if err != nil {
					return err
				}
				planReq.Options.DefaultStrategy = parsed
			}

			if cmd.Flags().Changed("parallel") {
				runOpts.Parallel = parallel
			}
			if cmd.Flags().Changed("max-retries") {
				runOpts.MaxRetries = maxRetries
			}
			if cmd.Flags().Changed("resume") {
				runOpts.Resume = resume
			}
			if cmd.Flags().Changed("dry-run") {
				runOpts.DryRun = dryRun
			}
			if runOpts.Resume && stateFile == "" {
				return fmt.Errorf("resume requested but no --state-file provided")
			}

			progress := ConsoleProgressSink{Out: cmd.OutOrStdout()}

			orch, err := f.orchestrator()
			if err != nil {
				return err
			}

			var state reposync.StateStore
			if stateFile != "" {
				state = reposync.NewFileStateStore(stateFile)
			}

			_, err = orch.Run(ctx, reposync.RunRequest{
				PlanRequest: planReq,
				RunOptions:  runOpts,
				Progress:    progress,
				State:       state,
			})
			if err != nil {
				return fmt.Errorf("sync from config failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to sync config file [required]")
	_ = cmd.MarkFlagRequired("config")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Default strategy override (reset|pull|fetch)")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Parallel workers (overrides config)")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Retry attempts per repo (overrides config)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from previous state (overrides config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry-run without touching git (overrides config)")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to persist and load run state for resume")

	return cmd
}
