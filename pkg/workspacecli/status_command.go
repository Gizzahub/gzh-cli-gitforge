// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

func (f CommandFactory) newStatusCmd() *cobra.Command {
	opts := &reposynccli.StatusOptions{
		Timeout:   30 * time.Second,
		Parallel:  4,
		ScanDepth: 1,
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check workspace health",
		Long: cliutil.QuickStartHelp(`  # Check all repositories from config
  gz-git workspace status

  # Check with specific config
  gz-git workspace status -c myworkspace.yaml

  # Check repositories in a directory (no config)
  gz-git workspace status --target ~/repos --scan-depth 2

  # Quick check (skip remote fetch)
  gz-git workspace status --skip-fetch

  # Interactive TUI mode
  gz-git workspace status --tui

  # Detailed output (show branches/divergence)
  gz-git workspace status --verbose`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use reposynccli's loader directly for consistent behavior
			loader := reposynccli.FileSpecLoader{}
			return reposynccli.RunStatus(cmd, opts, loader)
		},
	}

	cmd.Flags().StringVarP(&opts.ConfigFile, "config", "c", "", "Workspace config file")
	cmd.Flags().StringVar(&opts.TargetPath, "target", "", "Target directory to scan (instead of config)")
	cmd.Flags().IntVarP(&opts.ScanDepth, "scan-depth", "d", opts.ScanDepth, "Directory scan depth")

	cmd.Flags().BoolVar(&opts.SkipFetch, "skip-fetch", false, "Skip remote fetch (faster but may show stale data)")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", opts.Timeout, "Timeout for remote fetch per repository")
	cmd.Flags().IntVarP(&opts.Parallel, "parallel", "j", opts.Parallel, "Parallel health checks")

	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Verbose output")
	cmd.Flags().StringVarP(&opts.Format, "format", "f", "default", "Output format: default, json, compact")
	cmd.Flags().BoolVar(&opts.UseTUI, "tui", false, "Interactive TUI mode")

	cmd.Flags().BoolVar(&opts.SaveHistory, "save-history", false, "Save health snapshot to history")
	cmd.Flags().StringVar(&opts.HistoryDir, "history-dir", "", "History directory")

	return cmd
}

