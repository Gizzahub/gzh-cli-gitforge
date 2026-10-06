// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestMakeOutcomeEnvironmentCase(t *testing.T) {
	env := []string{"OTHER=1", makeOutcomeReportEnv + "=stale", strings.ToLower(makeOutcomeReportEnv) + "=also-stale", makeOutcomeReportEnv + "_OTHER=keep"}
	for _, insensitive := range []bool{false, true} {
		got := withoutEnvCase(env, makeOutcomeReportEnv, insensitive)
		want := []string{"OTHER=1"}
		if !insensitive {
			want = append(want, strings.ToLower(makeOutcomeReportEnv)+"=also-stale")
		}
		want = append(want, makeOutcomeReportEnv+"_OTHER=keep")
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("insensitive=%v: got %v, want %v", insensitive, got, want)
		}
	}
}

func TestCheck_MakeOutcomeReports(t *testing.T) {
	cases := []struct {
		name, baseChecks, branchChecks, baseOutput, branchOutput, want string
		baseExit, branchExit                                           int
		ready, removeDeclaration, firstAdoption, changeDiagnosticPath  bool
	}{
		{name: "mixed baseline improves", baseChecks: "format:fail,plain:fail", branchChecks: "format:pass,plain:fail", baseOutput: "pkg/old.go:1: broken", branchOutput: "plain failure", baseExit: 1, branchExit: 1, ready: true, want: "improved"},
		{name: "unchanged plain failure", baseChecks: "plain:fail", branchChecks: "plain:fail", baseExit: 1, branchExit: 1, ready: true, want: "non-worsening"},
		{name: "new plain failure", baseChecks: "plain:fail", branchChecks: "plain:fail,new:fail", baseExit: 1, branchExit: 1, want: "new failed check"},
		{name: "new passing check", baseChecks: "plain:fail", branchChecks: "plain:fail,new:pass", baseExit: 1, branchExit: 1, ready: true},
		{name: "passing check regresses", baseChecks: "plain:fail,other:pass", branchChecks: "plain:pass,other:fail", baseExit: 1, branchExit: 1, want: "regressed"},
		{name: "deleted ID even when branch passes", baseChecks: "plain:fail,other:fail", branchChecks: "plain:pass", baseExit: 1, branchExit: 0, want: "removed"},
		{name: "new location count still blocks", baseChecks: "plain:fail", branchChecks: "plain:fail", branchOutput: "pkg/old.go:1: broken", baseExit: 1, branchExit: 1, want: "count increased"},
		{name: "changed path still blocks", baseChecks: "plain:fail", branchChecks: "plain:fail", baseOutput: "pkg/old.go:1: broken", branchOutput: "pkg/changed.go:1: broken", baseExit: 1, branchExit: 1, changeDiagnosticPath: true, want: "changed paths"},
		{name: "report cannot conceal nonzero exit", baseChecks: "plain:pass", branchChecks: "plain:pass", branchExit: 1, want: "conflicts with make exit"},
		{name: "first adoption keeps old comparison", baseChecks: "plain:fail", branchChecks: "plain:fail", baseOutput: "pkg/old.go:1: broken", baseExit: 1, branchExit: 1, firstAdoption: true, want: "no file:line"},
		{name: "clean first adoption", baseChecks: "plain:pass", branchChecks: "plain:pass", firstAdoption: true, ready: true},
		{name: "declaration removal blocks", baseChecks: "plain:pass", branchChecks: "plain:pass", removeDeclaration: true, want: "removed target-owned"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Make prints physical directories. Keep this attribution fixture
			// physical too, independently of macOS's /var -> /private/var alias.
			physicalTmp, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("TMPDIR", physicalTmp)
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			fx := testutil.TempWorktreeWithBareOrigin(t)
			if err := os.MkdirAll(filepath.Join(fx.Clone, "pkg"), 0o755); err != nil {
				t.Fatal(err)
			}
			baseConfig := outcomeFixtureConfig(!tc.firstAdoption)
			writeRepoFile(t, fx.Clone, ".gz-git.yaml", baseConfig)
			writeRepoFile(t, fx.Clone, "Makefile", outcomeFixtureMake(t, tc.baseChecks, tc.baseOutput, tc.baseExit))
			writeRepoFile(t, fx.Clone, "pkg/old.go", "package fixture\n")
			writeRepoFile(t, fx.Clone, "pkg/changed.go", "package fixture\n")
			runGit(t, fx.Clone, "add", ".")
			runGit(t, fx.Clone, "commit", "-m", "baseline outcomes")
			runGit(t, fx.Clone, "branch", "develop")
			runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
			runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
			writeRepoFile(t, fx.Worktree, ".gz-git.yaml", outcomeFixtureConfig(!tc.removeDeclaration))
			writeRepoFile(t, fx.Worktree, "Makefile", outcomeFixtureMake(t, tc.branchChecks, tc.branchOutput, tc.branchExit))
			if tc.changeDiagnosticPath {
				writeRepoFile(t, fx.Worktree, "pkg/changed.go", "package changed\n")
			}
			runGit(t, fx.Worktree, "add", ".")
			// Unchanged scenarios still need a task commit for the integration fixture.
			runGit(t, fx.Worktree, "commit", "--allow-empty", "-m", "source outcomes")
			runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
			report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, NoFetch: true})
			if err != nil {
				t.Fatal(err)
			}
			if report.Ready != tc.ready || (tc.want != "" && !strings.Contains(FormatCheck(report), tc.want)) {
				t.Fatalf("want ready=%t and %q\n%s", tc.ready, tc.want, FormatCheck(report))
			}
		})
	}
}

