// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/tui"
)

// StatusOptions holds options for the status command.
type StatusOptions struct {
	ConfigFile  string
	TargetPath  string
	ScanDepth   int
	SkipFetch   bool
	Timeout     time.Duration
	Parallel    int
	Verbose     bool
	Format      string
	SaveHistory bool
	HistoryDir  string
}

func (f CommandFactory) newStatusCmd() *cobra.Command {
	opts := &StatusOptions{
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

  # Detailed output (show branches/divergence)
  gz-git workspace status --verbose`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.runStatus(cmd, opts)
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

	cmd.Flags().BoolVar(&opts.SaveHistory, "save-history", false, "Save health snapshot to history")
	cmd.Flags().StringVar(&opts.HistoryDir, "history-dir", "", "History directory")

	return cmd
}

func (f CommandFactory) runStatus(cmd *cobra.Command, opts *StatusOptions) error {
	ctx := cmd.Context()

	var repos []reposync.RepoSpec

	// Auto-detect config if neither --config nor --target specified
	if opts.ConfigFile == "" && opts.TargetPath == "" {
		detected, detectErr := detectConfigFile(".")
		if detectErr == nil {
			opts.ConfigFile = detected
			fmt.Fprintf(cmd.OutOrStdout(), "Using config: %s\n", detected)
		} else {
			opts.TargetPath = "."
		}
	}

	if opts.ConfigFile != "" {
		loader := FileSpecLoader{}
		configData, err := loader.Load(ctx, opts.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		repos = configData.Plan.Input.Repos
	} else if opts.TargetPath != "" {
		planner := reposync.FSPlanner{}
		plan, err := planner.Plan(ctx, reposync.PlanRequest{
			Options: reposync.PlanOptions{
				Roots:           []string{opts.TargetPath},
				DefaultStrategy: reposync.StrategyFetch,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}

		for _, action := range plan.Actions {
			repos = append(repos, action.Repo)
		}
	}

	if len(repos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No repositories found")
		return nil
	}

	diagOpts := reposync.DiagnosticOptions{
		SkipFetch:              opts.SkipFetch,
		FetchTimeout:           opts.Timeout,
		Parallel:               opts.Parallel,
		CheckWorkTree:          true,
		IncludeRecommendations: true,
	}

	executor := reposync.DiagnosticExecutor{}
	report, err := executor.CheckHealth(ctx, repos, diagOpts)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Save to history if requested
	if opts.SaveHistory {
		historyDir := opts.HistoryDir
		if historyDir == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				historyDir = filepath.Join(home, ".gz-git", "history")
			}
		}

		if historyDir != "" {
			store := reposync.NewFileHistoryStore(historyDir)
			if err := store.Save(ctx, report); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save history: %v\n", err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Snapshot saved to %s\n", historyDir)
			}
		}
	}

	// Display results
	switch opts.Format {
	case "json":
		return printHealthReportJSON(cmd.OutOrStdout(), report)
	case "compact":
		printHealthReportCompact(cmd.OutOrStdout(), report)
	default:
		printHealthReport(cmd.OutOrStdout(), report, opts.Verbose)
	}

	return nil
}

func printHealthReport(out io.Writer, report *reposync.HealthReport, verbose bool) {
	fmt.Fprintf(out, "\nChecking workspace health...\n\n")

	for _, health := range report.Results {
		icon := tui.FormatHealthIcon(health.HealthStatus)
		statusStr := tui.FormatStatusText(health)

		fmt.Fprintf(out, "%s %-30s  %s\n",
			icon,
			tui.FormatRepoName(health.Repo),
			statusStr,
		)

		if health.HealthStatus != reposync.HealthHealthy && health.Recommendation != "" {
			fmt.Fprintf(out, "  → %s\n", health.Recommendation)
		}

		if verbose {
			printVerboseHealth(out, health)
		}

		if health.Error != nil {
			fmt.Fprintf(out, "  ⚠  Error: %v\n", health.Error)
		}
	}

	fmt.Fprintf(out, "\nSummary: %d healthy, %d warnings, %d errors, %d unreachable (%d total)\n",
		report.Summary.Healthy,
		report.Summary.Warning,
		report.Summary.Error,
		report.Summary.Unreachable,
		report.Summary.Total,
	)

	fmt.Fprintf(out, "Total time: %v\n", report.TotalDuration.Round(time.Millisecond))
}

func printVerboseHealth(out io.Writer, health reposync.RepoHealth) {
	fmt.Fprintf(out, "     Branch: %s", health.CurrentBranch)
	if health.UpstreamBranch != "" {
		fmt.Fprintf(out, " → %s", health.UpstreamBranch)
	}
	fmt.Fprintln(out)

	if health.AheadBy > 0 || health.BehindBy > 0 {
		fmt.Fprintf(out, "     Divergence: %d↑ ahead, %d↓ behind\n", health.AheadBy, health.BehindBy)
	}

	if health.ModifiedFiles > 0 || health.UntrackedFiles > 0 || health.ConflictFiles > 0 {
		fmt.Fprintf(out, "     Working tree: %d modified, %d untracked, %d conflicts\n",
			health.ModifiedFiles, health.UntrackedFiles, health.ConflictFiles)
	}
}

func printHealthReportJSON(out io.Writer, report *reposync.HealthReport) error {
	type JSONRepoHealth struct {
		Name           string  `json:"name"`
		Path           string  `json:"path"`
		HealthStatus   string  `json:"health_status"`
		NetworkStatus  string  `json:"network_status"`
		DivergenceType string  `json:"divergence_type"`
		CurrentBranch  string  `json:"current_branch"`
		AheadBy        int     `json:"ahead_by"`
		BehindBy       int     `json:"behind_by"`
		Recommendation string  `json:"recommendation,omitempty"`
		Error          string  `json:"error,omitempty"`
		DurationMs     float64 `json:"duration_ms"`
	}

	type JSONReport struct {
		Results   []JSONRepoHealth       `json:"results"`
		Summary   reposync.HealthSummary `json:"summary"`
		TotalMs   float64                `json:"total_duration_ms"`
		CheckedAt string                 `json:"checked_at"`
	}

	jsonResults := make([]JSONRepoHealth, len(report.Results))
	for i, health := range report.Results {
		errStr := ""
		if health.Error != nil {
			errStr = health.Error.Error()
		}

		jsonResults[i] = JSONRepoHealth{
			Name:           tui.FormatRepoName(health.Repo),
			Path:           health.Repo.TargetPath,
			HealthStatus:   string(health.HealthStatus),
			NetworkStatus:  string(health.NetworkStatus),
			DivergenceType: string(health.DivergenceType),
			CurrentBranch:  health.CurrentBranch,
			AheadBy:        health.AheadBy,
			BehindBy:       health.BehindBy,
			Recommendation: health.Recommendation,
			Error:          errStr,
			DurationMs:     float64(health.Duration.Microseconds()) / 1000.0,
		}
	}

	jsonReport := JSONReport{
		Results:   jsonResults,
		Summary:   report.Summary,
		TotalMs:   float64(report.TotalDuration.Microseconds()) / 1000.0,
		CheckedAt: report.CheckedAt.Format(time.RFC3339),
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonReport)
}

func printHealthReportCompact(out io.Writer, report *reposync.HealthReport) {
	for _, health := range report.Results {
		icon := tui.FormatHealthIcon(health.HealthStatus)
		name := tui.FormatRepoName(health.Repo)

		status := ""
		switch {
		case health.AheadBy > 0 && health.BehindBy > 0:
			status = fmt.Sprintf("%d↑%d↓", health.AheadBy, health.BehindBy)
		case health.BehindBy > 0:
			status = fmt.Sprintf("%d↓", health.BehindBy)
		case health.AheadBy > 0:
			status = fmt.Sprintf("%d↑", health.AheadBy)
		default:
			status = "="
		}

		if health.WorkTreeStatus == reposync.WorkTreeDirty {
			status += " dirty"
		} else if health.WorkTreeStatus == reposync.WorkTreeConflict {
			status += " CONFLICT"
		}

		fmt.Fprintf(out, "%s %s (%s) %s\n", icon, name, health.CurrentBranch, status)
	}

	fmt.Fprintf(out, "\n%d ok, %d warn, %d err, %d unreachable\n",
		report.Summary.Healthy,
		report.Summary.Warning,
		report.Summary.Error,
		report.Summary.Unreachable,
	)
}
