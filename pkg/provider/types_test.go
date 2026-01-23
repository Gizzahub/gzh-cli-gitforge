// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package provider

import (
	"testing"
	"time"
)

func TestSyncAction(t *testing.T) {
	tests := []struct {
		action   SyncAction
		expected string
	}{
		{ActionCloned, "cloned"},
		{ActionUpdated, "updated"},
		{ActionSkipped, "skipped"},
		{ActionFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if string(tt.action) != tt.expected {
				t.Errorf("SyncAction = %q, want %q", tt.action, tt.expected)
			}
		})
	}
}

func TestRepository(t *testing.T) {
	now := time.Now()
	repo := &Repository{
		Name:          "test-repo",
		FullName:      "org/test-repo",
		CloneURL:      "https://github.com/org/test-repo.git",
		SSHURL:        "git@github.com:org/test-repo.git",
		HTMLURL:       "https://github.com/org/test-repo",
		Description:   "A test repository",
		DefaultBranch: "main",
		Private:       false,
		Archived:      false,
		Fork:          false,
		Disabled:      false,
		Language:      "Go",
		Size:          1024,
		Topics:        []string{"cli", "git"},
		Visibility:    "public",
		CreatedAt:     now,
		UpdatedAt:     now,
		PushedAt:      now,
	}

	if repo.Name != "test-repo" {
		t.Errorf("Name = %q, want %q", repo.Name, "test-repo")
	}
	if repo.FullName != "org/test-repo" {
		t.Errorf("FullName = %q, want %q", repo.FullName, "org/test-repo")
	}
	if len(repo.Topics) != 2 {
		t.Errorf("Topics length = %d, want 2", len(repo.Topics))
	}
}

func TestOrganization(t *testing.T) {
	org := &Organization{
		Name:        "test-org",
		Description: "A test organization",
		URL:         "https://github.com/test-org",
	}

	if org.Name != "test-org" {
		t.Errorf("Name = %q, want %q", org.Name, "test-org")
	}
}

func TestRateLimit(t *testing.T) {
	reset := time.Now().Add(time.Hour)
	rl := &RateLimit{
		Limit:     5000,
		Remaining: 4500,
		Reset:     reset,
		Used:      500,
	}

	if rl.Limit != 5000 {
		t.Errorf("Limit = %d, want 5000", rl.Limit)
	}
	if rl.Remaining != 4500 {
		t.Errorf("Remaining = %d, want 4500", rl.Remaining)
	}
	if rl.Used != 500 {
		t.Errorf("Used = %d, want 500", rl.Used)
	}
}

func TestSyncOptions(t *testing.T) {
	opts := SyncOptions{
		TargetPath:      "/tmp/repos",
		Parallel:        10,
		IncludeArchived: false,
		IncludeForks:    true,
		IncludePrivate:  true,
		DryRun:          true,
	}

	if opts.TargetPath != "/tmp/repos" {
		t.Errorf("TargetPath = %q, want %q", opts.TargetPath, "/tmp/repos")
	}
	if opts.Parallel != 10 {
		t.Errorf("Parallel = %d, want 10", opts.Parallel)
	}
	if opts.IncludeArchived {
		t.Error("IncludeArchived should be false")
	}
	if !opts.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestListOptions(t *testing.T) {
	opts := ListOptions{
		Page:    1,
		PerPage: 100,
	}

	if opts.Page != 1 {
		t.Errorf("Page = %d, want 1", opts.Page)
	}
	if opts.PerPage != 100 {
		t.Errorf("PerPage = %d, want 100", opts.PerPage)
	}
}
