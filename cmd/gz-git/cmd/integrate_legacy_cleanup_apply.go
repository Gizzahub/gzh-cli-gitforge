package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
	"github.com/spf13/cobra"
)

var (
	legacyCleanupApplyPlan    string
	legacyCleanupApplyDigest  string
	legacyCleanupApplyAttempt string
	legacyCleanupApplyJSON    bool
)

var integrateLegacyCleanupApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a prospective legacy cleanup plan",
	Args:  cobra.NoArgs,
	RunE:  runIntegrateLegacyCleanupApply,
}

func init() {
	integrateLegacyCleanupCmd.AddCommand(integrateLegacyCleanupApplyCmd)
	integrateLegacyCleanupApplyCmd.Flags().StringVar(&legacyCleanupApplyPlan, "plan", "", "absolute prospective plan path")
	integrateLegacyCleanupApplyCmd.Flags().StringVar(&legacyCleanupApplyDigest, "approve-digest", "", "exact plan digest approval")
	integrateLegacyCleanupApplyCmd.Flags().StringVar(&legacyCleanupApplyAttempt, "runtime-attempt", "", "exact CE runtime attestation attempt nonce")
	integrateLegacyCleanupApplyCmd.Flags().BoolVar(&legacyCleanupApplyJSON, "json", false, "emit machine-readable result")
}

func runIntegrateLegacyCleanupApply(cmd *cobra.Command, _ []string) error {
	fail := func(err error, code int) error {
		if legacyCleanupApplyJSON {
			payload := map[string]any{"schema_version": 1, "phase": "error", "exit_code": code, "error": err.Error()}
			if writeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(payload); writeErr != nil {
				return cliutil.NewExitError(code, writeErr)
			}
		}
		return cliutil.NewExitError(code, err)
	}
	if legacyCleanupApplyPlan == "" {
		return fail(fmt.Errorf("--plan is required"), 2)
	}
	if _, err := os.Stat(legacyCleanupApplyPlan); err != nil {
		return fail(err, 2)
	}
	err := integrate.ApplyLegacyCleanup(cmdContext(cmd), gitcmd.NewExecutor(), integrate.LegacyCleanupApplyOptions{PlanPath: legacyCleanupApplyPlan, ApproveDigest: legacyCleanupApplyDigest, RuntimeAttempt: legacyCleanupApplyAttempt})
	if err != nil {
		return fail(err, integrate.LegacyCleanupExitCode(err))
	}
	if legacyCleanupApplyJSON {
		err = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"schema_version": 1, "phase": "complete", "exit_code": 0})
	}
	return err
}
