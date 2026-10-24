// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/runtask"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

var (
	runStartJSON           bool
	runStartType           string
	runStartPurpose        string
	runStartResume         bool
	runFinishJSON          bool
	runRecoverJSON         bool
	runRecoverClaimDead    bool
	runAbortJSON           bool
	runAbortReason         string
	runDiscardJSON         bool
	runDiscardReason       string
	runDiscardTakeOverFrom string
	runDiscardLocalOnly    bool
	runImportCEJSON        bool
	runImportCEDryRun      bool
)

var runStartCmd = &cobra.Command{
	Use:   "start <task>",
	Short: "Create an owner-bound task worktree (--type, --purpose, --json)",
	Long: cliutil.QuickStartHelp(`  gz-git run start <task> --type <feat|fix|refactor|docs|test|chore|perf> [--purpose delivery|experiment|review] [--resume] [--json]

Creates one owner-bound task worktree through Worktrunk and records the
execution under the current actor/host identity. --resume returns an
existing execution of your own instead of refusing it; it cannot change
the recorded execution type or purpose.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runStartType) == "" {
			return cliutil.NewExitError(cliutil.ExitToolError, fmt.Errorf("--type is required (feat|fix|refactor|docs|test|chore|perf)"))
		}
		purpose := runtask.TaskExecutionPurpose(runStartPurpose)
		if runStartPurpose != "" && !purpose.Valid() {
			return cliutil.NewExitError(cliutil.ExitToolError, fmt.Errorf("--purpose must be delivery, experiment, or review"))
		}
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result := svc.StartWithPurpose(cmdContext(cmd), args[0], runtask.TaskExecutionType(runStartType), purpose, runStartResume)
		code := 0
		if result.Status == runtask.TaskExecutionStatusBlocked {
			code = 1
		}
		return runResult(cmd.OutOrStdout(), result, runStartJSON, false, code)
	},
}

var runFinishCmd = &cobra.Command{
	Use:   "finish [task]",
	Short: "Integrate and recover a clean pushed task (--json)",
	Long: cliutil.QuickStartHelp(`  gz-git run finish [task] [--json]

Verifies, integrates, and recovers a pushed clean task. Without a task
argument, exactly one ACTIVE execution is finished; zero or several leave
the choice to the caller.`),
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := ""
		if len(args) == 1 {
			task = args[0]
		}
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result, code := svc.Finish(cmdContext(cmd), task)
		return runResult(cmd.OutOrStdout(), result, runFinishJSON, false, code)
	},
}

var runRecoverCmd = &cobra.Command{
	Use:   "recover <task>",
	Short: "Close a proven partial cleanup or a dead run-finish claim (--json)",
	Long: cliutil.QuickStartHelp(`  gz-git run recover <task> [--claim-dead] [--json]

Closes a proven exit-3 provider cleanup failure without repeating
integration or removing residual paths or refs. --claim-dead recovers a
standing FINISHING claim the caller declares dead; a claim whose run-finish
process is still running on this host is refused.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		if runRecoverClaimDead {
			result, code := svc.RecoverClaimDead(cmdContext(cmd), args[0])
			return runResult(cmd.OutOrStdout(), result, runRecoverJSON, false, code)
		}
		result, code := svc.Recover(cmdContext(cmd), args[0])
		return runResult(cmd.OutOrStdout(), result, runRecoverJSON, false, code)
	},
}

var runAbortCmd = &cobra.Command{
	Use:   "abort <task>",
	Short: "Close a task execution as ABORTED without integrating it (--reason, --json)",
	Long: cliutil.QuickStartHelp(`  gz-git run abort <task> [--reason R] [--json]

Appends a terminal receipt; removes no worktree, branch, or remote ref.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result, code := svc.Abort(cmdContext(cmd), args[0], runAbortReason)
		return runResult(cmd.OutOrStdout(), result, runAbortJSON, false, code)
	},
}

var runDiscardCmd = &cobra.Command{
	Use:   "discard <task>",
	Short: "Discard a clean task worktree and branch without integration (--reason, --json)",
	Long: cliutil.QuickStartHelp(`  gz-git run discard <task> --reason R [--take-over-from actor/host] [--local-only] [--json]

Requires the exact owner or an explicit same-host takeover; refuses dirty
worktrees and remote branches. --local-only reclaims the local resources
only: no network is used and remote refs stay unchecked and unchanged.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runDiscardReason) == "" {
			return cliutil.NewExitError(cliutil.ExitToolError, fmt.Errorf("--reason is required to discard a task worktree"))
		}
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result, code := svc.DiscardWithOptions(cmdContext(cmd), args[0], runDiscardReason, runDiscardTakeOverFrom, runtask.DiscardOptions{LocalOnly: runDiscardLocalOnly})
		return runResult(cmd.OutOrStdout(), result, runDiscardJSON, false, code)
	},
}

