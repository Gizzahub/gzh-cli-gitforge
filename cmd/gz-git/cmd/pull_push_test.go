// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestValidatePullPush(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		format   string
		watch    bool
		wantErr  bool
	}{
		{"rebase default format", "rebase", "default", false, false},
		{"merge strategy refused", "merge", "default", false, true},
		{"ff-only refused", "ff-only", "default", false, true},
		{"json refused", "rebase", "json", false, true},
		{"watch refused", "rebase", "default", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePullPush(tt.strategy, tt.format, tt.watch)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validatePullPush(%q, %q, %v) error = %v, wantErr %v", tt.strategy, tt.format, tt.watch, err, tt.wantErr)
			}
		})
	}
}

func TestPullPushEligible(t *testing.T) {
	tests := []struct {
		status string
		dryRun bool
		want   bool
	}{
		{repository.StatusPulled, false, true},
		{repository.StatusUpToDate, false, true},
		{repository.StatusWouldPull, true, true},
		{repository.StatusWouldPull, false, false},
		{repository.StatusConflict, false, false},
		{repository.StatusRebaseInProgress, false, false},
		{repository.StatusMergeInProgress, false, false},
		{repository.StatusAuthRequired, false, false},
		{repository.StatusNoUpstream, false, false},
		{repository.StatusError, false, false},
		{repository.StatusSkipped, false, false},
	}
	for _, tt := range tests {
		if got := pullPushEligible(tt.status, tt.dryRun); got != tt.want {
			t.Errorf("pullPushEligible(%q, dryRun=%v) = %v, want %v", tt.status, tt.dryRun, got, tt.want)
		}
	}
}

func TestPullPushAccessSkipsIneligibleRepositories(t *testing.T) {
	result := &repository.BulkPullResult{
		Repositories: []repository.RepositoryPullResult{
			{Path: "/ws/ok", Status: repository.StatusPulled},
			{Path: "/ws/conflict", Status: repository.StatusConflict},
		},
	}
	access := pullPushAccess(result, false)

	allowed, reason, err := access("/ws/conflict")
	if err != nil || allowed || reason == "" {
		t.Fatalf("conflicted repo: allowed=%v reason=%q err=%v, want refused with a reason", allowed, reason, err)
	}

	allowed, reason, err = access("/ws/unknown")
	if err != nil || allowed {
		t.Fatalf("repo absent from pull result: allowed=%v reason=%q err=%v, want refused", allowed, reason, err)
	}
}
