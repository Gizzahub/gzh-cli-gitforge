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
	runFinishJSON          bool
	runRecoverJSON         bool
	runAbortJSON           bool
	runAbortReason         string
	runDiscardJSON         bool
	runDiscardReason       string
	runDiscardTakeOverFrom string
	runImportCEJSON        bool
	runImportCEDryRun      bool
)

var runStartCmd = &cobra.Command{
	Use:   "start <task>",
	Short: "Create an owner-bound task worktree (--type, --json)",
	Long: cliutil.QuickStartHelp(`  gz-git run start <task> --type <feat|fix|refactor|docs|test|chore|perf> [--json]

Creates one owner-bound task worktree through Worktrunk and records the
execution under the current actor/host identity.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runStartType) == "" {
			return cliutil.NewExitError(cliutil.ExitToolError, fmt.Errorf("--type is required (feat|fix|refactor|docs|test|chore|perf)"))
		}
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result := svc.Start(cmdContext(cmd), args[0], runtask.TaskExecutionType(runStartType))
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
	Short: "Close a proven partial cleanup without deleting residuals (--json)",
	Long: cliutil.QuickStartHelp(`  gz-git run recover <task> [--json]

Closes a proven exit-3 provider cleanup failure without repeating
integration or removing residual paths or refs.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
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
	Long: cliutil.QuickStartHelp(`  gz-git run discard <task> --reason R [--take-over-from actor/host] [--json]

Requires the exact owner or an explicit same-host takeover; refuses dirty
worktrees and remote branches.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runDiscardReason) == "" {
			return cliutil.NewExitError(cliutil.ExitToolError, fmt.Errorf("--reason is required to discard a task worktree"))
		}
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result, code := svc.Discard(cmdContext(cmd), args[0], runDiscardReason, runDiscardTakeOverFrom)
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
	runStartCmd.Flags().BoolVar(&runStartJSON, "json", false, "print the response document")
	runFinishCmd.Flags().BoolVar(&runFinishJSON, "json", false, "print the response document")
	runRecoverCmd.Flags().BoolVar(&runRecoverJSON, "json", false, "print the response document")
	runAbortCmd.Flags().StringVar(&runAbortReason, "reason", "", "why this execution will never integrate")
	runAbortCmd.Flags().BoolVar(&runAbortJSON, "json", false, "print the response document")
	runDiscardCmd.Flags().StringVar(&runDiscardReason, "reason", "", "why this task worktree is discarded (required)")
	runDiscardCmd.Flags().StringVar(&runDiscardTakeOverFrom, "take-over-from", "", "exact actor/host of the recorded owner, for same-host takeover")
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
