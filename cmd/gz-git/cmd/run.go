// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/runtask"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Owner-bound task run lifecycle",
	Long: cliutil.QuickStartHelp(`  # Are task runtime dependencies ready?
  gz-git run doctor

  # Start one owner-bound task worktree
  gz-git run start feat-me --type feat

  # Integrate and reclaim a clean pushed task
  gz-git run finish feat-me

run owns run identity, owner, lock and receipt state. task-manager calls
these commands and never reimplements the lifecycle. Every verb answers
--json with one response document (see docs/task-run-json-schema.md);
protocol action tokens stay hyphenated ("run-finish") while the verbs
themselves are the words after "gz-git run".`),
	Args: cobra.NoArgs,
}

var (
	runDoctorJSON bool
	runStatusJSON bool
	runListJSON   bool
)

var runDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check task runtime dependencies (--json)",
	Long: cliutil.QuickStartHelp(`  gz-git run doctor [--json]

Checks task runtime configuration, integration provider presence/version/
capabilities, lock, receipt evidence readability and parseability, Worktrunk,
and schema 2 output.`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result := svc.Doctor(cmdContext(cmd))
		code := 0
		if result.Status == runtask.TaskExecutionStatusBlocked {
			code = 1
		}
		return runResult(cmd.OutOrStdout(), result, runDoctorJSON, false, code)
	},
}

var runStatusCmd = &cobra.Command{
	Use:   "status [task]",
	Short: "Show derived execution state for one task (--json)",
	Long: cliutil.QuickStartHelp(`  gz-git run status [task] [--json]

Derives one task's runtime state without acquiring the mutation lock.`),
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
		result := svc.Status(cmdContext(cmd), task)
		code := 0
		if result.Status == runtask.TaskExecutionStatusBlocked || result.Status == runtask.TaskExecutionStatusUnknown {
			code = 1
		}
		return runResult(cmd.OutOrStdout(), result, runStatusJSON, false, code)
	},
}

var runListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repository-shared task executions (--json)",
	Long: cliutil.QuickStartHelp(`  gz-git run list [--json]

Lists repository-shared task executions without acquiring the mutation lock.
Human output gives each live execution its own line (task, branch, worktree);
a BLOCKED record keeps its line and carries its reason. Only terminal
executions (DONE, ABORTED) fold into one count line. --json returns two
parallel arrays: executions is history with no status field of its own, and
states carries the derived status for those same records in the same order.`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newRunService()
		if err != nil {
			return cliutil.NewExitError(cliutil.ExitToolError, err)
		}
		result := svc.List(cmdContext(cmd))
		code := 0
		if result.Status == runtask.TaskExecutionStatusBlocked || result.Status == runtask.TaskExecutionStatusUnknown {
			code = 1
		}
		return runResult(cmd.OutOrStdout(), result, runListJSON, true, code)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.AddCommand(runDoctorCmd)
	runCmd.AddCommand(runStatusCmd)
	runCmd.AddCommand(runListCmd)
	runDoctorCmd.Flags().BoolVar(&runDoctorJSON, "json", false, "print the response document")
	runStatusCmd.Flags().BoolVar(&runStatusJSON, "json", false, "print the response document")
	runListCmd.Flags().BoolVar(&runListJSON, "json", false, "print the response document")
}

// newRunService builds the lifecycle service for the current working
// directory, with the in-process integration engine.
func newRunService() (*runtask.Service, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	return runtask.NewService(dir, runtask.NewExecutor(), runtask.NewEngine()), nil
}

// runResult renders one response document and maps the exit code. JSON is the
// consumer contract; the human surface keeps the same fields readable. A
// refusal carries the reason on stderr with the same "Error: " shape CE
// refusal paths printed, next to the response document on stdout.
func runResult(w io.Writer, result runtask.TaskResponse, jsonOutput, listMode bool, code int) error {
	if err := writeRunResponse(w, result, jsonOutput, listMode); err != nil {
		return err
	}
	if code != 0 {
		return cliutil.NewExitError(code, fmt.Errorf("Error: %s", result.Reason))
	}
	return nil
}

// writeRunResponse renders one response for the --json or human surface,
// mirroring the run-list listing rules: each live execution prints its own
// line, a BLOCKED record keeps its line and its reason, and only terminal
// records fold into a count line.
func writeRunResponse(w io.Writer, result runtask.TaskResponse, jsonOutput, listMode bool) error {
	if jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w, string(data))
		return nil
	}
	_, _ = fmt.Fprintf(w, "%s", result.Status)
	if result.Task != "" {
		_, _ = fmt.Fprintf(w, " %s", result.Task)
	}
	_, _ = fmt.Fprintf(w, ": %s\n", result.Reason)
	if result.Execution != nil {
		_, _ = fmt.Fprintf(w, "branch: %s\nworktree: %s\nowner: %s\n", result.Execution.Branch, result.Execution.Worktree, result.Execution.Owner.String())
	}
	if listMode {
		writeRunExecutionStates(w, result.States)
	}
	for _, warning := range result.Warnings {
		_, _ = fmt.Fprintf(w, "warning: %s\n", warning)
	}
	if result.NextAction != "" {
		_, _ = fmt.Fprintf(w, "next: %s\n", result.NextAction)
	}
	return nil
}

func writeRunExecutionStates(w io.Writer, states []runtask.DerivedTaskState) {
	if len(states) == 0 {
		_, _ = fmt.Fprintln(w, "executions: none")
		return
	}
	settled := map[string]int{}
	for _, state := range states {
		if runtask.IsTerminalExecutionStatus(state.Status) {
			settled[state.Status]++
			continue
		}
		_, _ = fmt.Fprintf(w, "%s %s: branch=%s worktree=%s", state.Status, state.Execution.Task, state.Execution.Branch, state.Execution.Worktree)
		if state.Status != runtask.TaskExecutionStatusActive && state.Reason != "" {
			_, _ = fmt.Fprintf(w, " reason=%s", state.Reason)
		}
		_, _ = fmt.Fprintln(w)
	}
	if len(settled) == 0 {
		return
	}
	statuses := make([]string, 0, len(settled))
	for status := range settled {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	counts := make([]string, 0, len(statuses))
	for _, status := range statuses {
		counts = append(counts, fmt.Sprintf("%s=%d", status, settled[status]))
	}
	_, _ = fmt.Fprintf(w, "settled: %s\n", strings.Join(counts, " "))
}
