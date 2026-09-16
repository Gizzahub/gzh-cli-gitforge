// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// The readiness refusals are the only place the engine tells an operator how a
// target-owned contract may legitimately evolve; assert the literal guidance so
// the rotation path cannot silently disappear from the messages.

func TestBootstrapPlanOnDeclaredTargetGuidesToReadinessUpdate(t *testing.T) {
	work := readinessUpdateFixture(t, ".gz-git.yaml")
	_, err := BootstrapPlanFor(context.Background(), gitcmd.NewExecutor(), BootstrapOptions{RepoPath: work, Target: "origin/master", Issuer: "t", Expiry: time.Minute})
	if err == nil {
		t.Fatal("bootstrap accepted a target that already declares readiness")
	}
	for _, want := range []string{"target already declares readiness", "gz-git integrate readiness update"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q is missing guidance %q", err.Error(), want)
		}
	}
}

func TestContractChangeGateRefusalGuidesToReadinessUpdate(t *testing.T) {
	work := readinessUpdateFixture(t, ".gz-git.yaml")
	if err := os.WriteFile(filepath.Join(work, ".gz-git", "readiness", "helper"), []byte("helper v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitInTest(t, work, "add", "-A")
	runGitInTest(t, work, "commit", "-m", "rotate readiness tree")
	g := newGitRepo(gitcmd.NewExecutor(), work)
	var report CheckReport
	item := checkReadinessContract(context.Background(), g, TargetPlan{
		TargetSHA: runGitInTest(t, work, "rev-parse", "origin/master"),
		BranchSHA: runGitInTest(t, work, "rev-parse", "HEAD"),
	}, &report)
	if item.Status != checkFail {
		t.Fatalf("status = %q, detail = %q", item.Status, item.Detail)
	}
	for _, want := range []string{"readiness contract changed between target and source", "gz-git integrate readiness update"} {
		if !strings.Contains(item.Detail, want) {
			t.Fatalf("refusal %q is missing guidance %q", item.Detail, want)
		}
	}
}
