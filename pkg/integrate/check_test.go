// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestCheck_UndeclaredTargetsOriginHead(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   "feature/worktree",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Plan.Target != "origin/main" {
		t.Fatalf("target = %q, want origin/main", report.Plan.Target)
	}
	if report.Plan.Integration.Name != "main" || report.Plan.Integration.Source != SourceHeuristic {
		t.Fatalf("integration = %+v, want heuristic main", report.Plan.Integration)
	}
	if report.Warnings == 0 || !hasCheckDetail(report, "integration target", checkWarn, "inferred from the remote HEAD") {
		t.Fatalf("heuristic target must remain usable but visible:\n%s", FormatCheck(report))
	}
	if got := FormatCheck(report); !strings.Contains(got, "integration=main (heuristic: remote HEAD)") {
		t.Fatalf("formatted integration source = %q", got)
	}
}

func TestCheck_HeuristicTargetWithRenamedRemote(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Worktree, "remote", "rename", "origin", "upstream")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "feature/worktree"})
	if err != nil {
		t.Fatalf("Check with renamed remote: %v", err)
	}
	if report.Plan.Target != "upstream/main" || report.Plan.Integration.Source != SourceHeuristic {
		t.Fatalf("unexpected heuristic plan: %+v", report.Plan)
	}
	formatted := FormatCheck(report)
	if !strings.Contains(formatted, "integration=main (heuristic: remote HEAD)") || strings.Contains(formatted, "origin/HEAD") {
		t.Fatalf("resolution must describe the actual remote fallback without assuming origin: %s", formatted)
	}
}

func TestCheck_ReadyWhenFreshCleanPushed(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   "dev/actor/feat/task",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready {
		t.Fatalf("want READY, got\n%s", FormatCheck(report))
	}
	if got := FormatCheck(report); !strings.Contains(got, "integration=develop (config[0])") {
		t.Fatalf("declared integration source missing from check report: %q", got)
	}
}

func TestCheck_IntegrationUpstreamHasSafeRemediation(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	branch := "dev/actor/feat/task"
	runGit(t, fx.Worktree, "branch", "--set-upstream-to", fx.Remote+"/develop", branch)

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   branch,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, item := range report.Items {
		if item.Name != "upstream" {
			continue
		}
		if item.Status != checkFail || !strings.Contains(item.Detail, "targets integration branch develop") {
			t.Fatalf("upstream item = %+v", item)
		}
		if strings.Contains(item.Detail, "— git push\n") || strings.HasSuffix(item.Detail, "— git push") {
			t.Fatalf("unsafe bare push remediation: %q", item.Detail)
		}
		if !strings.Contains(item.Detail, "git branch --set-upstream-to="+fx.Remote+"/"+branch) {
			t.Fatalf("missing local tracking repair for existing task ref: %q", item.Detail)
		}
		return
	}
	t.Fatalf("missing upstream failure:\n%s", FormatCheck(report))
}

func TestCheck_IntegrationUpstreamUsesExplicitRefspecWhenTaskRefMissing(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"upstream", "tracking"} {
		t.Run(mode, func(t *testing.T) {
			fx := readyTaskFixture(t)
			branch := "dev/actor/feat/task"
			runGit(t, fx.Worktree, "push", fx.Remote, "--delete", branch)
			runGit(t, fx.Worktree, "branch", "--set-upstream-to", fx.Remote+"/develop", branch)
			runGit(t, fx.Worktree, "config", "push.default", mode)
			integrationBefore := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")

			report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
				RepoPath: fx.Worktree,
				Branch:   branch,
			})
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			found := false
			for _, item := range report.Items {
				if item.Name != "upstream" {
					continue
				}
				found = true
				if !strings.Contains(item.Detail, "git push --set-upstream "+fx.Remote+" HEAD:refs/heads/"+branch) {
					t.Fatalf("missing explicit task refspec: %q", item.Detail)
				}
			}
			if !found {
				t.Fatalf("missing upstream failure:\n%s", FormatCheck(report))
			}

			// Execute the prescribed argv under the dangerous push.default values:
			// only the same-named task ref may move.
			runGit(t, fx.Worktree, "push", "--set-upstream", fx.Remote, "HEAD:refs/heads/"+branch)
			runGit(t, fx.Worktree, "fetch", fx.Remote)
			if got := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop"); got != integrationBefore {
				t.Fatalf("integration ref moved: got %s, want %s", got, integrationBefore)
			}
			if got, want := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/"+branch), gitOutput(t, fx.Worktree, "rev-parse", "HEAD"); got != want {
				t.Fatalf("task ref = %s, want HEAD %s", got, want)
			}
		})
	}
}