func TestMakeOutcomeExecutionCannotRescueIncompleteRuns(t *testing.T) {
	for _, recipe := range []string{"@true", "@echo SKIPPED CHECK: incomplete", "@sleep 5", "@echo 'panic: checker died'; exit 1"} {
		t.Run(recipe, func(t *testing.T) {
			dir := t.TempDir()
			body := outcomeFixtureMake(t, "plain:pass", "", 0)
			if recipe == "@true" {
				body = "check:\n\t@true\n" // declared report is absent
			} else {
				body += "\t" + recipe + "\n"
			}
			writeRepoFile(t, dir, "Makefile", body)
			probe := runMakeTargetWithOutcomes(context.Background(), dir, "check", 200*time.Millisecond, true)
			for _, allow := range []bool{false, true} {
				item := judgeMakeLegacy(context.Background(), gitRepo{}, TargetPlan{}, probe, allow, 0)
				if item.Status != checkFail {
					t.Fatalf("incomplete run accepted (allowSkipped=%t): %+v", allow, item)
				}
			}
		})
	}
}

func TestMakeOutcomeEnvironmentAndCleanup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv(makeOutcomeReportEnv, filepath.Join(tmp, "ambient.json"))
	dir := t.TempDir()
	writeRepoFile(t, dir, "Makefile", "check:\n\t@test -z \"$$"+makeOutcomeReportEnv+"\"\n")
	if probe := runMakeTarget(context.Background(), dir, "check", time.Second); probe.Err != nil {
		t.Fatalf("ambient report variable leaked into legacy make: %v", probe.Err)
	}
	writeRepoFile(t, dir, "Makefile", outcomeFixtureMake(t, "plain:pass", "", 0))
	if probe := runMakeTargetWithOutcomes(context.Background(), dir, "check", time.Second, true); probe.Err != nil || probe.Outcomes == nil {
		t.Fatalf("complete report rejected: %+v", probe)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "gz-git-make-outcome-") || entry.Name() == "ambient.json" {
			t.Fatalf("unexpected report artifact: %s", entry.Name())
		}
	}
}

func outcomeFixtureConfig(enabled bool) string {
	text := "branch:\n  integrationBranch: develop\n"
	if enabled {
		text += "  makeOutcomeReport:\n    version: 1\n    targets: [check]\n"
	}
	return text
}

func outcomeFixtureMake(t *testing.T, checks, output string, exit int) string {
	t.Helper()
	report := makeOutcomeReport{Version: 1, Target: "check", Complete: true}
	for _, pair := range strings.Split(checks, ",") {
		id, outcome, _ := strings.Cut(pair, ":")
		report.Checks = append(report.Checks, makeCheckOutcome{ID: id, Outcome: outcome})
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	// Only fixed test data enters this Make recipe; none is user input.
	return fmt.Sprintf("check:\n\t@if test -n \"$$%s\"; then printf '%%s' '%s' > \"$$%s\"; fi\n\t@printf '%%s\\n' '%s'\n\t@exit %d\n", makeOutcomeReportEnv, data, makeOutcomeReportEnv, output, exit)
}
