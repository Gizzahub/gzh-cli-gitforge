// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

var (
	integrateRunTarget           string
	integrateRunDirectToDefault  bool
	integrateRunRelease          bool
	integrateRunAllowSkipped     bool
	integrateRunControllerConfig string
	integrateRunNoFetch          bool
	integrateRunExpectSource     string
	integrateRunLockWait         time.Duration
)

var integrateRunCmd = &cobra.Command{
	Use:   "run [branch]",
	Short: "Fast-forward integrate and reclaim the task branch",
	Long: cliutil.QuickStartHelp(`  # Integrate the current branch
  gz-git integrate run

  # Required when no integration branch can be resolved
  gz-git integrate run --target origin/main --direct-to-default

Reclaim only runs for names matching the repo-root taskPattern.
No declaration means reclaim nothing. Remote branch delete uses
--force-with-lease against the commit that just landed.
Run a bare integrate from a task-branch worktree, not from the target checkout.

Engine: gz-git-integrate (Go)

Exit Codes:
  0  integrated (reclaim finished or intentionally skipped)
  1  not ready, or the integrate itself failed
  2  usage or execution error
  3  integrated, but reclaim did not finish
  4  --expect-source: the source is not that commit; nothing was pushed`),
	Args: cobra.MaximumNArgs(1),
	RunE: runIntegrateRun,
}

func init() {
	integrateCmd.AddCommand(integrateRunCmd)
	integrateRunCmd.Flags().StringVar(&integrateRunTarget, "target", "", "integration target (required when none can be resolved)")
	integrateRunCmd.Flags().BoolVar(&integrateRunDirectToDefault, "direct-to-default", false, "allow targeting the default branch when no integration branch exists")
	integrateRunCmd.Flags().BoolVar(&integrateRunRelease, "release", false, "promote the integration branch onto the default branch")
	integrateRunCmd.Flags().BoolVar(&integrateRunAllowSkipped, "allow-skipped-checks", false, "allow a repo with no check/lint gate, downgrade SKIPPED CHECK banners to warnings, and downgrade an unmeasurable baseline comparison to a warning")
	integrateRunCmd.Flags().StringVar(&integrateRunControllerConfig, "controller-config", "", "explicit devbox/controller config; never searched automatically")
	integrateRunCmd.Flags().BoolVar(&integrateRunNoFetch, "no-fetch", false, "integrate from local tracking refs without fetching; remote delete failures fail closed")
	integrateRunCmd.Flags().DurationVar(&integrateRunLockWait, "lock-wait", 0, "give up, exit non-zero, and measure nothing if the host measurement lock is not free within this duration (default: wait without bound)")
	integrateRunCmd.Flags().StringVar(&integrateRunExpectSource, "expect-source", "", "fail with exit 4, before any measurement or push, unless the source is this full commit SHA")
}

func runIntegrateRun(cmd *cobra.Command, args []string) error {
	ctx := cmdContext(cmd)
	dir, err := os.Getwd()
	if err != nil {
		return cliutil.NewExitError(cliutil.ExitToolError, err)
	}
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	}

	report, err := integrate.Run(ctx, gitcmd.NewExecutor(), integrate.RunOptions{
		CheckOptions: integrate.CheckOptions{
			RepoPath:           dir,
			Branch:             branch,
			Target:             integrateRunTarget,
			DirectToDefault:    integrateRunDirectToDefault,
			Release:            integrateRunRelease,
			AllowSkippedChecks: integrateRunAllowSkipped,
			ControllerConfig:   integrateRunControllerConfig,
			NoFetch:            integrateRunNoFetch,
			ExpectSource:       integrateRunExpectSource,
			LockWait:           integrateRunLockWait,
			LockNotice:         cmd.ErrOrStderr(),
		},
	})
	if report != nil && !quiet {
		fmt.Fprint(cmd.OutOrStdout(), integrate.FormatRun(report))
	}
	if err != nil {
		msg := err.Error()
		if errors.Is(err, integrate.ErrSourceMismatch) {
			return cliutil.NewExitError(cliutil.ExitSourceMismatch, err)
		}
		if errors.Is(err, integrate.ErrImplicitSourceIsTarget) || strings.Contains(msg, "not ready") || strings.Contains(msg, "--target") || strings.Contains(msg, "integration branch") {
			return cliutil.NewExitError(1, err)
		}
		return cliutil.NewExitError(2, err)
	}
	if report != nil && report.Integrated && report.Reclaim.Incomplete() {
		return cliutil.NewExitError(cliutil.ExitReclaimIncomplete, fmt.Errorf("reclaim incomplete"))
	}
	return nil
}
