// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// lazyBaselineFixture is a task branch on a repository that declares the
// familybook-ent-v1 preparation profile. Every preparation and every make
// run appends the tree it ran in ("source" or "target") to a log outside
// the repository, so a test can state exactly which side was prepared and
// which make targets were measured where.
type lazyBaselineFixture struct {
	repo, prepareLog, makeLog string
}

// newLazyBaselineFixture commits config, as .gz-git.yaml, and targetMake on
// develop and branchMake on the task branch. %[1]s in either Makefile is
// replaced by the make log path.
func newLazyBaselineFixture(t *testing.T, config, targetMake, branchMake string) lazyBaselineFixture {
	t.Helper()
	// Make prints physical directories; keep diagnostics attributable.
	physicalTmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", physicalTmp)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	logs := t.TempDir()
	fx := lazyBaselineFixture{prepareLog: filepath.Join(logs, "prepare.log"), makeLog: filepath.Join(logs, "make.log")}
	t.Setenv("PATH", fakeGo(t, fmt.Sprintf("basename \"$PWD\" >> '%s'; mkdir -p ent/generated; : > ent/generated/out", fx.prepareLog))+":"+os.Getenv("PATH"))

	repo := testutil.TempWorktreeWithBareOrigin(t)
	for _, dir := range []string{"ent", "pkg"} {
		if err := os.MkdirAll(filepath.Join(repo.Clone, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeRepoFile(t, repo.Clone, ".gz-git.yaml", config)
	writeRepoFile(t, repo.Clone, ".gitignore", "ent/generated/\n")
	writeRepoFile(t, repo.Clone, "ent/.keep", "")
	writeRepoFile(t, repo.Clone, "pkg/old.go", "package fixture\n")
	writeRepoFile(t, repo.Clone, "Makefile", fmt.Sprintf(targetMake, fx.makeLog))
	runGit(t, repo.Clone, "add", ".")
	runGit(t, repo.Clone, "commit", "-m", "target")
	runGit(t, repo.Clone, "branch", "develop")
	runGit(t, repo.Clone, "push", "-u", repo.Remote, "develop")

	runGit(t, repo.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeRepoFile(t, repo.Worktree, "Makefile", fmt.Sprintf(branchMake, fx.makeLog))
	writeRepoFile(t, repo.Worktree, "task.txt", "task\n")
	runGit(t, repo.Worktree, "add", ".")
	runGit(t, repo.Worktree, "commit", "-m", "task")
	runGit(t, repo.Worktree, "push", "-u", repo.Remote, "HEAD")
	fx.repo = repo.Worktree
	return fx
}

func (fx lazyBaselineFixture) check(t *testing.T) *CheckReport {
	t.Helper()
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.repo, NoFetch: true})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return report
}

// lazyBaselineLog returns the sorted, de-duplicated lines of a log, or none
// when the log was never written.
func lazyBaselineLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			seen[line] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for line := range seen {
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

func lazyBaselineRecord(t *testing.T, report *CheckReport, target string) BaselineRecord {
	t.Helper()
	for _, record := range report.Baselines {
		if record.Target == target {
			return record
		}
	}
	t.Fatalf("no baseline record for make %s:\n%s", target, FormatCheck(report))
	return BaselineRecord{}
}

// Each recipe logs "<tree> <target>" before doing anything else.
const (
	lazyConfig      = "branch:\n  integrationBranch: develop\n  prepareProfile: familybook-ent-v1\n"
	lazyPass        = "check:\n\t@echo \"$(notdir $(CURDIR)) check\" >> '%[1]s'\nlint:\n\t@echo \"$(notdir $(CURDIR)) lint\" >> '%[1]s'\n"
	lazyCheckFails  = "check:\n\t@echo \"$(notdir $(CURDIR)) check\" >> '%[1]s'\n\t@echo 'pkg/old.go:1: broken'; exit 1\nlint:\n\t@echo \"$(notdir $(CURDIR)) lint\" >> '%[1]s'\n"
	lazyPassingLint = lazyPass
)

func TestLazyBaseline_PassingSourceNeverPreparesTarget(t *testing.T) {
	fx := newLazyBaselineFixture(t, lazyConfig, lazyPass, lazyPass)
	report := fx.check(t)
	if !report.Ready {
		t.Fatalf("a passing source must integrate:\n%s", FormatCheck(report))
	}
	if got := lazyBaselineLog(t, fx.prepareLog); strings.Join(got, ",") != "source" {
		t.Fatalf("prepared trees = %v, want only the source", got)
	}
	if got := lazyBaselineLog(t, fx.makeLog); strings.Join(got, ",") != "source check,source lint" {
		t.Fatalf("make runs = %v, want only the source's check and lint", got)
	}
}

func TestLazyBaselineFailing_MeasuresTargetForCheckOnly(t *testing.T) {
	for _, tc := range []struct {
		name, targetMake string
		ready            bool
		want             string
	}{
		// The target tip passes, so the failing branch made it worse.
		{name: "target passes", targetMake: lazyPass, want: "failed here (rc=2) but target tip passes"},
		// The target tip already fails the same way at an unchanged
		// location, so the branch is non-worsening.
		{name: "target fails alike", targetMake: lazyCheckFails, ready: true, want: "make check — baseline failure, non-worsening: count 1 → 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newLazyBaselineFixture(t, lazyConfig, tc.targetMake, lazyCheckFails)
			report := fx.check(t)
			text := FormatCheck(report)
			if report.Ready != tc.ready || !strings.Contains(text, tc.want) {
				t.Fatalf("want ready=%t and %q:\n%s", tc.ready, tc.want, text)
			}
			if got := lazyBaselineLog(t, fx.prepareLog); strings.Join(got, ",") != "source,target" {
				t.Fatalf("prepared trees = %v, want source and target", got)
			}
			if got := lazyBaselineLog(t, fx.makeLog); strings.Join(got, ",") != "source check,source lint,target check" {
				t.Fatalf("make runs = %v, want the target measured for check only", got)
			}
			if record := lazyBaselineRecord(t, report, "check"); !record.Measured || record.Reason != baselineMeasuredSourceFail {
				t.Fatalf("check baseline record = %+v", record)
			}
			if record := lazyBaselineRecord(t, report, "lint"); record.Measured || record.Reason != baselineSkippedSourcePassed {
				t.Fatalf("lint baseline record = %+v", record)
			}
		})
	}
}

func TestLazyBaselineReport_PassingSourceStatesWhyNothingWasMeasured(t *testing.T) {
	fx := newLazyBaselineFixture(t, lazyConfig, lazyPass, lazyPassingLint)
	report := fx.check(t)
	text := FormatCheck(report)
	for _, target := range []string{"check", "lint"} {
		record := lazyBaselineRecord(t, report, target)
		if record.Measured || record.Reason != baselineSkippedSourcePassed {
			t.Fatalf("make %s baseline record = %+v, want not measured because the source passed", target, record)
		}
		want := "INFO  baseline make " + target + " — not measured: source passed"
		if !strings.Contains(text, want) {
			t.Fatalf("report does not state %q:\n%s", want, text)
		}
	}
	// The record is only true if nothing was in fact measured on the target.
	for _, run := range lazyBaselineLog(t, fx.makeLog) {
		if strings.HasPrefix(run, "target ") {
			t.Fatalf("report says not measured, but the target ran %q", run)
		}
	}
}

// lazyOutcomeMake is a passing check recipe that writes a complete outcome
// report with checks, plus a passing lint, both logging like lazyPass.
func lazyOutcomeMake(t *testing.T, checks string) string {
	t.Helper()
	// The recipe is formatted again with the log path, so escape it first.
	body := strings.ReplaceAll(outcomeFixtureMake(t, checks, "ok", 0), "%", "%%")
	body = strings.Replace(body, "check:\n", "check:\n\t@echo \"$(notdir $(CURDIR)) check\" >> '%[1]s'\n", 1)
	return body + "lint:\n\t@echo \"$(notdir $(CURDIR)) lint\" >> '%[1]s'\n"
}

// A declared outcome report compares check IDs even when the source passes:
// a branch that drops an ID the target reports must still be blocked, so the
// baseline of a declaring target is measured although the source passed.
func TestLazyBaseline_DeclaredOutcomeReportMeasuresPassingSource(t *testing.T) {
	config := lazyConfig + "  makeOutcomeReport:\n    version: 1\n    targets: [check]\n"
	for _, tc := range []struct {
		name, targetChecks string
		ready              bool
		want               string
	}{
		// The outcome verdict wording predates this change.
		{name: "same checks", targetChecks: "plain:pass", ready: true, want: "outcome report is non-worsening"},
		{name: "deleted ID", targetChecks: "plain:pass,other:pass", want: "removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newLazyBaselineFixture(t, config, lazyOutcomeMake(t, tc.targetChecks), lazyOutcomeMake(t, "plain:pass"))
			report := fx.check(t)
			text := FormatCheck(report)
			if report.Ready != tc.ready || !strings.Contains(text, tc.want) {
				t.Fatalf("want ready=%t and %q:\n%s", tc.ready, tc.want, text)
			}
			if got := lazyBaselineLog(t, fx.makeLog); strings.Join(got, ",") != "source check,source lint,target check" {
				t.Fatalf("make runs = %v, want the target measured for the declaring check only", got)
			}
			if record := lazyBaselineRecord(t, report, "check"); !record.Measured || record.Reason != baselineMeasuredOutcomes {
				t.Fatalf("check baseline record = %+v", record)
			}
			if record := lazyBaselineRecord(t, report, "lint"); record.Measured || record.Reason != baselineSkippedSourcePassed {
				t.Fatalf("lint baseline record = %+v", record)
			}
		})
	}
}
