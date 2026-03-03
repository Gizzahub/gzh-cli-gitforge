// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func TestRenderTree_Simple(t *testing.T) {
	root := &TreeNode{
		Name: "workstation",
		Children: []*TreeNode{
			{Name: "devbox", Message: "(3 synced)", Children: []*TreeNode{
				{Name: "repo-a", Message: "develop"},
				{Name: "repo-b", Message: "master|↓3"},
				{Name: "repo-c", Message: "master"},
			}},
			{Name: "notes", Message: "(2 synced)", Children: []*TreeNode{
				{Name: "wiki", Message: "master|↓1"},
				{Name: "book", Message: "master"},
			}},
		},
	}

	var buf strings.Builder
	renderTree(&buf, root)
	output := buf.String()

	// Check root
	if !strings.Contains(output, "workstation") {
		t.Error("output should contain root name")
	}

	// Check tree connectors
	if !strings.Contains(output, "├──") {
		t.Error("output should contain ├── connector")
	}
	if !strings.Contains(output, "└──") {
		t.Error("output should contain └── connector")
	}

	// Check children
	if !strings.Contains(output, "devbox") {
		t.Error("output should contain devbox")
	}
	if !strings.Contains(output, "notes") {
		t.Error("output should contain notes")
	}
	if !strings.Contains(output, "repo-a") {
		t.Error("output should contain repo-a")
	}
	if !strings.Contains(output, "master|↓3") {
		t.Error("output should contain compact status 'master|↓3'")
	}
}

func TestRenderTree_Nil(t *testing.T) {
	var buf strings.Builder
	renderTree(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil tree, got %q", buf.String())
	}
}

func TestRenderTree_Flat(t *testing.T) {
	root := &TreeNode{
		Name: "root",
		Children: []*TreeNode{
			{Name: "repo-a", Message: "main"},
			{Name: "repo-b", Message: "develop|↑2"},
		},
	}

	var buf strings.Builder
	renderTree(&buf, root)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 { // root + 2 children
		t.Errorf("expected 3 lines, got %d: %q", len(lines), output)
	}
}

func TestBuildResultTree(t *testing.T) {
	result := reposync.ExecutionResult{
		Succeeded: []reposync.ActionResult{
			{
				Action:     reposync.Action{Repo: reposync.RepoSpec{Name: "r1"}, Workspace: "devbox"},
				PostStatus: &reposync.PostSyncStatus{Branch: "develop", BehindBy: 2},
			},
			{
				Action:     reposync.Action{Repo: reposync.RepoSpec{Name: "r2"}, Workspace: "devbox"},
				PostStatus: &reposync.PostSyncStatus{Branch: "master"},
			},
			{
				Action:     reposync.Action{Repo: reposync.RepoSpec{Name: "r3"}, Workspace: "notes"},
				PostStatus: &reposync.PostSyncStatus{Branch: "main", IsDirty: true},
			},
		},
		Failed: []reposync.ActionResult{
			{
				Action: reposync.Action{Repo: reposync.RepoSpec{Name: "r4"}, Workspace: "notes"},
				Error:  nil, // technically failed but no error set in this test
			},
		},
	}

	tree := buildResultTree(result, "test-root")

	if tree.Name != "test-root" {
		t.Errorf("root name = %q, want %q", tree.Name, "test-root")
	}

	// Should have 2 workspace children
	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(tree.Children))
	}

	devbox := tree.Children[0]
	if devbox.Name != "devbox" {
		t.Errorf("first child name = %q, want %q", devbox.Name, "devbox")
	}
	if len(devbox.Children) != 2 {
		t.Errorf("devbox children = %d, want 2", len(devbox.Children))
	}

	notes := tree.Children[1]
	if notes.Name != "notes" {
		t.Errorf("second child name = %q, want %q", notes.Name, "notes")
	}
	if len(notes.Children) != 2 {
		t.Errorf("notes children = %d, want 2", len(notes.Children))
	}
}

func TestBuildResultTree_FlatRepos(t *testing.T) {
	result := reposync.ExecutionResult{
		Succeeded: []reposync.ActionResult{
			{
				Action:     reposync.Action{Repo: reposync.RepoSpec{Name: "flat-1"}},
				PostStatus: &reposync.PostSyncStatus{Branch: "main"},
			},
			{
				Action:     reposync.Action{Repo: reposync.RepoSpec{Name: "flat-2"}},
				PostStatus: &reposync.PostSyncStatus{Branch: "develop"},
			},
		},
	}

	tree := buildResultTree(result, "root")

	// Flat repos should be direct children (no workspace parent node)
	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 direct children, got %d", len(tree.Children))
	}
	if tree.Children[0].Name != "flat-1" {
		t.Errorf("child 0 name = %q, want %q", tree.Children[0].Name, "flat-1")
	}
}

func TestBuildResultTree_Empty(t *testing.T) {
	result := reposync.ExecutionResult{}
	tree := buildResultTree(result, "empty")

	if len(tree.Children) != 0 {
		t.Errorf("expected 0 children for empty result, got %d", len(tree.Children))
	}
}

func TestCountSucceeded(t *testing.T) {
	results := []reposync.ActionResult{
		{Error: nil},
		{Error: nil},
		{Error: nil},
	}
	if got := countSucceeded(results); got != 3 {
		t.Errorf("countSucceeded() = %d, want 3", got)
	}
}
