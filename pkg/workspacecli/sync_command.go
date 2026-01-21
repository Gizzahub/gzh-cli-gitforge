// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func (f CommandFactory) newSyncCmd() *cobra.Command {
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
		Use:   "sync",
		Short: "Clone/update repositories from config",
		Long: `Quick Start:
  # Sync from default config (.gz-git.yaml)
  gz-git workspace sync

  # Sync from specific config
  gz-git workspace sync -c myworkspace.yaml

  # Preview without making changes
  gz-git workspace sync --dry-run

  # Override strategy for all repos
  gz-git workspace sync --strategy pull

  # Resume interrupted sync
  gz-git workspace sync --resume --state-file state.json

Config File Structure (Reference):
  strategy: reset # reset|pull|fetch
  parallel: 4
  repositories:
    - name: my-project
      url: https://github.com/owner/my-project.git
      targetPath: ./repos/my-project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Auto-detect config file if not specified
			if configPath == "" {
				detected, err := detectConfigFile(".")
				if err != nil {
					return fmt.Errorf("no config file specified and auto-detection failed: %w", err)
				}
				configPath = detected
				fmt.Fprintf(cmd.OutOrStdout(), "Using config: %s\n", configPath)
			}

			loader := FileSpecLoader{}
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

			progress := &consoleProgress{out: cmd.OutOrStdout()}

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
				return fmt.Errorf("workspace sync failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file (auto-detects "+DefaultConfigFile+")")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Strategy override (reset|pull|fetch)")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Parallel workers (overrides config)")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Retry attempts per repo (overrides config)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from previous state")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without making changes")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to persist run state for resume")

	return cmd
}

// consoleProgress implements reposync.ProgressSink for console output.
type consoleProgress struct {
	out io.Writer
}

// OnStart implements reposync.ProgressSink.
func (p *consoleProgress) OnStart(action reposync.Action) {
	fmt.Fprintf(p.out, "→ [%s] %s (%s)\n", action.Type, action.Repo.Name, action.Repo.TargetPath)
}

// OnProgress implements reposync.ProgressSink.
func (p *consoleProgress) OnProgress(action reposync.Action, message string, progress float64) {
	fmt.Fprintf(p.out, "   [%s] %s: %s (%.0f%%)\n", action.Type, action.Repo.Name, message, progress*100)
}

// OnComplete implements reposync.ProgressSink.
func (p *consoleProgress) OnComplete(result reposync.ActionResult) {
	if result.Error != nil {
		fmt.Fprintf(p.out, "✗ [%s] %s: %v\n", result.Action.Type, result.Action.Repo.Name, result.Error)
		return
	}
	fmt.Fprintf(p.out, "✓ [%s] %s: %s\n", result.Action.Type, result.Action.Repo.Name, result.Message)
}
