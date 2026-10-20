// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func siblingContractYAML(remote, revision string) string {
	return fmt.Sprintf("branch:\n  integrationBranch: develop\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n  siblingProvision:\n    version: 1\n    siblings:\n      - name: gitrump-ce\n        remote: %q\n        path: ../gitrump-ce\n        revision: %q\n", remote, revision)
}

func TestResolveSiblingContractCommitOwnership(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	base := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	first := siblingContractYAML("ssh://git@example.test/ce.git", strings.Repeat("a", 40))
	writeFile(t, fx.Worktree, ".gz-git.yaml", first)
	runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "declare siblings")
	declared := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	writeFile(t, fx.Worktree, ".gz-git.yaml", strings.Replace(first, "ce.git", "other.git", 1))
	runGitInTest(t, fx.Worktree, "commit", "-am", "change siblings")
	changed := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	for _, tc := range []struct {
		name, target, source string
		present, fail        bool
	}{
		{"legacy", base, base, false, false},
		{"first declaration", base, declared, true, false},
		{"matching", declared, declared, true, false},
		{"changed", declared, changed, false, true},
		{"removed", declared, base, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, present, err := resolveSiblingContractPair(context.Background(), g, TargetPlan{TargetSHA: tc.target, BranchSHA: tc.source})
			if (err != nil) != tc.fail || (!tc.fail && present != tc.present) {
				t.Fatalf("present=%v err=%v; want present=%v fail=%v", present, err, tc.present, tc.fail)
			}
		})
	}
}

func TestReadinessSiblingSnapshotsAndMissingSourceCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX readiness runner")
	}
	for _, tc := range []struct {
		name, declaredRemote, sourceRemote string
		missing                            bool
		nested                             bool
	}{
		{name: "adjacent", nested: false},
		{name: "declared nested workspace", nested: true},
		{name: "declared nested workspace remote mismatch", nested: true, declaredRemote: "ssh://git@example.test/other.git"},
		{name: "declared nested workspace source remote mismatch", nested: true, sourceRemote: "ssh://git@example.test/other.git"},
		{name: "missing", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dependency, dep := siblingFixture(t)
			fx := readinessFixture(t)
			oid := strings.TrimSpace(runGitInTest(t, dependency.Clone, "rev-parse", "HEAD"))
			runner := "#!/bin/sh\nset -eu\ntest \"$(cat ../gitrump-ce/payload.txt)\" = source\ntest \"$(cat \"$2/../gitrump-ce/payload.txt\")\" = source\ntest \"$PWD/../gitrump-ce\" != \"$2/../gitrump-ce\"\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"snapshots ready\"}'\n"
			declaredRemote := dep.Remote
			if tc.declaredRemote != "" {
				declaredRemote = tc.declaredRemote
			}
			manifest := siblingContractYAML(dep.Remote, oid)
			if tc.nested {
				manifest += "workspaces:\n  " + dep.Name + ":\n    url: " + declaredRemote + "\n"
			}
			advanceReadinessTarget(t, fx, manifest, runner, "declare dependency probe")
			if !tc.missing {
				sourceRoot := filepath.Dir(fx.Clone)
				if tc.nested {
					sourceRoot = fx.Clone
				}
				source := filepath.Join(sourceRoot, dep.Name)
				runGit(t, fx.Clone, "clone", dependency.Origin, source)
				runGit(t, source, "checkout", "-B", "master", "origin/master")
				sourceRemote := dep.Remote
				if tc.sourceRemote != "" {
					sourceRemote = tc.sourceRemote
				}
				runGit(t, source, "remote", "set-url", "origin", sourceRemote)
				runGit(t, source, "remote", "set-url", "--push", "origin", sourceRemote)
			}
			g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
			before := runGitInTest(t, fx.Worktree, "worktree", "list", "--porcelain")
			target := strings.TrimSpace(runGitInTest(t, fx.Clone, "rev-parse", "HEAD"))
			source := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
			contract, _, err := loadReadinessContract(context.Background(), g, target)
			if err != nil {
				t.Fatal(err)
			}
			result, _, err := runContractWithTimeout(context.Background(), g, TargetPlan{TargetSHA: target, BranchSHA: source}, contract, readinessTimeout)
			if tc.missing || tc.declaredRemote != "" || tc.sourceRemote != "" {
				if err == nil || result.Status != "unavailable" {
					t.Fatalf("unavailable sibling source: result=%+v err=%v", result, err)
				}
			} else if err != nil || result.Status != "ready" || !strings.Contains(result.Summary, dep.Name+"@"+oid) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if after := runGitInTest(t, fx.Worktree, "worktree", "list", "--porcelain"); before != after {
				t.Fatalf("readiness worktree leaked: before=%s after=%s", before, after)
			}
			if !tc.missing {
				sourceRoot := filepath.Dir(fx.Clone)
				if tc.nested {
					sourceRoot = fx.Clone
				}
				data, readErr := os.ReadFile(filepath.Join(sourceRoot, dep.Name, "payload.txt"))
				if readErr != nil || string(data) != "source\n" {
					t.Fatalf("primary sibling changed: data=%q err=%v", data, readErr)
				}
			}
		})
	}
}
