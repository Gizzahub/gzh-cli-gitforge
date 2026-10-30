// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

var (
	integrateReceiptExecutionID string
	integrateReceiptGeneration  string
	integrateReceiptRecord      string
	integrateReceiptJSON        bool
)

var integrateReceiptCmd = &cobra.Command{
	Use:   "receipt",
	Short: "Read the durable integrate receipt for one execution",
	Long: cliutil.QuickStartHelp(`  # What does the durable evidence say about this run?
  gz-git integrate receipt --record run-42@g7

--record <id>@<generation> is --execution-id plus --generation; supply it or
the legacy pair, not both. The legacy flags are deprecated and stay for one
release.

Reads the receipt recorded under the repository's Git common dir
(gz-integrate/v1/receipts). The query is read-only: it never creates
the evidence directory or a receipt.

New receipts use schema_version 2; legacy schema_version 1 remains readable
but cannot authorize cleanup or replay. No automatic migration is performed.

Receipt JSON (snake_case): execution_id, generation,
owner{actor,host,kind}, repository_common_dir, task_branch, task_worktree,
task_sha, target_ref, target_before_sha, remote, push_remote, source_pushed,
worktree_removed, local_branch_removed, remote_branch_removed, phase,
revision, claim{pid,process_host,started_at}, provenance, updated_at.

Exit Codes:
  0  receipt found (and generation matches when given)
  2  no receipt for that execution-id, generation mismatch, or error`),
	Args: cobra.NoArgs,
	RunE: runIntegrateReceipt,
}

func init() {
	integrateCmd.AddCommand(integrateReceiptCmd)
	integrateReceiptCmd.Flags().StringVar(&integrateReceiptExecutionID, "execution-id", "", "deprecated, use --record: execution id of the run to read")
	integrateReceiptCmd.Flags().StringVar(&integrateReceiptGeneration, "generation", "", "deprecated, use --record: exact runtime generation to verify against the receipt")
	integrateReceiptCmd.Flags().StringVar(&integrateReceiptRecord, "record", "", "ADR-0161 form of --execution-id and --generation: <id>@<generation> (required unless the legacy pair is given)")
	integrateReceiptCmd.Flags().BoolVar(&integrateReceiptJSON, "json", false, "print the receipt document as JSON (default prints a human summary)")
}

func runIntegrateReceipt(cmd *cobra.Command, args []string) error {
	executionID, generation, err := executionRecordFromFlags(cmd, integrateReceiptRecord, integrateReceiptExecutionID, integrateReceiptGeneration)
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	ctx := cmdContext(cmd)
	dir, err := os.Getwd()
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	receipt, err := integrate.QueryReceipt(ctx, gitcmd.NewExecutor(), integrate.ReceiptOptions{
		RepoPath:    dir,
		ExecutionID: executionID,
		Generation:  generation,
	})
	if !quiet {
		out := cmd.OutOrStdout()
		if integrateReceiptJSON {
			data, marshalErr := json.Marshal(receipt)
			if marshalErr != nil {
				return cliutil.NewExitError(2, marshalErr)
			}
			fmt.Fprintln(out, string(data))
		} else if receipt != nil {
			fmt.Fprintf(out, "EXECUTION %s generation %s owner %s\n", receipt.ExecutionID, receipt.Generation, receipt.Owner)
			fmt.Fprintf(out, "TASK %s (%s) -> %s before %s\n", receipt.TaskBranch, receipt.TaskSHA, receipt.TargetRef, receipt.TargetBeforeSHA)
			fmt.Fprintf(out, "WORKTREE %s removed=%v local-branch removed=%v remote-branch removed=%v\n", receipt.TaskWorktree, receipt.WorktreeRemoved, receipt.LocalBranchRemoved, receipt.RemoteBranchRemoved)
			fmt.Fprintf(out, "PHASE %s source_pushed=%v revision=%d updated %s\n", receipt.Phase, receipt.SourcePushed, receipt.Revision, receipt.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
		}
	}
	if err != nil {
		if errors.Is(err, integrate.ErrCleanupReceiptMissing) || errors.Is(err, integrate.ErrCleanupGenerationMismatch) {
			return cliutil.NewExitError(2, err)
		}
		return cliutil.NewExitError(2, err)
	}
	return nil
}
