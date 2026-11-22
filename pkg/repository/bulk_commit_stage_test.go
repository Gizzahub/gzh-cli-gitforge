// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCommitStage(t *testing.T) {
	tests := []struct {
		in      string
		want    CommitStage
		wantErr bool
	}{
		{"", StageAll, false},
		{"all", StageAll, false},
		{"tracked", StageTracked, false},
		{"staged", StageStaged, false},
		{"index", "", true},
	}
	for _, tt := range tests {
		got, err := ParseCommitStage(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseCommitStage(%q) = %q, %v; want %q, err=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestCommitStageArgs(t *testing.T) {
	tests := []struct {
		name   string
		stage  CommitStage
		nested []string
		want   []string
	}{
		{"all", StageAll, nil, []string{"add", "-A"}},
		{"zero value is all", "", nil, []string{"add", "-A"}},
		{
			"all excludes nested", StageAll,
			[]string{"inner", "a b/deep"},
			[]string{"add", "-A", "--", ".", ":(exclude,literal)inner/", ":(exclude,literal)a b/deep/"},
		},
		{"tracked", StageTracked, []string{"inner"}, []string{"add", "-u"}},
		{"staged stages nothing", StageStaged, []string{"inner"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commitStageArgs(tt.stage, tt.nested); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("commitStageArgs = %q, want %q", got, tt.want)
			}
		})
	}
}

// An embedded repository shows up as `?? dir/` even under -uall; it belongs in
// NestedRepos, never in Entries, in every scope.
func TestChangeSetRecordsNestedRepositories(t *testing.T) {
	repoPath := newRepoIn(t, t.TempDir(), "outer")
	writeIn(t, repoPath, "new.txt", "n\n")
	inner := filepath.Join(repoPath, "group", "inner")
	gitIn(t, repoPath, "init", "--quiet", inner)
	writeIn(t, inner, "x.txt", "x\n")

	c := testClient(t)
	for _, scope := range []ChangeScope{ScopeHead, ScopeTracked, ScopeStagedOnly, ScopeWorktreeOnly} {
		cs, err := c.collectChangeSet(context.Background(), repoPath, scope)
		if err != nil {
			t.Fatalf("collectChangeSet(%s): %v", scope, err)
		}
		if want := []string{"group/inner"}; !reflect.DeepEqual(cs.NestedRepos, want) {
			t.Errorf("%s: NestedRepos = %q, want %q", scope, cs.NestedRepos, want)
		}
		for _, entry := range cs.Entries {
			if entry.Path == "group/inner/" {
				t.Errorf("%s: nested repository listed as an entry", scope)
			}
		}
	}
}

// A committed embedded repository is exactly what `git add -A` turns into a
// gitlink. The commit must record the outer change and leave inner out.
func TestBulkCommitLeavesCommittedNestedRepositoryOut(t *testing.T) {
	rootDir := t.TempDir()
	outer := newRepoIn(t, rootDir, "outer")
	writeIn(t, outer, "new.txt", "n\n")
	newRepoIn(t, outer, filepath.Join("group", "inner"))

	result := bulkCommitAll(t, BulkCommitOptions{Directory: rootDir, Message: "feat: outer"})
	if result.TotalCommitted != 1 || result.TotalFailed != 0 {
		t.Fatalf("committed=%d failed=%d, want 1/0: %+v", result.TotalCommitted, result.TotalFailed, result.Repositories)
	}
	if got := result.Repositories[0].NestedRepos; !reflect.DeepEqual(got, []string{"group/inner"}) {
		t.Errorf("NestedRepos = %q, want group/inner", got)
	}
	tree := gitIn(t, outer, "ls-tree", "-r", "--name-only", "HEAD")
	if want := "README.md\nnew.txt\n"; tree != want {
		t.Errorf("outer HEAD tree = %q, want %q", tree, want)
	}
}

// --stage tracked must not predict a commit for a repository whose only change
// is an untracked file.
func TestCommitPreviewTrackedIgnoresUntracked(t *testing.T) {
	rootDir := t.TempDir()
	repoPath := newRepoIn(t, rootDir, "repo")
	writeIn(t, repoPath, "untracked.txt", "u\n")

	result := bulkCommitAll(t, BulkCommitOptions{Directory: rootDir, DryRun: true, Stage: StageTracked})
	if len(result.Repositories) != 1 || result.Repositories[0].Status != "clean" {
		t.Fatalf("tracked preview = %+v, want one clean repository", result.Repositories)
	}
}
