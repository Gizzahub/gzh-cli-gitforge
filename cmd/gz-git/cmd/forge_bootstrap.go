package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/forgebootstrap"
	"github.com/spf13/cobra"
)

var (
	bootstrapRequestFile, bootstrapPlanFile, bootstrapConfirm string
	forgeBootstrapPlanCmd                                     = &cobra.Command{Use: "plan", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		b, e := os.ReadFile(bootstrapRequestFile)
		if e != nil {
			return e
		}
		var r forgebootstrap.Request
		if e = json.Unmarshal(b, &r); e != nil {
			return e
		}
		p, e := forgebootstrap.NewPlan(r, time.Now())
		if e != nil {
			return e
		}
		if e = forgebootstrap.WritePlan(bootstrapPlanFile, p); e != nil {
			return e
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "PLAN_DIGEST %s\n", p.PlanDigest)
		return nil
	}}
)

var forgeBootstrapApplyCmd = &cobra.Command{Use: "apply", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	p, e := forgebootstrap.ReadPlan(bootstrapPlanFile)
	if e != nil {
		return e
	}
	if bootstrapConfirm != p.PlanDigest {
		return fmt.Errorf("--confirm must match plan digest")
	}
	token := os.Getenv("GITLAB_TOKEN")
	remote, e := forgebootstrap.NewGitLabRemote(p.Request.GitLabURL, token)
	if e != nil {
		return e
	}
	rec, e := forgebootstrap.Apply(context.Background(), p, forgebootstrap.ApplyOptions{Remote: remote, ReceiptPath: p.Request.Checkout + ".bootstrap-receipt.json"})
	if e != nil {
		return e
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(rec)
}}
var forgeBootstrapCmd = &cobra.Command{Use: "bootstrap", Short: "Bootstrap an independent repository from a pinned template"}

func init() {
	forgeBootstrapCmd.AddCommand(forgeBootstrapPlanCmd, forgeBootstrapApplyCmd)
	forgeBootstrapPlanCmd.Flags().StringVar(&bootstrapRequestFile, "input", "", "request JSON file")
	forgeBootstrapPlanCmd.Flags().StringVarP(&bootstrapPlanFile, "output", "o", "", "plan JSON output")
	forgeBootstrapApplyCmd.Flags().StringVar(&bootstrapPlanFile, "plan", "", "plan JSON file")
	forgeBootstrapApplyCmd.Flags().StringVar(&bootstrapConfirm, "confirm", "", "plan digest acknowledgement")
	for _, c := range rootCmd.Commands() {
		if c.Name() == "forge" {
			c.AddCommand(forgeBootstrapCmd)
			break
		}
	}
}
