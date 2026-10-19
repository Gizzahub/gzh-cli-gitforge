// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

// integrateCmd groups queue, readiness, and integration commands.
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
fast-forwards then reclaims.

A held host resource slot is exported to gate children through
HOST_SLOTS_HELD, a cooperative comma-separated list. A child that inherits
integrate runs under its parent's slot and keeps the parent's holder record.
Each repository and target ref retains its own Git target lock.`),
	Args: cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(integrateCmd)
}

// printIntegrateProvenance writes the run provenance line before anything
// else on the notice stream (stderr), so a captured log names the device and
// the gz-git build that wrote it. It never goes to stdout: --json documents
// stay one protocol document. It ignores --quiet like the lock notices it
// attributes. Only the commands that run gates or wait on host or target
// locks print it; read-only queries and plan files do not.
func printIntegrateProvenance(cmd *cobra.Command) {
	device := integrate.ResolveDevice(cmdContext(cmd), gitcmd.NewExecutor(), ".")
	// A failed write to stderr has nowhere visible to go, and the line is
	// display only; the run itself must not fail on it.
	_ = integrate.WriteRunProvenance(cmd.ErrOrStderr(), device, appVersion)
}
