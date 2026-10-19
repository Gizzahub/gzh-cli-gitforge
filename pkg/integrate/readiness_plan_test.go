// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

const testPlanDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestParseReadinessPlan(t *testing.T) {
	valid := `{"version":1,"resource":"light","plan_digest":"` + testPlanDigest + `","summary":"two checks"}`
	plan, err := parseReadinessPlan([]byte(valid))
	if err != nil || plan.Resource != readinessResourceLight || plan.PlanDigest != testPlanDigest {
		t.Fatalf("parseReadinessPlan(valid) = %+v, %v", plan, err)
	}
	for name, input := range map[string]string{
		"empty":           ``,
		"missing digest":  `{"version":1,"resource":"light","summary":"s"}`,
		"unknown field":   `{"version":1,"resource":"light","plan_digest":"` + testPlanDigest + `","summary":"s","status":"ready"}`,
		"unknown class":   `{"version":1,"resource":"medium","plan_digest":"` + testPlanDigest + `","summary":"s"}`,
		"version 2":       `{"version":2,"resource":"light","plan_digest":"` + testPlanDigest + `","summary":"s"}`,
		"uppercase hex":   `{"version":1,"resource":"high","plan_digest":"` + strings.ToUpper(testPlanDigest) + `","summary":"s"}`,
		"short digest":    `{"version":1,"resource":"high","plan_digest":"abc","summary":"s"}`,
		"digest argument": `{"version":1,"resource":"high","plan_digest":"--resource-class","summary":"s"}`,
		"trailing value":  `{"version":1,"resource":"light","plan_digest":"` + testPlanDigest + `","summary":"s"} {}`,
	} {
		if _, err := parseReadinessPlan([]byte(input)); err == nil {
			t.Errorf("%s: parseReadinessPlan accepted %q", name, input)
		}
	}
}

// planRunner answers --result-format plan-v1 with the given class and digest
// and records the held slots of both invocations plus the execution binding.
func planRunner(planRecord, execRecord, resource, digest string) string {
	return "#!/bin/sh\nprev=\nfor a; do\n  case \"$prev\" in\n    --result-format) mode=$a ;;\n    --resource-class) class=$a ;;\n    --plan-digest) bound=$a ;;\n  esac\n  prev=$a\ndone\n" +
		"if [ \"$mode\" = plan-v1 ]; then\n  printf '%s' \"$" + hostSlotsHeldEnv + "\" > " + strconv.Quote(planRecord) + "\n" +
		"  printf '{\"version\":1,\"resource\":\"" + resource + "\",\"plan_digest\":\"" + digest + "\",\"summary\":\"planned\"}'\n  exit 0\nfi\n" +
		"printf '%s|%s|%s' \"$" + hostSlotsHeldEnv + "\" \"$class\" \"$bound\" > " + strconv.Quote(execRecord) + "\n" +
		"printf '{\"version\":1,\"status\":\"ready\",\"summary\":\"selected checks passed\"}'\n"
}

func planManifest(staticHighLoad bool) string {
	m := "branch:\n  integrationBranch: develop\n"
	if staticHighLoad {
		m += "  resourceHighLoad: true\n"
	}
	return m + "  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    resourcePlan: plan-v1\n"
}

func TestReadinessResourcePlanSelectsSlot(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	for _, tc := range []struct {
		name     string
		static   bool
		resource string
		held     string
		granted  string
	}{
		{"light plan runs without the slot", false, readinessResourceLight, "", readinessResourceLight},
		{"high plan takes the slot", false, readinessResourceHigh, hostSlotName, readinessResourceHigh},
		{"static high load cannot be lowered by a light plan", true, readinessResourceLight, hostSlotName, readinessResourceHigh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			planRecord, execRecord := filepath.Join(dir, "plan"), filepath.Join(dir, "exec")
			fx := readinessFixture(t)
			advanceReadinessTarget(t, fx, planManifest(tc.static), planRunner(planRecord, execRecord, tc.resource, testPlanDigest), "declare planned readiness")
			report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
			if err != nil || report == nil || !report.Ready {
				t.Fatalf("Check = %v\n%s", err, FormatCheck(report))
			}
			if data, err := os.ReadFile(planRecord); err != nil || len(data) != 0 {
				t.Fatalf("plan must run without a held slot: %q, %v", data, err)
			}
			data, err := os.ReadFile(execRecord)
			if want := tc.held + "|" + tc.granted + "|" + testPlanDigest; err != nil || string(data) != want {
				t.Fatalf("execution binding = %q, %v; want %q", data, err, want)
			}
			if report.PlanResource != tc.resource || report.ResourceClass != tc.granted || report.PlanDigest != testPlanDigest {
				t.Fatalf("plan evidence = %q/%q/%q", report.PlanResource, report.ResourceClass, report.PlanDigest)
			}
			if tc.held == "" && report.ResourceWaitDuration != 0 {
				t.Fatalf("light execution recorded resource wait %s", report.ResourceWaitDuration)
			}
			if !strings.Contains(FormatCheck(report), "resource plan — Requested="+tc.resource+" Granted="+tc.granted) {
				t.Fatalf("human report lacks plan line:\n%s", FormatCheck(report))
			}
		})
	}
}

func TestReadinessResourcePlanInvalidFailsClosed(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	dir := t.TempDir()
	planRecord, execRecord := filepath.Join(dir, "plan"), filepath.Join(dir, "exec")
	fx := readinessFixture(t)
	advanceReadinessTarget(t, fx, planManifest(false), planRunner(planRecord, execRecord, "light", "not-a-digest"), "declare broken plan")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready || report.ReadinessStatus != "unavailable" || !hasCheckDetail(report, "readiness contract", checkFail, "resource plan") {
		t.Fatalf("invalid plan must fail closed:\n%s", FormatCheck(report))
	}
	if _, err := os.Stat(execRecord); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("execution ran after an invalid plan: %v", err)
	}
}

