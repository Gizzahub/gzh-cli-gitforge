// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// failingPlanRunner runs planBody for --result-format plan-v1 and otherwise
// records that the execution started, so a plan that should fail closed
// leaves the plan record but never the execution record.
func failingPlanRunner(planRecord, execRecord, planBody string) string {
	return "#!/bin/sh\nmode=\nprev=\nfor a; do\n  if [ \"$prev\" = --result-format ]; then mode=$a; fi\n  prev=$a\ndone\n" +
		"if [ \"$mode\" = plan-v1 ]; then\n  : > " + strconv.Quote(planRecord) + "\n" + planBody + "\nfi\n" +
		": > " + strconv.Quote(execRecord) + "\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"executed\"}'\n"
}

// TestReadinessResourcePlanFailuresFailClosed pins every plan-v1 failure path
// to an unavailable result before the execution starts, each with its own
// reason. The plan budget is injected the same way the execution budget is.
func TestReadinessResourcePlanFailuresFailClosed(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	// A valid light plan precedes the exit and timeout failures, so ignoring
	// either would start the execution instead of only changing the error.
	lightPlan := "  printf '{\"version\":1,\"resource\":\"light\",\"plan_digest\":\"" + testPlanDigest + "\",\"summary\":\"s\"}'\n"
	for _, tc := range []struct {
		name        string
		planBody    string
		planTimeout time.Duration
		reason      string
		timeout     bool
	}{
		{"nonzero plan exit", lightPlan + "  printf 'plan broke' >&2\n  exit 3", readinessPlanTimeout, "resource plan: runner failed: plan broke", false},
		// A valid light plan padded past the limit with whitespace: dropping
		// every size bound would start the execution, and the capture limit
		// rejects it before the parser's own size bound is reached.
		{"plan output over the limit", lightPlan + fmt.Sprintf("  head -c %d /dev/zero | tr '\\000' ' '\n  exit 0", readinessMaxIO), readinessPlanTimeout, "resource plan: runner output exceeded limit", false},
		{"plan timeout", lightPlan + "  sleep 30\n  exit 0", time.Second, "resource plan: runner timed out", true},
		// The later key would win a lenient decode and lower a high plan.
		{"duplicate plan keys", "  printf '{\"version\":1,\"resource\":\"high\",\"resource\":\"light\",\"plan_digest\":\"" + testPlanDigest + "\",\"summary\":\"s\"}'\n  exit 0", readinessPlanTimeout, "resource plan: runner result has duplicate or invalid fields", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			planRecord, execRecord := filepath.Join(dir, "plan"), filepath.Join(dir, "exec")
			fx := readinessFixture(t)
			advanceReadinessTarget(t, fx, planManifest(false), failingPlanRunner(planRecord, execRecord, tc.planBody), "declare failing plan")
			g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
			plan := TargetPlan{BranchSHA: gitOutput(t, fx.Worktree, "rev-parse", "HEAD"), TargetSHA: gitOutput(t, fx.Worktree, "rev-parse", "develop")}
			contract, present, err := loadReadinessContract(context.Background(), g, plan.TargetSHA)
			if err != nil || !present {
				t.Fatalf("load readiness contract: %+v, %v", contract, err)
			}
			var observed readinessObservation
			result, duration, err := runContractWithTimeoutResource(context.Background(), g, plan, contract, time.Minute, tc.planTimeout, false, CheckOptions{readinessObservation: &observed})
			if err == nil || result.Status != "unavailable" || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("result = %+v, %v; want unavailable with %q", result, err, tc.reason)
			}
			if _, err := os.Stat(planRecord); err != nil {
				t.Fatalf("plan did not run: %v", err)
			}
			if _, err := os.Stat(execRecord); !errors.Is(err, os.ErrNotExist) || duration != 0 {
				t.Fatalf("execution ran after a failed plan: %v, duration %s", err, duration)
			}
			if observed.PlanResource != "" || observed.ResourceClass != "" || observed.PlanDuration <= 0 {
				t.Fatalf("failed plan evidence = %+v", observed)
			}
			if tc.timeout && (result.failure == nil || result.failure.Stage != "contract_plan" || result.failure.Class != "timeout" || result.failure.BudgetMS != int(tc.planTimeout.Milliseconds())) {
				t.Fatalf("plan timeout failure = %+v, want contract_plan timeout at the injected budget", result.failure)
			}
		})
	}
}

