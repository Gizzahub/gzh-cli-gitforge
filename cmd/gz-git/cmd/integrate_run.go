// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
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
	integrateRunExpectTarget     string
	integrateRunLockWait         time.Duration
	integrateRunExecutionID      string
	integrateRunGeneration       string
	integrateRunOwnerActor       string
	integrateRunOwnerHost        string
	integrateRunOwnerKind        string
	integrateRunJSON             bool
)

var integrateRunCmd = &cobra.Command{
	Use:   "run [branch]",
	Short: "Fast-forward integrate and reclaim the task branch",
	Long: cliutil.QuickStartHelp(`  # Integrate the current branch
  gz-git integrate run

  # Required when no integration branch can be resolved
  gz-git integrate run --target origin/main --direct-to-default

  # Durable evidence for an external task runtime (TASK-302): all context
  # flags together, --expect-source included, or none of them
  gz-git integrate run --execution-id run-42 --generation g7 \
    --owner-actor claude --owner-host mbp --owner-kind agent \
    --expect-source <full-sha> --json

Reclaim only runs for names matching the repo-root taskPattern.
No declaration means reclaim nothing. Remote branch delete uses
--force-with-lease against the commit that just landed.
Run a bare integrate from a task-branch worktree, not from the target checkout.

Execution context (--execution-id, --generation, --owner-actor, --owner-host,
--owner-kind, plus --expect-source) records a durable receipt under the Git
common dir (gz-integrate/v1, receipt schema_version 2) before the push: intent, then pushed, then the
reclaim results. The identity is immutable — reusing an execution-id with a
different generation/owner/repo/task/target, push endpoint or protections
fails before pushing. Legacy v1 receipts are read-only. With
--json, run prints the machine protocol document (schema_version 1,
snake_case: execution_id, generation, owner{actor,host,kind}, task_sha,
target_ref, source_pushed, receipt_phase, worktree_removed,
local_branch_removed, remote_branch_removed, reclaim_*, exit_code, error),
plus the check's timing as explicit integer milliseconds:
resource_wait_duration_ms (slot admission) and validation_duration_ms
(gate runtime), both present even when zero. On a budget timeout,
readiness_failures optionally carries typed stage, class, budget_ms,
elapsed_ms, leaf_exit_code, termination_request and diagnostic_path evidence.
A diagnostic path is local to the host that ran the command and is not
portable evidence.
Readiness refusal details are printed on stderr while JSON stdout remains
one protocol document; diagnostic paths identify the retained command logs.

When a complete execution context is refused before the target push because
target ownership timed out or readiness rejected it, pre_mutation_outcome
may carry a separately durable pre-mutation-outcome/v1 binding. Its absence
does not prove that no push occurred, and a receipt of any phase prevents a
negative outcome from being reused for that execution.

Engine: gz-git-integrate (Go)

Exit Codes:
  0  integrated (reclaim finished or intentionally skipped)
  1  not ready, or the integrate itself failed
  2  usage or execution error
  3  integrated, but reclaim did not finish
  4  --expect-source/--expect-target: the approved commit pair changed; nothing was pushed`),
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
	integrateRunCmd.Flags().DurationVar(&integrateRunLockWait, "lock-wait", 0, "wait per high-load host-slot acquisition (default: 1h; 0 = unbounded; overrides branch.lockWait)")
	integrateRunCmd.Flags().StringVar(&integrateRunExpectSource, "expect-source", "", "fail with exit 4, before any measurement or push, unless the source is this full commit SHA")
	integrateRunCmd.Flags().StringVar(&integrateRunExpectTarget, "expect-target", "", "fail with exit 4, before any measurement or push, unless the target is this full commit SHA")
	integrateRunCmd.Flags().StringVar(&integrateRunExecutionID, "execution-id", "", "durable-evidence execution id; supply together with --generation, --owner-actor, --owner-host, --owner-kind and --expect-source, or not at all")
	integrateRunCmd.Flags().StringVar(&integrateRunGeneration, "generation", "", "exact runtime generation of this execution; opaque, compared exactly by receipt and cleanup")
	integrateRunCmd.Flags().StringVar(&integrateRunOwnerActor, "owner-actor", "", "logical actor executing the run (e.g. claude, celee)")
	integrateRunCmd.Flags().StringVar(&integrateRunOwnerHost, "owner-host", "", "logical device name (GIT_WORK_HOST / ce.workHost); never a hostname")
	integrateRunCmd.Flags().StringVar(&integrateRunOwnerKind, "owner-kind", "", "owner kind: agent or human")
	integrateRunCmd.Flags().BoolVar(&integrateRunJSON, "json", false, "print the machine JSON protocol document instead of the human report")
}

