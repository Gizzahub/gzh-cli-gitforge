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
	integrateRunExpectSource     string
	integrateRunExpectTarget     string
	integrateRunExecutionID      string
	integrateRunGeneration       string
	integrateRunOwnerActor       string
	integrateRunOwnerHost        string
	integrateRunOwnerKind        string
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

  # The same run with the legacy flags, accepted for one release
  gz-git integrate run --execution-id run-42 --generation g7 \
    --owner-actor claude --owner-host mbp --owner-kind agent \
    --expect-source <full-sha> --json

Reclaim only runs for names matching the repo-root taskPattern.
No declaration means reclaim nothing. Remote branch delete uses
--force-with-lease against the commit that just landed.
Run a bare integrate from a task-branch worktree, not from the target checkout.

ADR-0161 arguments map onto the legacy flags: --source is --expect-source,
--target <ref>@<sha> is --target <ref> plus --expect-target <sha> (only a full
SHA after the last '@' pins it; a hex suffix of 7 or more characters that is
not a full SHA is refused as truncated; anything else stays a plain ref),
--record <id>@<generation> is --execution-id plus --generation, and
--owner <actor>/<host>/<kind> is the three --owner-* flags. A new argument and the legacy flag it replaces are mutually exclusive.
The legacy flags are removed in a later minor release.

Execution context (--execution-id, --generation, --owner-actor, --owner-host,
--owner-kind, plus --expect-source) records a durable receipt under the Git
common dir (gz-integrate/v1, receipt schema_version 2) before the push: intent, then pushed, then the
reclaim results. The identity is immutable — reusing an execution-id with a
different generation/owner/repo/task/target, push endpoint or protections
fails before pushing. Legacy v1 receipts are read-only. With
--json, run prints the machine protocol document (schema_version 1,
snake_case: execution_id, generation, owner{actor,host,kind}, task_sha,
target_ref, source_pushed, receipt_phase, worktree_removed,
local_branch_removed, remote_branch_removed, reclaim_*, exit_code, error).
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

When a complete execution context is refused before the target push because
readiness was rejected, pre_mutation_outcome
may carry a separately durable pre-mutation-outcome/v1 binding. Its absence
does not prove that no push occurred, and a receipt of any phase prevents a
negative outcome from being reused for that execution.

SIGINT or SIGTERM before the intent boundary (the receipt, or the push
without an execution context) stops the run, records no pre-mutation
outcome, and exits 2 with "interrupted before the target push". Past that
boundary a signal is ignored and the push, local fast-forward and reclaim
finish; only the first signal is handled, so a second one terminates gz-git
at once.

Engine: gz-git-integrate (Go)

Exit Codes:
  0  integrated (reclaim finished or intentionally skipped)
  1  not ready, or the integrate itself failed
  2  usage or execution error
  3  integrated, but reclaim did not finish
  4  --source/--expect-source or the pinned target: the approved commit pair changed; nothing was pushed`),
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
	integrateRunCmd.Flags().StringVar(&integrateRunExpectSource, "expect-source", "", "fail with exit 4, before any push, unless the source is this full commit SHA")
	integrateRunCmd.Flags().StringVar(&integrateRunExpectTarget, "expect-target", "", "fail with exit 4, before any push, unless the target is this full commit SHA")
	integrateRunCmd.Flags().StringVar(&integrateRunExecutionID, "execution-id", "", "durable-evidence execution id; supply together with --generation, --owner-actor, --owner-host, --owner-kind and --expect-source, or not at all")
	integrateRunCmd.Flags().StringVar(&integrateRunGeneration, "generation", "", "exact runtime generation of this execution; opaque, compared exactly by receipt and cleanup")
	integrateRunCmd.Flags().StringVar(&integrateRunOwnerActor, "owner-actor", "", "logical actor executing the run (e.g. claude, celee)")
	integrateRunCmd.Flags().StringVar(&integrateRunOwnerHost, "owner-host", "", "logical device name (GIT_WORK_HOST / ce.workHost); never a hostname")
	integrateRunCmd.Flags().StringVar(&integrateRunOwnerKind, "owner-kind", "", "owner kind: agent or human")
	integrateRunCmd.Flags().StringVar(&integrateRunSource, "source", "", "ADR-0161 form of --expect-source: the full commit SHA to land")
	integrateRunCmd.Flags().StringVar(&integrateRunRecord, "record", "", "ADR-0161 form of --execution-id and --generation: <id>@<generation>")
	integrateRunCmd.Flags().StringVar(&integrateRunOwner, "owner", "", "ADR-0161 form of the --owner-* flags: <actor>/<host>/<kind>")
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
		return integrate.ExecutionContext{}, false, fmt.Errorf("--record, --owner and --source (or --execution-id, --generation, --owner-actor, --owner-host, --owner-kind and --expect-source) are supplied together or not at all")
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
