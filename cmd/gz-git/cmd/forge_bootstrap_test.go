package cmd

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/forgebootstrap"
)

func TestForgeBootstrapFlagsAndEffects(t *testing.T) {
	for _, item := range []struct {
		path  []string
		flags []string
	}{{[]string{"forge", "bootstrap", "plan"}, []string{"input", "output", "seed-dir"}}, {[]string{"forge", "bootstrap", "apply"}, []string{"plan", "confirm", "commit-model"}}} {
		command, _, err := rootCmd.Find(item.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range item.flags {
			if command.Flags().Lookup(flag) == nil {
				t.Fatalf("missing flag %s", flag)
			}
		}
	}
	if len(commandEffects["forge bootstrap plan"]) != 0 {
		t.Fatal("plan changes retained state")
	}
	for _, effect := range []string{mutatesFilesystem, mutatesRefs, mutatesRemote, mutatesForge} {
		if !slices.Contains(commandEffects["forge bootstrap apply"], effect) {
			t.Fatalf("missing apply mutation %s", effect)
		}
	}
}

func TestForgeBootstrapFrontdoorRejectsUnconfirmedAndUnauthenticatedRequests(t *testing.T) {
	planContext, applyContext := forgeBootstrapPlanCmd.Context(), forgeBootstrapApplyCmd.Context()
	forgeBootstrapPlanCmd.SetContext(t.Context())
	forgeBootstrapApplyCmd.SetContext(t.Context())
	t.Cleanup(func() { forgeBootstrapPlanCmd.SetContext(planContext); forgeBootstrapApplyCmd.SetContext(applyContext) })
	args, verify := readOnlyBootstrapPlanFixture(t)
	bootstrapRequestFile, bootstrapPlanFile, bootstrapSeed = args[4], args[6], ""
	savedConfirm, savedModel := bootstrapConfirm, bootstrapModel
	t.Cleanup(func() { bootstrapConfirm = savedConfirm; bootstrapModel = savedModel })
	var output bytes.Buffer
	forgeBootstrapPlanCmd.SetOut(&output)
	forgeBootstrapPlanCmd.SetErr(&output)
	forgeBootstrapApplyCmd.SetOut(&output)
	forgeBootstrapApplyCmd.SetErr(&output)
	t.Cleanup(func() {
		forgeBootstrapPlanCmd.SetOut(nil)
		forgeBootstrapPlanCmd.SetErr(nil)
		forgeBootstrapApplyCmd.SetOut(nil)
		forgeBootstrapApplyCmd.SetErr(nil)
	})
	if err := forgeBootstrapPlanCmd.RunE(forgeBootstrapPlanCmd, nil); err != nil {
		t.Fatal(err)
	}
	plan, err := forgebootstrap.ReadPlan(bootstrapPlanFile)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapConfirm = "unapproved-digest"
	bootstrapModel = "gpt-6.1-sol"
	if err := forgeBootstrapApplyCmd.RunE(forgeBootstrapApplyCmd, nil); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("unapproved apply error=%v", err)
	}
	verify()
	bootstrapConfirm = plan.PlanDigest
	t.Setenv("GITLAB_TOKEN", "")
	if err := forgeBootstrapApplyCmd.RunE(forgeBootstrapApplyCmd, nil); err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN") {
		t.Fatalf("unauthenticated apply error=%v", err)
	}
	if err := forgeBootstrapPlanCmd.RunE(forgeBootstrapPlanCmd, nil); err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN") {
		t.Fatalf("unauthenticated plan error=%v", err)
	}
	verify()
	bootstrapPlanFile = filepath.Join(filepath.Dir(args[6]), "missing-plan.json")
	if err := forgeBootstrapApplyCmd.RunE(forgeBootstrapApplyCmd, nil); err == nil {
		t.Fatal("missing plan accepted")
	}
	verify()
	if strings.Contains(output.String(), "readonly-plan-token") {
		t.Fatal("frontdoor output exposed authentication token")
	}
}
