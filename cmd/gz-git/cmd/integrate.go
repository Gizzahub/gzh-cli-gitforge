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

gz-git never makes one integration wait for another: it detects and
reports. Concurrent runs on one target are not ordered; the exact-lease
push lets one land, and every other run refuses at once, naming the
target, the commit it checked, the current commit, and the rebase to do.
Two commands never reclaim one task's worktree and branches at once: the
second refuses immediately and names the holding process.`),
	Args: cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(integrateCmd)
}

// printIntegrateProvenance writes the run provenance line before anything
// else on the notice stream (stderr), so a captured log names the device and
// the gz-git build that wrote it. It never goes to stdout: --json documents
// stay one protocol document. It ignores --quiet so a refusal on stderr is
// always attributed. Only the commands that mutate git print it; read-only queries and plan files do not.
func printIntegrateProvenance(cmd *cobra.Command) {
	device := integrate.ResolveDevice(cmdContext(cmd), gitcmd.NewExecutor(), ".")
	// A failed write to stderr has nowhere visible to go, and the line is
	// display only; the run itself must not fail on it.
	_ = integrate.WriteRunProvenance(cmd.ErrOrStderr(), device, appVersion)
}
