package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

var (
	readinessUpdateBranch, readinessUpdateTarget, readinessUpdateIssuer, readinessUpdateOutput string
	readinessUpdateExpiry                                                                      time.Duration
	readinessUpdatePlanFile, readinessUpdateConfirm                                            string
)

var (
	integrateReadinessCmd           = &cobra.Command{Use: "readiness", Short: "Manage target-owned readiness contracts"}
	integrateReadinessUpdateCmd     = &cobra.Command{Use: "update", Short: "Update a target-owned readiness contract"}
	integrateReadinessUpdatePlanCmd = &cobra.Command{
		Use: "plan", Short: "Create a read-only, expiring readiness-update plan", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, exec := cmdContext(cmd), gitcmd.NewExecutor()
			issuer, err := resolveIssuer(ctx, exec, ".", readinessUpdateIssuer)
			if err != nil {
				return cliutil.NewExitError(2, err)
			}
			p, err := integrate.ReadinessUpdatePlanFor(ctx, exec, integrate.ReadinessUpdateOptions{RepoPath: ".", Branch: readinessUpdateBranch, Target: readinessUpdateTarget, Issuer: issuer, Expiry: readinessUpdateExpiry})
			if err != nil {
				return cliutil.NewExitError(2, err)
			}
			if err := integrate.WriteReadinessUpdatePlan(readinessUpdateOutput, p); err != nil {
				return cliutil.NewExitError(2, err)
			}
			digest := integrate.ReadinessUpdatePlanDigest(p)
			if digest == "" {
				return cliutil.NewExitError(2, fmt.Errorf("encode readiness update plan digest"))
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "CONFIRM_DIGEST %s\n", digest)
			if readinessUpdateOutput == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "next: re-run with --output <file>, then apply --plan <file> --confirm %s\n", digest)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "next: gz-git integrate readiness update apply --plan %s --confirm %s\n", readinessUpdateOutput, digest)
			}
			return nil
		},
	}
)

var integrateReadinessUpdateApplyCmd = &cobra.Command{
	Use: "apply", Short: "Apply an exact readiness-update plan with a lease", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// A policy-contract change is deliberately not automatable. The digest is
		// a human review acknowledgement, and a pipe/CI runner has no human at
		// that boundary. There is intentionally no --yes or environment bypass.
		if !stdinIsInteractive() {
			return cliutil.NewExitError(2, fmt.Errorf("readiness update apply requires an interactive terminal"))
		}
		if readinessUpdatePlanFile == "" {
			return cliutil.NewExitError(2, fmt.Errorf("--plan is required"))
		}
		p, err := integrate.ReadReadinessUpdatePlan(readinessUpdatePlanFile)
		if err != nil {
			return cliutil.NewExitError(2, err)
		}
		if err := integrate.ReadinessUpdateApply(cmdContext(cmd), gitcmd.NewExecutor(), p, ".", readinessUpdateConfirm); err != nil {
			return cliutil.NewExitError(1, err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "READINESS_UPDATED", p.TargetRef, p.SourceSHA)
		return nil
	},
}

func init() {
	integrateCmd.AddCommand(integrateReadinessCmd)
	integrateReadinessCmd.AddCommand(integrateReadinessUpdateCmd)
	integrateReadinessUpdateCmd.AddCommand(integrateReadinessUpdatePlanCmd, integrateReadinessUpdateApplyCmd)
	integrateReadinessUpdatePlanCmd.Flags().StringVar(&readinessUpdateBranch, "branch", "", "source branch")
	integrateReadinessUpdatePlanCmd.Flags().StringVar(&readinessUpdateTarget, "target", "", "target ref")
	integrateReadinessUpdatePlanCmd.Flags().StringVar(&readinessUpdateIssuer, "issuer", "", "identity recorded in the plan (default: git config gzgit.issuer, then user.email)")
	integrateReadinessUpdatePlanCmd.Flags().DurationVar(&readinessUpdateExpiry, "expires-in", 15*time.Minute, "plan lifetime")
	integrateReadinessUpdatePlanCmd.Flags().StringVarP(&readinessUpdateOutput, "output", "o", "", "write plan JSON to this file (stdout when omitted)")
	integrateReadinessUpdateApplyCmd.Flags().StringVar(&readinessUpdatePlanFile, "plan", "", "plan JSON file")
	integrateReadinessUpdateApplyCmd.Flags().StringVar(&readinessUpdateConfirm, "confirm", "", "explicit human confirmation: canonical plan digest")
}