func TestCheck_IntegrationUpstreamDetectedWhenSHAsMatch(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	branch := "dev/actor/feat/task"
	// Advance the fixture's integration ref to the task tip so a SHA-first
	// implementation would incorrectly call this pushed.
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD:develop")
	runGit(t, fx.Worktree, "branch", "--set-upstream-to", fx.Remote+"/develop", branch)

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   branch,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, item := range report.Items {
		if item.Name == "upstream" && item.Status == checkFail {
			return
		}
	}
	t.Fatalf("same-SHA integration upstream was not detected:\n%s", FormatCheck(report))
}

func TestCheck_IntegrationUpstreamWithSlashRemoteAndBranch(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		remote      string
		integration string
	}{
		{name: "slash remote and branch", remote: "team/upstream", integration: "release/2.0"},
		{name: "remote begins refs remotes", remote: "refs/remotes/team/upstream", integration: "main"},
		{name: "remote begins refs heads", remote: "refs/heads/team/upstream", integration: "main"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fx := testutil.TempWorktreeWithBareOriginRemote(t, tt.remote)
			remote := fx.Remote
			branch := "dev/actor/feat/task"
			if tt.integration != "main" {
				runGit(t, fx.Clone, "branch", tt.integration)
			}
			runGit(t, fx.Clone, "push", remote, tt.integration)
			runGit(t, fx.Worktree, "checkout", "-B", branch, remote+"/"+tt.integration)
			writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: "+tt.integration+"\n")
			runGit(t, fx.Worktree, "add", ".gz-git.yaml")
			runGit(t, fx.Worktree, "commit", "-m", "declare policy")
			runGit(t, fx.Worktree, "push", "-u", remote, "HEAD:refs/heads/"+branch)
			runGit(t, fx.Worktree, "branch", "--set-upstream-to", remote+"/"+tt.integration, branch)

			report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
				RepoPath: fx.Worktree,
				Branch:   branch,
			})
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			for _, item := range report.Items {
				if item.Name == "upstream" && item.Status == checkFail &&
					strings.Contains(item.Detail, "targets integration branch "+tt.integration) {
					return
				}
			}
			t.Fatalf("remote %q integration upstream was not detected:\n%s", remote, FormatCheck(report))
		})
	}
}

// TestCheck_NoContractRunsNoRepositoryCode pins the owner decision that
// gz-git builds, executes, and lints no repository code by itself. The
// fixture carries a Makefile whose targets would fail, and a make on PATH
// that records any invocation. Without a readiness contract the gate mode is
// "none", the contract row passes, and the git checks alone decide READY.
func TestCheck_NoContractRunsNoRepositoryCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake make on PATH is a POSIX shell script")
	}
	fx := readyTaskFixture(t)
	writeRepoFile(t, fx.Worktree, "Makefile", "check:\n\t@false\nlint:\n\t@false\n")
	runGit(t, fx.Worktree, "add", "Makefile")
	runGit(t, fx.Worktree, "commit", "-m", "failing make targets")
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD")
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "make-invoked")
	if err := os.WriteFile(filepath.Join(bin, "make"), []byte("#!/bin/sh\necho \"$@\" >> '"+marker+"'\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   "dev/actor/feat/task",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready {
		t.Fatalf("no-contract repository must be READY on git checks alone:\n%s", FormatCheck(report))
	}
	if report.GateMode != gateModeNone {
		t.Fatalf("GateMode = %q, want %q", report.GateMode, gateModeNone)
	}
	if !hasCheckDetail(report, "readiness contract", checkPass, "gz-git runs no repository code") {
		t.Fatalf("readiness contract row must pass and say no repository code runs:\n%s", FormatCheck(report))
	}
	for _, item := range report.Items {
		if strings.HasPrefix(item.Name, "make") {
			t.Fatalf("no make row may exist, got %+v", item)
		}
	}
	if got := FormatCheck(report); !strings.Contains(got, "readiness provenance — mode=none ") {
		t.Fatalf("provenance must name mode=none:\n%s", got)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		data, _ := os.ReadFile(marker)
		t.Fatalf("make was invoked (%v): %s", statErr, data)
	}
}

// TestCheck_NoContractStillFailsGitChecks shows the git checks keep their
// authority without a gate: an unpushed source is NOT READY.
func TestCheck_NoContractStillFailsGitChecks(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	writeFile(t, fx.Worktree, "unpushed.txt", "x\n")
	runGit(t, fx.Worktree, "add", "unpushed.txt")
	runGit(t, fx.Worktree, "commit", "-m", "unpushed")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   "dev/actor/feat/task",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready || !hasCheckDetail(report, "push", checkFail, "upstream differs") {
		t.Fatalf("unpushed source must fail the push check:\n%s", FormatCheck(report))
	}
}