// TestReadinessResourcePlanDuplicateJSONContractFailsClosed covers the JSON
// contract form: a repeated resourcePlan key rejects the contract before any
// runner invocation, plan or execution.
func TestReadinessResourcePlanDuplicateJSONContractFailsClosed(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	dir := t.TempDir()
	planRecord, execRecord := filepath.Join(dir, "plan"), filepath.Join(dir, "exec")
	fx := readinessFixture(t)
	if err := os.Remove(filepath.Join(fx.Clone, ".gz-git.yaml")); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, fx.Clone, ".gz-git.json", `{"branch":{"integrationBranch":"develop","readiness":{"version":1,"runner":".gz-git/readiness/check","resourcePlan":"plan-v1","resourcePlan":"plan-v1"}}}`)
	advanceReadinessTarget(t, fx, "", planRunner(planRecord, execRecord, readinessResourceLight, testPlanDigest), "declare duplicate JSON resourcePlan")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready || !hasCheckDetail(report, "readiness contract", checkFail, ".gz-git.json: branch.readiness must be an object") {
		t.Fatalf("duplicate JSON resourcePlan must fail closed:\n%s", FormatCheck(report))
	}
	for _, record := range []string{planRecord, execRecord} {
		if _, err := os.Stat(record); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("runner ran under a rejected contract: %s: %v", record, err)
		}
	}
}

// TestNDBuildTimeIsPreparationNotValidation pins the phase split of a planned
// ND contract: the six project-cargo builds are preparation, the plan is its
// own span, and validation is the started execution process alone.
func TestNDBuildTimeIsPreparationNotValidation(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	const buildSleep = 100 * time.Millisecond
	dependency, dep := siblingFixture(t)
	if err := os.MkdirAll(filepath.Join(dependency.Clone, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, dependency.Clone, "scripts/project-cargo", "#!/bin/sh\nsleep 0.1\n")
	if err := os.Chmod(filepath.Join(dependency.Clone, "scripts", "project-cargo"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dependency.Clone, "add", "scripts/project-cargo")
	runGit(t, dependency.Clone, "commit", "-m", "slow compiler fixture")
	runGit(t, dependency.Clone, "push", dependency.Origin, "HEAD:master")
	fx := readinessFixture(t)
	primary := filepath.Join(filepath.Dir(fx.Clone), "ndea-rs")
	runGit(t, fx.Clone, "clone", dependency.Origin, primary)
	runGit(t, primary, "checkout", "-B", "master", "origin/master")
	runGit(t, primary, "remote", "set-url", "origin", dep.Remote)
	runGit(t, primary, "remote", "set-url", "--push", "origin", dep.Remote)
	manifest := fmt.Sprintf("branch:\n  integrationBranch: develop\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    prepareProfile: nd-stack-checked-tests-v1\n    resourcePlan: plan-v1\n  siblingProvision:\n    version: 1\n    siblings:\n      - name: ndea-rs\n        remote: %q\n        path: ../ndea-rs\n        sourceBranch: master\n        revision: source-tip\n", dep.Remote)
	dir := t.TempDir()
	advanceReadinessTarget(t, fx, manifest, planRunner(filepath.Join(dir, "plan"), filepath.Join(dir, "exec"), readinessResourceLight, testPlanDigest), "declare planned ND readiness")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil || report == nil || !report.Ready {
		t.Fatalf("Check = %v\n%s", err, FormatCheck(report))
	}
	// Three build steps per side, each sleeping buildSleep.
	if report.PreparationDuration < 6*buildSleep {
		t.Fatalf("preparation duration = %s, want at least the six %s builds", report.PreparationDuration, buildSleep)
	}
	if report.PlanDuration <= 0 || report.ValidationDuration <= 0 {
		t.Fatalf("plan/validation durations = %s/%s, want both measured", report.PlanDuration, report.ValidationDuration)
	}
	// Load-robust core: validation lies inside the execution span, which
	// starts after preparation and the plan, so neither can leak into it.
	if report.ValidationDuration > report.ReadinessDuration {
		t.Fatalf("validation duration = %s exceeds the execution span %s; build or plan time leaked into validation", report.ValidationDuration, report.ReadinessDuration)
	}
}
