// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

// integrateCmd groups queue, check, and integration commands.
var integrateCmd = &cobra.Command{
	Use:   "integrate",
	Short: "Inspect and apply task-branch integration",
	Long: cliutil.QuickStartHelp(`  # What is waiting to land?
  gz-git integrate queue

  # Is this branch ready?
  gz-git integrate check

  # Fast-forward the target and reclaim the task branch
  gz-git integrate run

integrate answers a different question than branch list. queue lists
unfinished task branches, check is the readiness gate, and run
fast-forwards then reclaims. gz-git builds, runs, and lints no
repository code: git checks alone decide readiness.

Each repository and target ref has its own Git target lock. A command
waits for it until the lock is free or the command is interrupted.`),
	Args: cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(integrateCmd)
}

// printIntegrateProvenance writes the run provenance line before anything
// else on the notice stream (stderr), so a captured log names the device and
// the gz-git build that wrote it. It never goes to stdout: --json documents
// stay one protocol document. It ignores --quiet like the lock notices it
// attributes. Only the commands that wait on git-mutation locks print it; read-only queries and plan files do not.
func printIntegrateProvenance(cmd *cobra.Command) {
	device := integrate.ResolveDevice(cmdContext(cmd), gitcmd.NewExecutor(), ".")
	// A failed write to stderr has nowhere visible to go, and the line is
	// display only; the run itself must not fail on it.
	_ = integrate.WriteRunProvenance(cmd.ErrOrStderr(), device, appVersion)
}