func TestCheck_StaleTargetFailsFreshness(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	// Advance develop after the task branch was created.
	runGit(t, fx.Clone, "checkout", "develop")
	writeFile(t, fx.Clone, "later.txt", "later\n")
	runGit(t, fx.Clone, "add", "later.txt")
	runGit(t, fx.Clone, "commit", "-m", "develop moves")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   "dev/actor/feat/task",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready {
		t.Fatal("stale target must not be READY")
	}
	found := false
	for _, item := range report.Items {
		if item.Name == "freshness" && item.Status == checkFail {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected freshness FAIL:\n%s", FormatCheck(report))
	}
}

func TestCheck_DirectToDefaultWithoutIntegration(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath:        fx.Worktree,
		Branch:          "feature/worktree",
		Target:          fx.Remote + "/main",
		DirectToDefault: true,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Plan.Target != fx.Remote+"/main" {
		t.Fatalf("target = %q", report.Plan.Target)
	}
}

func hasCheckDetail(report *CheckReport, name, status, detail string) bool {
	for _, item := range report.Items {
		if item.Name == name && item.Status == status && strings.Contains(item.Detail, detail) {
			return true
		}
	}
	return false
}

func readyTaskFixture(t *testing.T) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n")
	runGit(t, fx.Worktree, "add", ".gz-git.yaml")
	runGit(t, fx.Worktree, "commit", "-m", "declare integration branch")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	return fx
}

func writeRepoFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestBuildRunProtocolOmitsFailuresWithoutTimeouts(t *testing.T) {
	t.Parallel()
	protocol := BuildRunProtocol(&RunReport{Check: &CheckReport{
		Items: []CheckItem{{Name: "readiness contract", Status: checkFail, Detail: "ordinary failure"}},
	}}, nil, 1)
	if protocol.ReadinessFailures != nil {
		t.Fatalf("non-timeout failures = %+v, want omitted readiness failures", protocol.ReadinessFailures)
	}
	data, err := json.Marshal(protocol)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if _, found := document["readiness_failures"]; found {
		t.Fatalf("non-timeout protocol includes readiness_failures: %#v", document["readiness_failures"])
	}
}

func TestBuildRunProtocolTimeoutFailuresKeepNullableFieldsAndAllEntries(t *testing.T) {
	t.Parallel()
	signal := "SIGKILL"
	request := &TerminationRequest{Method: "process_group", Signal: &signal}
	protocol := BuildRunProtocol(&RunReport{Check: &CheckReport{ReadinessFailures: []ReadinessFailure{
		{Stage: "contract_runner", Class: "timeout", BudgetMS: 20, ElapsedMS: 21, TerminationRequest: request},
		{Stage: "contract_plan", Class: "timeout", BudgetMS: 30, ElapsedMS: 31},
		{Stage: "contract_runner", Class: "timeout", BudgetMS: 40, ElapsedMS: 41, TerminationRequest: &TerminationRequest{Method: "job_object"}},
	}}}, nil, 1)
	data, err := json.Marshal(protocol)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	failures, ok := document["readiness_failures"].([]any)
	if !ok || len(failures) != 3 {
		t.Fatalf("readiness_failures = %#v, want three entries", document["readiness_failures"])
	}
	for _, raw := range failures {
		failure, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("failure JSON = %#v, want object", raw)
		}
		for _, key := range []string{"leaf_exit_code", "diagnostic_path"} {
			value, found := failure[key]
			if !found || value != nil {
				t.Fatalf("failure JSON = %#v, want explicit %s:null", raw, key)
			}
		}
		if _, found := failure["termination_request"]; !found {
			t.Fatalf("failure JSON = %#v, want termination_request key", raw)
		}
	}
	firstFailure := protocolFailureObject(t, failures[0])
	firstRequest := protocolFailureObject(t, firstFailure["termination_request"])
	if firstRequest["method"] != "process_group" || firstRequest["signal"] != "SIGKILL" {
		t.Fatalf("termination request = %#v", firstRequest)
	}
	second := protocolFailureObject(t, failures[1])
	if request, found := second["termination_request"]; !found || request != nil {
		t.Fatalf("uncanceled timeout request = %#v, want explicit null", second["termination_request"])
	}
	third := protocolFailureObject(t, failures[2])
	jobRequest := protocolFailureObject(t, third["termination_request"])
	if jobRequest["method"] != "job_object" {
		t.Fatalf("Windows termination request = %#v", third["termination_request"])
	}
	if signal, found := jobRequest["signal"]; !found || signal != nil {
		t.Fatalf("Windows signal = %#v, want explicit null", jobRequest["signal"])
	}
}

func protocolFailureObject(t *testing.T, raw any) map[string]any {
	t.Helper()
	object, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("protocol object = %#v, want JSON object", raw)
	}
	return object
}
