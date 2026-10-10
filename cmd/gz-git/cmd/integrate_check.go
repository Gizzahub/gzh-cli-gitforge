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
	integrateCheckTarget           string
	integrateCheckDirectToDefault  bool
	integrateCheckRelease          bool
	integrateCheckAllowSkipped     bool
	integrateCheckControllerConfig string
	integrateCheckNoFetch          bool
	integrateCheckExpectSource     string
	integrateCheckLockWait         time.Duration
)

var integrateCheckCmd = &cobra.Command{
	Use:   "check [branch]",
	Short: "Verify a task branch is ready to integrate",
	Long: cliutil.QuickStartHelp(`  # Check the current branch against the integration branch
  gz-git integrate check

  # Required when no integration branch can be resolved
  gz-git integrate check --target origin/main --direct-to-default

This is read-only. It never pushes and never reclaims.
Run a bare check from a task-branch worktree, not from the target checkout.

Engine: gz-git-integrate (Go)

Exit Codes:
  0  READY
  1  NOT READY, or --target required
  2  the check itself could not run
  4  --expect-source: the source is not that commit`),
	Args: cobra.MaximumNArgs(1),
	RunE: runIntegrateCheck,
}

func init() {
	integrateCmd.AddCommand(integrateCheckCmd)
	integrateCheckCmd.Flags().StringVar(&integrateCheckTarget, "target", "", "integration target (required when none can be resolved)")
	integrateCheckCmd.Flags().BoolVar(&integrateCheckDirectToDefault, "direct-to-default", false, "allow targeting the default branch when no integration branch exists")
	integrateCheckCmd.Flags().BoolVar(&integrateCheckRelease, "release", false, "promote the integration branch onto the default branch")
	integrateCheckCmd.Flags().BoolVar(&integrateCheckAllowSkipped, "allow-skipped-checks", false, "allow a repo with no check/lint gate, downgrade SKIPPED CHECK banners to warnings, and downgrade an unmeasurable baseline comparison to a warning")
	integrateCheckCmd.Flags().StringVar(&integrateCheckControllerConfig, "controller-config", "", "explicit devbox/controller config; never searched automatically")
	integrateCheckCmd.Flags().BoolVar(&integrateCheckNoFetch, "no-fetch", false, "resolve the target from local tracking refs without fetching")
	integrateCheckCmd.Flags().DurationVar(&integrateCheckLockWait, "lock-wait", 0, "give up, exit non-zero, and measure nothing if the host measurement lock is not free within this duration (default: wait without bound)")
	integrateCheckCmd.Flags().StringVar(&integrateCheckExpectSource, "expect-source", "", "fail with exit 4, before any measurement or push, unless the source is this full commit SHA")
}

func runIntegrateCheck(cmd *cobra.Command, args []string) error {
	ctx := cmdContext(cmd)
	dir, err := os.Getwd()
	if err != nil {
		return cliutil.NewExitError(cliutil.ExitToolError, err)
	}
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	}

	report, err := integrate.Check(ctx, gitcmd.NewExecutor(), integrate.CheckOptions{
		RepoPath:           dir,
		Branch:             branch,
		Target:             integrateCheckTarget,
		DirectToDefault:    integrateCheckDirectToDefault,
		Release:            integrateCheckRelease,
		AllowSkippedChecks: integrateCheckAllowSkipped,
		ControllerConfig:   integrateCheckControllerConfig,
		NoFetch:            integrateCheckNoFetch,
		ExpectSource:       integrateCheckExpectSource,
		LockWait:           integrateCheckLockWait,
		LockNotice:         cmd.ErrOrStderr(),
	})
	if err != nil {
		msg := err.Error()
		if errors.Is(err, integrate.ErrSourceMismatch) {
			if !quiet {
				fmt.Fprintln(cmd.ErrOrStderr(), "integrate check:", msg)
			}
			return cliutil.NewExitError(cliutil.ExitSourceMismatch, err)
		}
		if errors.Is(err, integrate.ErrImplicitSourceIsTarget) || strings.Contains(msg, "--target") || strings.Contains(msg, "integration branch") {
			if !quiet {
				fmt.Fprintln(cmd.ErrOrStderr(), "integrate check:", msg)
			}
			return cliutil.NewExitError(1, err)
		}
		return cliutil.NewExitError(2, err)
	}

	if !quiet {
		fmt.Fprint(cmd.OutOrStdout(), integrate.FormatCheck(report))
	}
	if !report.Ready {
		return cliutil.NewExitError(1, fmt.Errorf("not ready"))
	}
	return nil
}