// executionContextFromFlags enforces the all-or-nothing rule: any context
// flag means every context flag, --expect-source included. A partial context
// is rejected before any check or mutation. The absent case is a zero value
// with supplied=false, never a nil pointer dressed as success.
func executionContextFromFlags() (execution integrate.ExecutionContext, supplied bool, err error) {
	supplied = integrateRunExecutionID != "" || integrateRunGeneration != "" ||
		integrateRunOwnerActor != "" || integrateRunOwnerHost != "" || integrateRunOwnerKind != ""
	if !supplied {
		return integrate.ExecutionContext{}, false, nil
	}
	if integrateRunExecutionID == "" || integrateRunGeneration == "" || integrateRunOwnerActor == "" ||
		integrateRunOwnerHost == "" || integrateRunOwnerKind == "" || integrateRunExpectSource == "" {
		return integrate.ExecutionContext{}, false, fmt.Errorf("--execution-id, --generation, --owner-actor, --owner-host, --owner-kind and --expect-source are supplied together or not at all")
	}
	execution = integrate.ExecutionContext{
		ExecutionID:  integrateRunExecutionID,
		Generation:   integrateRunGeneration,
		Owner:        integrate.OwnerIdentity{Actor: integrateRunOwnerActor, Host: integrateRunOwnerHost, Kind: integrateRunOwnerKind},
		ExpectSource: integrateRunExpectSource,
	}
	return execution, true, nil
}

func runIntegrateRun(cmd *cobra.Command, args []string) error {
	expectTarget, err := expectedTargetFromFlag(cmd, integrateRunExpectTarget)
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	ctx := cmdContext(cmd)
	dir, err := os.Getwd()
	if err != nil {
		return cliutil.NewExitError(cliutil.ExitToolError, err)
	}
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	}
	execution, executionSupplied, err := executionContextFromFlags()
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	var executionContext *integrate.ExecutionContext
	if executionSupplied {
		executionContext = &execution
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
			ExpectTarget:       expectTarget,
			LockWait:           integrateRunLockWait,
			LockWaitSet:        cmd.Flags().Changed("lock-wait"),
			LockNotice:         cmd.ErrOrStderr(),
		},
		Execution: executionContext,
	})
	exit := 0
	if err != nil {
		switch {
		case report != nil && report.Integrated:
			exit = cliutil.ExitReclaimIncomplete
		case errors.Is(err, integrate.ErrSourceMismatch), errors.Is(err, integrate.ErrTargetMismatch):
			exit = cliutil.ExitSourceMismatch
		case errors.Is(err, integrate.ErrImplicitSourceIsTarget) || strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "--target") || strings.Contains(err.Error(), "integration branch"):
			exit = 1
		default:
			exit = 2
		}
	} else if report != nil && report.Integrated && report.Reclaim.Incomplete() {
		err = fmt.Errorf("reclaim incomplete")
		exit = cliutil.ExitReclaimIncomplete
	}
	if !quiet {
		if integrateRunJSON {
			if report != nil && report.Check != nil && !report.Check.Ready {
				if _, printErr := fmt.Fprint(cmd.ErrOrStderr(), integrate.FormatCheck(report.Check)); printErr != nil {
					return cliutil.NewExitError(2, fmt.Errorf("write readiness failure diagnostics: %w", printErr))
				}
			}
			proto := integrate.BuildRunProtocol(report, err, exit)
			data, marshalErr := json.Marshal(proto)
			if marshalErr != nil {
				return cliutil.NewExitError(2, marshalErr)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
		} else if report != nil {
			fmt.Fprint(cmd.OutOrStdout(), integrate.FormatRun(report))
		}
	}
	return cliutil.NewExitError(exit, err)
}
