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
	legacyCleanupPlanTarget, legacyCleanupPlanOutput                 string
	legacyCleanupPlanDoneArtifact, legacyCleanupPlanReleasedArtifact string
	legacyCleanupPlanRuntimeGuard                                    string
	legacyCleanupPlanJSON                                            bool
)

var integrateLegacyCleanupPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Create a read-only prospective legacy cleanup plan",
	Args:  cobra.NoArgs,
	RunE:  runIntegrateLegacyCleanupPlan,
}

func init() {
	integrateLegacyCleanupCmd.AddCommand(integrateLegacyCleanupPlanCmd)
	f := integrateLegacyCleanupPlanCmd.Flags()
	f.StringVar(&legacyCleanupPlanTarget, "target", "", "exact integration target branch")
	f.StringVar(&legacyCleanupPlanOutput, "output", "", "absolute output plan path")
	f.StringVar(&legacyCleanupPlanDoneArtifact, "done-artifact", "", "absolute retired DONE artifact")
	f.StringVar(&legacyCleanupPlanReleasedArtifact, "released-artifact", "", "absolute retired RELEASED artifact")
	f.StringVar(&legacyCleanupPlanRuntimeGuard, "runtime-guard", "", "absolute CE legacy-cleanup guard JSON")
	f.BoolVar(&legacyCleanupPlanJSON, "json", false, "emit machine-readable result")
}

func runIntegrateLegacyCleanupPlan(cmd *cobra.Command, _ []string) error {
	fail := func(err error) error {
		if legacyCleanupPlanJSON {
			if writeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"schema_version": 1, "phase": "error", "exit_code": 2, "error": err.Error()}); writeErr != nil {
				return cliutil.NewExitError(2, writeErr)
			}
		}
		return cliutil.NewExitError(2, err)
	}
	if legacyCleanupPlanOutput == "" || legacyCleanupPlanDoneArtifact == "" || legacyCleanupPlanReleasedArtifact == "" || legacyCleanupPlanTarget == "" {
		return fail(fmt.Errorf("--target, --output, --done-artifact, and --released-artifact are required"))
	}
	root, err := os.Getwd()
	if err != nil {
		return fail(err)
	}
	owner, err := integrate.ResolveCallerOwner(cmdContext(cmd), gitcmd.NewExecutor())
	if err != nil {
		return fail(err)
	}
	var guard *integrate.LegacyRuntimeGuard
	if legacyCleanupPlanRuntimeGuard != "" {
		guard, err = integrate.ReadLegacyRuntimeGuard(legacyCleanupPlanRuntimeGuard)
		if err != nil {
			return fail(err)
		}
	}
	// Building a plan replaces the task's witness and so retires any earlier
	// plan; refuse an occupied output first so a failed write leaves it usable.
	if _, err := os.Lstat(legacyCleanupPlanOutput); err == nil {
		return fail(fmt.Errorf("plan output already exists: %s", legacyCleanupPlanOutput))
	} else if !os.IsNotExist(err) {
		return fail(err)
	}
	plan, err := integrate.BuildLegacyCleanupPlan(cmdContext(cmd), gitcmd.NewExecutor(), integrate.LegacyCleanupPlanOptions{RepoPath: root, Target: legacyCleanupPlanTarget, DoneArtifactPath: legacyCleanupPlanDoneArtifact, ReleasedArtifactPath: legacyCleanupPlanReleasedArtifact, Owner: owner, RuntimeGuard: guard})
	if err != nil {
		return fail(err)
	}
	if err := integrate.WriteLegacyCleanupPlan(legacyCleanupPlanOutput, plan); err != nil {
		return cliutil.NewExitError(2, err)
	}
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(plan); err != nil {
		return cliutil.NewExitError(2, err)
	}
	return nil
}
