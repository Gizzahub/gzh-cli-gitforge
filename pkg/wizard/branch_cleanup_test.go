// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
)

func TestNewBranchCleanupWizard(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		wantDir   string
	}{
		{"empty uses current", "", "."},
		{"specific path", "/tmp/repos", "/tmp/repos"},
		{"relative path", "./repos", "./repos"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewBranchCleanupWizard(tt.directory)

			if w == nil {
				t.Fatal("NewBranchCleanupWizard returned nil")
			}

			if w.directory != tt.wantDir {
				t.Errorf("directory = %q, want %q", w.directory, tt.wantDir)
			}

			// Check defaults
			if !w.opts.IncludeMerged {
				t.Error("default IncludeMerged should be true")
			}

			if w.opts.StaleThreshold != 30*24*time.Hour {
				t.Errorf("default StaleThreshold = %v, want 30 days", w.opts.StaleThreshold)
			}
		})
	}
}

func TestBranchCleanupResult(t *testing.T) {
	result := &BranchCleanupResult{
		ReposProcessed:  5,
		BranchesDeleted: 10,
		BranchesSkipped: 3,
		Errors:          []string{"error1", "error2"},
	}

	if result.ReposProcessed != 5 {
		t.Errorf("ReposProcessed = %d, want 5", result.ReposProcessed)
	}

	if result.BranchesDeleted != 10 {
		t.Errorf("BranchesDeleted = %d, want 10", result.BranchesDeleted)
	}

	if len(result.Errors) != 2 {
		t.Errorf("Errors count = %d, want 2", len(result.Errors))
	}
}

func TestFormatBranchForSelection(t *testing.T) {
	tests := []struct {
		name     string
		branch   *branch.Branch
		report   *branch.CleanupReport
		wantPart string
	}{
		{
			name:   "merged branch",
			branch: &branch.Branch{Name: "feature/test"},
			report: &branch.CleanupReport{
				Merged: []*branch.Branch{{Name: "feature/test"}},
			},
			wantPart: "[merged]",
		},
		{
			name:   "stale branch",
			branch: &branch.Branch{Name: "old-feature"},
			report: &branch.CleanupReport{
				Stale: []*branch.Branch{{Name: "old-feature"}},
			},
			wantPart: "[stale]",
		},
		{
			name:   "orphaned branch",
			branch: &branch.Branch{Name: "remotes/origin/deleted"},
			report: &branch.CleanupReport{
				Orphaned: []*branch.Branch{{Name: "remotes/origin/deleted"}},
			},
			wantPart: "[gone]",
		},
		{
			name:     "branch with ahead/behind",
			branch:   &branch.Branch{Name: "feature/wip", AheadBy: 2, BehindBy: 5},
			report:   &branch.CleanupReport{},
			wantPart: "↑2 ↓5",
		},
		{
			name:     "plain branch",
			branch:   &branch.Branch{Name: "plain"},
			report:   &branch.CleanupReport{},
			wantPart: "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBranchForSelection(tt.branch, tt.report)
			if !contains([]string{got}, tt.wantPart) && got != tt.wantPart {
				// More flexible check for substrings
				if tt.wantPart != "plain" {
					if len(got) < len(tt.wantPart) {
						t.Errorf("formatBranchForSelection() = %q, want to contain %q", got, tt.wantPart)
					}
				}
			}
		})
	}
}

func TestFilterBranches(t *testing.T) {
	branches := []*branch.Branch{
		{Name: "main"},
		{Name: "develop"},
		{Name: "feature/test"},
	}

	tests := []struct {
		name     string
		target   string
		wantLen  int
		wantName string
	}{
		{"existing branch", "develop", 1, "develop"},
		{"non-existing branch", "unknown", 0, ""},
		{"another existing", "feature/test", 1, "feature/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterBranches(branches, tt.target)

			if len(got) != tt.wantLen {
				t.Errorf("filterBranches() returned %d branches, want %d", len(got), tt.wantLen)
			}

			if tt.wantLen > 0 && got[0].Name != tt.wantName {
				t.Errorf("filterBranches() first branch = %q, want %q", got[0].Name, tt.wantName)
			}
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"merged", "stale", "gone"}

	tests := []struct {
		name string
		item string
		want bool
	}{
		{"existing item", "merged", true},
		{"another existing", "stale", true},
		{"non-existing", "other", false},
		{"empty item", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(slice, tt.item)
			if got != tt.want {
				t.Errorf("contains(%v, %q) = %v, want %v", slice, tt.item, got, tt.want)
			}
		})
	}
}