func TestReadinessWithoutResourcePlanPassesNoBinding(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	dir := t.TempDir()
	planRecord, execRecord := filepath.Join(dir, "plan"), filepath.Join(dir, "exec")
	fx := readinessFixture(t)
	manifest := "branch:\n  integrationBranch: develop\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n"
	advanceReadinessTarget(t, fx, manifest, planRunner(planRecord, execRecord, "high", testPlanDigest), "declare unplanned readiness")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil || !report.Ready {
		t.Fatalf("Check = %v\n%s", err, FormatCheck(report))
	}
	if _, err := os.Stat(planRecord); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("plan ran without resourcePlan: %v", err)
	}
	if data, err := os.ReadFile(execRecord); err != nil || string(data) != "||" {
		t.Fatalf("unplanned execution got a binding: %q, %v", data, err)
	}
	if report.PlanResource != "" || report.ResourceClass != readinessResourceLight {
		t.Fatalf("unplanned evidence = %q/%q", report.PlanResource, report.ResourceClass)
	}
}

func TestReadinessResourcePlanSourceOnlyChangeFailsClosed(t *testing.T) {
	fx := readinessFixture(t)
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", planManifest(false))
	runGit(t, fx.Worktree, "add", ".gz-git.yaml")
	runGit(t, fx.Worktree, "commit", "-m", "source opts into a plan")
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready || !hasCheckDetail(report, "readiness contract", checkFail, "changed") {
		t.Fatalf("source-only resourcePlan must fail closed:\n%s", FormatCheck(report))
	}
}

func TestFormatCheckReportsFailedPlan(t *testing.T) {
	out := FormatCheck(&CheckReport{PlanDuration: 1_500_000_000})
	if !strings.Contains(out, "resource plan — Requested=- Granted=- PlanDigest=- Duration=1.5s") {
		t.Fatalf("failed plan line missing:\n%s", out)
	}
	if out := FormatCheck(&CheckReport{}); strings.Contains(out, "resource plan") {
		t.Fatalf("unplanned check must not print a plan line:\n%s", out)
	}
}

func TestRunProtocolPhaseDurations(t *testing.T) {
	report := &RunReport{IntegrationDuration: 7_000_000, Check: &CheckReport{PlanDuration: 3_000_000, PreparationDuration: 5_000_000, ResourceClass: readinessResourceLight}}
	data, err := json.Marshal(BuildRunProtocol(report, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"plan_duration_ms":3`, `"preparation_duration_ms":5`, `"integration_duration_ms":7`, `"resource_class":"light"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("protocol lacks %s: %s", want, data)
		}
	}
	data, err = json.Marshal(BuildRunProtocol(nil, errors.New("boom"), 1))
	if err != nil || !strings.Contains(string(data), `"plan_duration_ms":0`) || strings.Contains(string(data), "resource_class") {
		t.Fatalf("empty protocol shape = %s, %v", data, err)
	}
}

func TestReadinessPythonUVPreparesSourceEnvironment(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	remotes, inputs := infraOpsChildRemotes(t)
	original := infraOpsDependencies
	infraOpsDependencies = remotes
	t.Cleanup(func() { infraOpsDependencies = original })
	fx := readinessFixture(t)
	writeInfraOpsFixture(t, fx.Clone, inputs)
	log := filepath.Join(t.TempDir(), "environment")
	manifest := "branch:\n  integrationBranch: develop\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    prepareProfile: infra-ops-python-uv-v1\n"
	runner := "#!/bin/sh\nfail() { printf '{\"version\":1,\"status\":\"unavailable\",\"summary\":\"%s\"}' \"$1\"; exit 0; }\n" +
		"case \"$UV_PROJECT_ENVIRONMENT\" in */infra-ops-python-uv-v1/source/venv) ;; *) fail environment ;; esac\n" +
		"test \"$VIRTUAL_ENV\" = \"$UV_PROJECT_ENVIRONMENT\" || fail virtualenv\ntest \"$UV_NO_SYNC\" = 1 || fail nosync\n" +
		"test -f \"$UV_PROJECT_ENVIRONMENT/.ready\" || fail prepared\n" +
		"printf '%s' \"$UV_PROJECT_ENVIRONMENT\" > " + strconv.Quote(log) + "\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"prepared\"}'\n"
	advanceReadinessTarget(t, fx, manifest, runner, "declare prepared readiness")
	t.Setenv("PATH", fakeInfraOpsTools(t)+":"+os.Getenv("PATH"))
	t.Setenv("UV_PROJECT_ENVIRONMENT", "/unsafe/live")
	t.Setenv("VIRTUAL_ENV", "/unsafe/live")
	t.Setenv("UV_NO_SYNC", "0")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil || !report.Ready {
		t.Fatalf("Check = %v\n%s", err, FormatCheck(report))
	}
	environment, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("runner did not record its environment: %v", err)
	}
	if _, err := os.Stat(string(environment)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("prepared environment was not reclaimed: %s: %v", environment, err)
	}
	if report.PrepareProfile != infraOpsPythonUVV1 || !strings.Contains(report.PrepareInputs, "source:uv.lock@") || !strings.Contains(report.PrepareInputs, "source:sbkube@"+inputs[1].OID) {
		t.Fatalf("readiness preparation provenance = %q/%q", report.PrepareProfile, report.PrepareInputs)
	}
	if report.PreparationDuration <= 0 {
		t.Fatalf("preparation duration not recorded: %s", report.PreparationDuration)
	}
}
