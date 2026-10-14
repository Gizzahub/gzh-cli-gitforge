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
	reconcileTarget, reconcileEvidence, reconcileArtifact, reconcileExecution, reconcileGeneration, reconcileActor, reconcileHost, reconcileKind, reconcileSource string
	reconcileNoFetch, reconcileJSON                                                                                                                               bool
	integrateReconcileCmd                                                                                                                                         = &cobra.Command{Use: "reconcile [branch]", Short: "Attest a bounded legacy pre-mutation refusal", Args: cobra.MaximumNArgs(1), RunE: runIntegrateReconcile}
)

func init() {
	integrateCmd.AddCommand(integrateReconcileCmd)
	f := integrateReconcileCmd.Flags()
	f.StringVar(&reconcileTarget, "target", "", "recorded integration target")
	f.StringVar(&reconcileEvidence, "evidence-file", "", "absolute legacy evidence file")
	f.StringVar(&reconcileArtifact, "legacy-artifact", "", "absolute retained legacy artifact")
	f.StringVar(&reconcileExecution, "execution-id", "", "execution id")
	f.StringVar(&reconcileGeneration, "generation", "", "generation")
	f.StringVar(&reconcileActor, "owner-actor", "", "owner actor")
	f.StringVar(&reconcileHost, "owner-host", "", "owner host")
	f.StringVar(&reconcileKind, "owner-kind", "", "owner kind")
	f.StringVar(&reconcileSource, "expect-source", "", "approved source SHA")
	f.BoolVar(&reconcileNoFetch, "no-fetch", false, "record original no-fetch invocation metadata")
	f.BoolVar(&reconcileJSON, "json", false, "print JSON")
}

func runIntegrateReconcile(cmd *cobra.Command, args []string) error {
	if !reconcileJSON || reconcileTarget == "" || reconcileEvidence == "" || reconcileArtifact == "" || reconcileExecution == "" || reconcileGeneration == "" || reconcileActor == "" || reconcileHost == "" || reconcileKind == "" || reconcileSource == "" {
		return cliutil.NewExitError(2, fmt.Errorf("--target, --evidence-file, --legacy-artifact, complete execution context, --expect-source, and --json are required"))
	}
	dir, err := os.Getwd()
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	}
	r, err := integrate.ReconcileLegacyPreMutation(cmdContext(cmd), gitcmd.NewExecutor(), integrate.LegacyReconcileOptions{RepoPath: dir, Branch: branch, Target: reconcileTarget, EvidenceFile: reconcileEvidence, LegacyArtifact: reconcileArtifact, NoFetch: reconcileNoFetch, Execution: &integrate.ExecutionContext{ExecutionID: reconcileExecution, Generation: reconcileGeneration, Owner: integrate.OwnerIdentity{Actor: reconcileActor, Host: reconcileHost, Kind: reconcileKind}, ExpectSource: reconcileSource}})
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	data, err := json.Marshal(integrate.BuildLegacyReconcileProtocol(r))
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return cliutil.NewExitError(0, err)
}
