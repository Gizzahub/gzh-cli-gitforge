package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/forgebootstrap"
)

var (
	bootstrapRequestFile, bootstrapPlanFile, bootstrapConfirm, bootstrapModel, bootstrapSeed string
	forgeBootstrapPlanCmd                                                                    = &cobra.Command{Use: "plan", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var r forgebootstrap.Request
		if e := forgebootstrap.ReadJSONFile(bootstrapRequestFile, &r); e != nil {
			return e
		}
		if bootstrapSeed != "" {
			r.SeedDir = bootstrapSeed
			r.TemplateURL = ""
			r.TemplateSHA = ""
		}
		p, e := forgebootstrap.NewPlan(r, time.Now())
		if e != nil {
			return e
		}
		if e := forgebootstrap.ValidateGitLabRequest(p.Request); e != nil {
			return e
		}
		remote, e := forgebootstrap.NewGitLabRemote(p.Request.GitLabURL, os.Getenv("GITLAB_TOKEN"))
		if e != nil {
			return e
		}
		state, e := remote.Inspect(cmd.Context(), p.Request)
		if e != nil {
			return e
		}
		if state.Exists && !state.Empty {
			return fmt.Errorf("destination repository contains content")
		}
		p.RemoteState = state
		p.PlanDigest = forgebootstrap.Digest(p)
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
	if e := forgebootstrap.ValidateGitLabRequest(p.Request); e != nil {
		return e
	}
	token := os.Getenv("GITLAB_TOKEN")
	remote, e := forgebootstrap.NewGitLabRemote(p.Request.GitLabURL, token)
	if e != nil {
		return e
	}
	rec, e := forgebootstrap.Apply(cmd.Context(), p, forgebootstrap.ApplyOptions{Remote: remote, CommitModel: bootstrapModel, ReceiptPath: p.Request.Checkout + ".bootstrap-receipt.json"})
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
	forgeBootstrapPlanCmd.Flags().StringVar(&bootstrapSeed, "seed-dir", "", "initial template only: prepared local source tree instead of URL/SHA")
	forgeBootstrapApplyCmd.Flags().StringVar(&bootstrapModel, "commit-model", "", "explicit versioned runtime identity for the initial commit footer")
	forgeBootstrapApplyCmd.Flags().StringVar(&bootstrapConfirm, "confirm", "", "plan digest acknowledgement")
	for _, c := range rootCmd.Commands() {
		if c.Name() == "forge" {
			c.AddCommand(forgeBootstrapCmd)
			break
		}
	}
}
