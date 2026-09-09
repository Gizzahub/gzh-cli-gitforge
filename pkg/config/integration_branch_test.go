// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestResolveIntegrationBranchFromFacts_PreservesPrecedenceAndMissingSemantics(t *testing.T) {
	tests := []struct {
		name  string
		facts IntegrationBranchFacts
		want  IntegrationBranchResolution
	}{
		{
			name:  "declared branch wins over remote HEAD",
			facts: IntegrationBranchFacts{Config: []string{"develop"}, Refs: []string{"refs/remotes/origin/master", "refs/remotes/origin/develop"}, Remotes: []string{"origin"}, DefaultName: "master"},
			want:  IntegrationBranchResolution{Participates: true, Name: "develop", Source: "config[0]"},
		},
		{
			name:  "declared missing branch does not fall back",
			facts: IntegrationBranchFacts{Config: []string{"developp"}, Refs: []string{"refs/remotes/origin/develop"}, Remotes: []string{"origin"}, DefaultName: "develop"},
			want:  IntegrationBranchResolution{Source: IntegrationBranchSourceNone},
		},
		{
			name:  "missing remote HEAD does not select local branch",
			facts: IntegrationBranchFacts{Refs: []string{"refs/heads/develop"}},
			want:  IntegrationBranchResolution{Source: IntegrationBranchSourceNone},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveIntegrationBranchFromFacts(tt.facts); got != tt.want {
				t.Fatalf("ResolveIntegrationBranchFromFacts() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeIntegrationBranchName_UsesLongestRemotePrefix(t *testing.T) {
	remotes := []string{"team", "team/upstream"}
	if got := NormalizeIntegrationBranchName("refs/remotes/team/upstream/release/2.0", remotes); got != "release/2.0" {
		t.Fatalf("NormalizeIntegrationBranchName() = %q, want release/2.0", got)
	}
	remote, branch, ok := SplitIntegrationRemoteBranch("team/upstream/release/2.0", remotes)
	if !ok || remote != "team/upstream" || branch != "release/2.0" {
		t.Fatalf("SplitIntegrationRemoteBranch() = %q, %q, %v", remote, branch, ok)
	}
}

func TestResolveIntegrationBranch_ReadsRemoteHEAD(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runIntegrationGit(t, fx.Clone, "branch", "develop")
	runIntegrationGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")

	got, err := ResolveIntegrationBranch(context.Background(), gitcmd.NewExecutor(), fx.Clone, nil)
	if err != nil {
		t.Fatalf("ResolveIntegrationBranch: %v", err)
	}
	want := IntegrationBranchResolution{Participates: true, Name: "main", Source: IntegrationBranchSourceHeuristic}
	if got != want {
		t.Fatalf("ResolveIntegrationBranch() = %+v, want %+v", got, want)
	}
}

func runIntegrationGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := gitcmd.NewExecutor().RunOutput(context.Background(), dir, args...); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
