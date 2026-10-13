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
	integrateCleanupExecutionID string
	integrateCleanupGeneration  string
	integrateCleanupJSON        bool
)

var integrateCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup-only retry for an evidenced integrate run",
	Long: cliutil.QuickStartHelp(`  # Finish the reclaim half of a run that crashed after its push
  gz-git integrate cleanup --execution-id run-42 --generation g7

Reclaims the task worktree, local branch, and remote task branch recorded
in the durable receipt, without re-integrating: cleanup never runs checks,
readiness, gates, watchers, merge, or a target push. It acquires the same
repo/target lock, verifies the exact generation and that the caller resolves
to the receipt's owner (GIT_WORK_ACTOR/GZ_GIT_TASK_ACTOR + GIT_WORK_HOST /
ce.workHost — never a hostname), refuses while the receipt's holder is
alive or of unknown liveness (an elapsed lease never authorizes stealing),
proves the remote target actually contains the task SHA, and validates every
residual identity — branch at the captured SHA, registration-marked clean
worktree — before removing anything. A mismatch, a dirty tree, a foreign or
newer ref, or an unprovable fact preserves all residuals.

Cleanup JSON (--json, schema_version 1, snake_case): execution_id,
generation, owner{actor,host,kind}, task_sha, target_ref, source_pushed,
remote_target_contains_task, worktree_removed, local_branch_removed,
remote_branch_removed, phase, complete, actions, residuals_preserved,
exit_code, error.

Exit Codes:
  0  complete: every residual removed or proven already absent
  3  partial: some removals recorded durably; residuals preserved
  2  refusal or error (owner/generation mismatch, live or unknown holder,
     unprovable target proof, unpushed run, moved refs, dirty or foreign
     worktree, missing receipt)`),
	Args: cobra.NoArgs,
	RunE: runIntegrateCleanup,
}

func init() {
	integrateCmd.AddCommand(integrateCleanupCmd)
	integrateCleanupCmd.Flags().StringVar(&integrateCleanupExecutionID, "execution-id", "", "execution id of the run to finish reclaiming (required)")
	integrateCleanupCmd.Flags().StringVar(&integrateCleanupGeneration, "generation", "", "exact runtime generation of that execution (required)")
	integrateCleanupCmd.Flags().BoolVar(&integrateCleanupJSON, "json", false, "print the machine JSON protocol document instead of the human report")
	_ = integrateCleanupCmd.MarkFlagRequired("execution-id")
	_ = integrateCleanupCmd.MarkFlagRequired("generation")
}

func runIntegrateCleanup(cmd *cobra.Command, args []string) error {
	ctx := cmdContext(cmd)
	dir, err := os.Getwd()
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	report, err := integrate.CleanupRetry(ctx, gitcmd.NewExecutor(), integrate.CleanupOptions{
		RepoPath:    dir,
		ExecutionID: integrateCleanupExecutionID,
		Generation:  integrateCleanupGeneration,
		LockNotice:  cmd.ErrOrStderr(),
	})
	exit := integrate.CleanupExitCode(err)
	if !quiet {
		out := cmd.OutOrStdout()
		if integrateCleanupJSON {
			proto := integrate.BuildCleanupProtocol(report, err, exit)
			data, marshalErr := json.Marshal(proto)
			if marshalErr != nil {
				return cliutil.NewExitError(2, marshalErr)
			}
			fmt.Fprintln(out, string(data))
		} else if report != nil && report.Receipt != nil {
			for _, action := range report.Actions {
				fmt.Fprintf(out, "CLEANED %s\n", action)
			}
			if report.Complete {
				fmt.Fprintf(out, "CLEANUP complete execution %s generation %s\n", report.Receipt.ExecutionID, report.Receipt.Generation)
			}
			for _, residual := range report.ResidualsPreserved {
				fmt.Fprintf(out, "PRESERVED %s\n", residual)
			}
		}
	}
	return cliutil.NewExitError(exit, err)
}