var runImportCECmd = &cobra.Command{
	Use:   "import-ce",
	Short: "One-shot import of in-flight CE run records (--json)",
	Long: cliutil.QuickStartHelp(`  gz-git run import-ce [--dry-run] [--json]

Copies in-flight CE task-runtime records into gz-git state, byte for byte.
Refuses to merge existing gz-git state; runs once, never as a fallback.
The lifecycle itself never reads CE state.`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		report, err := svc.ImportCE(cmdContext(cmd), runImportCEDryRun)
		if err != nil {
			return runImportFailure(cmd, report, err)
		}
		if runImportCEJSON {
			return printJSON(cmd.OutOrStdout(), report)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "imported %d execution(s), %d receipt(s) from %s into %s\n",
			report.Executions, report.Receipts, report.SourceDir, report.TargetDir)
		return nil
	},
}

// runImportFailure keeps the report readable when the import refuses: the
// JSON envelope names what was (not) imported next to the failure reason.
func runImportFailure(cmd *cobra.Command, report runtask.ImportCEReport, importErr error) error {
	if runImportCEJSON {
		_ = printJSON(cmd.OutOrStdout(), report)
	}
	return cliutil.NewExitError(cliutil.ExitToolError, importErr)
}

func init() {
	runCmd.AddCommand(runStartCmd)
	runCmd.AddCommand(runFinishCmd)
	runCmd.AddCommand(runRecoverCmd)
	runCmd.AddCommand(runAbortCmd)
	runCmd.AddCommand(runDiscardCmd)
	runCmd.AddCommand(runImportCECmd)
	runStartCmd.Flags().StringVar(&runStartType, "type", "", "execution type (feat|fix|refactor|docs|test|chore|perf)")
	runStartCmd.Flags().StringVar(&runStartPurpose, "purpose", "", "execution purpose (delivery|experiment|review); default delivery")
	runStartCmd.Flags().BoolVar(&runStartResume, "resume", false, "return your own existing execution instead of refusing it")
	runStartCmd.Flags().BoolVar(&runStartJSON, "json", false, "print the response document")
	runFinishCmd.Flags().BoolVar(&runFinishJSON, "json", false, "print the response document")
	runRecoverCmd.Flags().BoolVar(&runRecoverClaimDead, "claim-dead", false, "recover a FINISHING claim whose run-finish process is declared dead")
	runRecoverCmd.Flags().BoolVar(&runRecoverJSON, "json", false, "print the response document")
	runAbortCmd.Flags().StringVar(&runAbortReason, "reason", "", "why this execution will never integrate")
	runAbortCmd.Flags().BoolVar(&runAbortJSON, "json", false, "print the response document")
	runDiscardCmd.Flags().StringVar(&runDiscardReason, "reason", "", "why this task worktree is discarded (required)")
	runDiscardCmd.Flags().StringVar(&runDiscardTakeOverFrom, "take-over-from", "", "exact actor/host of the recorded owner, for same-host takeover")
	runDiscardCmd.Flags().BoolVar(&runDiscardLocalOnly, "local-only", false, "reclaim local resources only; remote branch is not checked or changed")
	runDiscardCmd.Flags().BoolVar(&runDiscardJSON, "json", false, "print the response document")
	runImportCECmd.Flags().BoolVar(&runImportCEDryRun, "dry-run", false, "report what would be imported without writing")
	runImportCECmd.Flags().BoolVar(&runImportCEJSON, "json", false, "print the import report document")
}

func printJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
