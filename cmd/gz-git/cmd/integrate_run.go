// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

var (
	integrateRunTarget           string
	integrateRunDirectToDefault  bool
	integrateRunRelease          bool
	integrateRunControllerConfig string
	integrateRunNoFetch          bool
	integrateRunSource           string
	integrateRunRecord           string
	integrateRunOwner            string
	integrateRunJSON             bool
)

var integrateRunCmd = &cobra.Command{
	Use:   "run [branch]",
	Short: "Fast-forward integrate and reclaim the task branch",
	Long: cliutil.QuickStartHelp(`  # Integrate the current branch
  gz-git integrate run

  # Required when no integration branch can be resolved
  gz-git integrate run --target origin/main --direct-to-default

  # Durable evidence for an external task runtime (ADR-0161 arguments):
  # the pinned pair, the record and the owner together, or none of them
  gz-git integrate run dev/claude/mbp/feat/x --source <full-sha> \
    --target master@<full-sha> --record run-42@g7 \
    --owner claude/mbp/agent --json

Reclaim only runs for names matching the repo-root taskPattern.
No declaration means reclaim nothing. Remote branch delete uses
--force-with-lease against the commit that just landed.
Run a bare integrate from a task-branch worktree, not from the target checkout.

The pinned target --target <ref>@<sha> accepts only a full SHA after the last
'@'; a hex suffix of 7 or more characters that is not a full SHA is refused as
truncated, and anything else stays a plain ref.

Execution context (--record, --owner, plus --source) records a durable
receipt under the Git common dir (gz-integrate/v1, receipt schema_version 2)
before the push: intent, then pushed, then the reclaim results. The identity
is immutable — reusing a record id with a different generation/owner/repo/
task/target, push endpoint or protections fails before pushing. Legacy v1
receipts are read-only. With --json, run prints the machine protocol document
(schema_version 1, snake_case: execution_id, generation,
owner{actor,host,kind}, task_sha, target_ref, source_pushed, receipt_phase,
worktree_removed, local_branch_removed, remote_branch_removed, reclaim_*,
exit_code, error).
resource_wait_duration_ms, validation_duration_ms, plan_duration_ms and
preparation_duration_ms stay in the document as explicit zeros: gz-git runs
no repository code, so nothing is admitted, validated or prepared.
Without an execution context, source_pushed and the three *_removed fields
report positive outcomes observed by this invocation; they do not create a
durable receipt or owner identity. With a receipt, its fields are authoritative.
False is not proof that an operation did not occur: positive evidence may be
unavailable. Verify actual Git/remote/filesystem state when reconciling a run.
Readiness refusal details are printed on stderr while JSON stdout remains
one protocol document.

SIGINT or SIGTERM before the intent boundary (the receipt, or the push
without an execution context) stops the run and exits 2 with "interrupted
before the target push". Past that boundary a signal is ignored and the push,
local fast-forward and reclaim finish; only the first signal is handled, so a
second one terminates gz-git at once.

Engine: gz-git-integrate (Go)

Exit Codes:
  0  integrated (reclaim finished or intentionally skipped)
  1  not ready, or the integrate itself failed
  2  usage or execution error
  3  integrated, but reclaim did not finish
  4  --source or the pinned target: the approved commit pair changed; nothing was pushed`),
	Args: cobra.MaximumNArgs(1),
	RunE: runIntegrateRun,
}

func init() {
	integrateCmd.AddCommand(integrateRunCmd)
	integrateRunCmd.Flags().StringVar(&integrateRunTarget, "target", "", "integration target (required when none can be resolved)")
	integrateRunCmd.Flags().BoolVar(&integrateRunDirectToDefault, "direct-to-default", false, "allow targeting the default branch when no integration branch exists")
	integrateRunCmd.Flags().BoolVar(&integrateRunRelease, "release", false, "promote the integration branch onto the default branch")
	integrateRunCmd.Flags().StringVar(&integrateRunControllerConfig, "controller-config", "", "explicit devbox/controller config; never searched automatically")
	integrateRunCmd.Flags().BoolVar(&integrateRunNoFetch, "no-fetch", false, "integrate from local tracking refs without fetching; remote delete failures fail closed")
	integrateRunCmd.Flags().StringVar(&integrateRunSource, "source", "", "full commit SHA of the source to land; supplied together with --record and --owner, or not at all")
	integrateRunCmd.Flags().StringVar(&integrateRunRecord, "record", "", "durable-evidence record: <id>@<generation>")
	integrateRunCmd.Flags().StringVar(&integrateRunOwner, "owner", "", "execution owner: <actor>/<host>/<kind>")
	integrateRunCmd.Flags().BoolVar(&integrateRunJSON, "json", false, "print the machine JSON protocol document instead of the human report")
}

// executionContextFromArgs enforces the all-or-nothing rule: any context
// field means every context field, the expected source included. A partial
// context is rejected before any check or mutation. The absent case is a
// zero value with supplied=false, never a nil pointer dressed as success.
func executionContextFromArgs(a integrateRunArgs) (execution integrate.ExecutionContext, supplied bool, err error) {
	supplied = a.ExecutionID != "" || a.Generation != "" ||
		a.OwnerActor != "" || a.OwnerHost != "" || a.OwnerKind != ""
	if !supplied {
		return integrate.ExecutionContext{}, false, nil
	}
	if a.ExecutionID == "" || a.Generation == "" || a.OwnerActor == "" ||
		a.OwnerHost == "" || a.OwnerKind == "" || a.ExpectSource == "" {
		return integrate.ExecutionContext{}, false, fmt.Errorf("--record, --owner and --source are supplied together or not at all")
	}
	execution = integrate.ExecutionContext{
		ExecutionID:  a.ExecutionID,
		Generation:   a.Generation,
		Owner:        integrate.OwnerIdentity{Actor: a.OwnerActor, Host: a.OwnerHost, Kind: a.OwnerKind},
		ExpectSource: a.ExpectSource,
	}
	return execution, true, nil
}

func runIntegrateRun(cmd *cobra.Command, args []string) error {
	printIntegrateProvenance(cmd)
	runArgs, err := integrateRunArgsFromFlags(cmd)
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	// A signal cancels the run before the intent boundary; integrate.Run
	// ignores it past that boundary.
	ctx, cancel := withInterruptCancel(cmdContext(cmd))
	defer cancel()
	dir, err := os.Getwd()
	if err != nil {
		return cliutil.NewExitError(cliutil.ExitToolError, err)
	}
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	}
	execution, executionSupplied, err := executionContextFromArgs(runArgs)
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	var executionContext *integrate.ExecutionContext
	if executionSupplied {
		executionContext = &execution
	}

	report, err := integrate.Run(ctx, gitcmd.NewExecutor(), integrate.RunOptions{
		CheckOptions: integrate.CheckOptions{
			RepoPath:         dir,
			Branch:           branch,
			Target:           runArgs.Target,
			DirectToDefault:  integrateRunDirectToDefault,
			Release:          integrateRunRelease,
			ControllerConfig: integrateRunControllerConfig,
			NoFetch:          integrateRunNoFetch,
			ExpectSource:     runArgs.ExpectSource,
			ExpectTarget:     runArgs.ExpectTarget,
		},
		Execution: executionContext,
	})
	exit := integrate.RunExitCode(report, err)
	if err == nil && report != nil && report.Integrated && report.Reclaim.Incomplete() {
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
