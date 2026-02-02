// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func TestBuildSyncSummary(t *testing.T) {
	tests := []struct {
		name       string
		actions    []reposync.Action
		wantClone  int
		wantUpdate int
		wantSkip   int
		wantDelete int
		wantTotal  int
	}{
		{
			name:      "empty actions",
			actions:   []reposync.Action{},
			wantTotal: 0,
		},
		{
			name: "mixed actions",
			actions: []reposync.Action{
				{Type: reposync.ActionClone, Repo: reposync.RepoSpec{Name: "repo1"}},
				{Type: reposync.ActionClone, Repo: reposync.RepoSpec{Name: "repo2"}},
				{Type: reposync.ActionUpdate, Repo: reposync.RepoSpec{Name: "repo3"}},
				{Type: reposync.ActionSkip, Repo: reposync.RepoSpec{Name: "repo4"}},
			},
			wantClone:  2,
			wantUpdate: 1,
			wantSkip:   1,
			wantTotal:  4,
		},
		{
			name: "all updates",
			actions: []reposync.Action{
				{Type: reposync.ActionUpdate, Repo: reposync.RepoSpec{Name: "repo1"}},
				{Type: reposync.ActionUpdate, Repo: reposync.RepoSpec{Name: "repo2"}},
				{Type: reposync.ActionUpdate, Repo: reposync.RepoSpec{Name: "repo3"}},
			},
			wantUpdate: 3,
			wantTotal:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := buildSyncSummary(tt.actions)

			if summary.Clone != tt.wantClone {
				t.Errorf("Clone = %d, want %d", summary.Clone, tt.wantClone)
			}
			if summary.Update != tt.wantUpdate {
				t.Errorf("Update = %d, want %d", summary.Update, tt.wantUpdate)
			}
			if summary.Skip != tt.wantSkip {
				t.Errorf("Skip = %d, want %d", summary.Skip, tt.wantSkip)
			}
			if summary.Delete != tt.wantDelete {
				t.Errorf("Delete = %d, want %d", summary.Delete, tt.wantDelete)
			}
			if summary.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", summary.Total, tt.wantTotal)
			}
		})
	}
}

func TestDisplaySyncSummary(t *testing.T) {
	tests := []struct {
		name         string
		summary      syncSummary
		wantContains []string
	}{
		{
			name: "mixed summary",
			summary: syncSummary{
				Clone:  2,
				Update: 3,
				Skip:   1,
				Total:  6,
			},
			wantContains: []string{
				"Sync Preview",
				"Total: 6 repositories",
				"2 will be cloned",
				"3 will be updated",
				"1 will be skipped",
			},
		},
		{
			name: "clone only",
			summary: syncSummary{
				Clone: 5,
				Total: 5,
			},
			wantContains: []string{
				"5 will be cloned",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			displaySyncSummary(&buf, tt.summary)
			output := buf.String()

			for _, want := range tt.wantContains {
				if !bytes.Contains([]byte(output), []byte(want)) {
					t.Errorf("output missing %q, got:\n%s", want, output)
				}
			}
		})
	}
}
