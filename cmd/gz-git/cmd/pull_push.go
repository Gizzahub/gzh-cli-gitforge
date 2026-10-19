// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// validatePullPush rejects --push combinations whose meaning is ambiguous
// before anything touches a repository.
func validatePullPush(strategy, format string, watch bool) error {
	if strategy != "rebase" {
		return fmt.Errorf("--push requires --merge-strategy rebase (got %q)", strategy)
	}
	if format == "json" {
		return fmt.Errorf("--push does not support --format json: pull and push would emit two documents")
	}
	if watch {
		return fmt.Errorf("--push does not support --watch")
	}
	return nil
}

// pullPushEligible reports whether a pull outcome may be followed by a push.
// Only a rebase that finished cleanly qualifies; conflicts, in-progress
// operations, auth failures and skips are never pushed.
func pullPushEligible(status string, dryRun bool) bool {
	switch status {
	case repository.StatusPulled, repository.StatusUpToDate:
		return true
	case repository.StatusWouldPull:
		return dryRun
	default:
		return false
	}
}

// pullPushAccess narrows the workspace push access to repositories whose pull
// was eligible, so the push batch reports every other repository as skipped.
func pullPushAccess(result *repository.BulkPullResult, dryRun bool) func(string) (bool, string, error) {
	eligible := make(map[string]bool, len(result.Repositories))
	for _, repo := range result.Repositories {
		if pullPushEligible(repo.Status, dryRun) {
			eligible[filepath.Clean(repo.Path)] = true
		}
	}

	return func(repoPath string) (bool, string, error) {
		if !eligible[filepath.Clean(repoPath)] {
			return false, "pull --rebase did not succeed; push skipped", nil
		}
		return configuredWorkspacePushAccess(repoPath)
	}
}

// runPullPush pushes the repositories a successful rebase pull left ahead.
// It never forces: after a clean rebase onto upstream a plain push is a
// fast-forward, and a rejection is reported as a failure.
func runPullPush(ctx context.Context, client repository.Client, effective *config.EffectiveConfig, pullOpts repository.BulkPullOptions, pullResult *repository.BulkPullResult) (failed, processed int, err error) {
	guards, err := resolvePushGuards(effective, pushOverrides{})
	if err != nil {
		return 0, 0, err
	}

	setUpstream := effective != nil && effective.Push.SetUpstream

	result, err := client.BulkPush(ctx, repository.BulkPushOptions{
		Directory:         pullOpts.Directory,
		Parallel:          pullOpts.Parallel,
		MaxDepth:          pullOpts.MaxDepth,
		DryRun:            pullOpts.DryRun,
		Verbose:           pullOpts.Verbose,
		SetUpstream:       setUpstream,
		PushAccess:        pullPushAccess(pullResult, pullOpts.DryRun),
		Policy:            guards.policy,
		Identity:          guards.identity,
		IncludeSubmodules: pullOpts.IncludeSubmodules,
		IncludePattern:    pullOpts.IncludePattern,
		ExcludePattern:    pullOpts.ExcludePattern,
		Logger:            pullOpts.Logger,
		ProgressCallback:  createProgressCallback("Pushing", pullFlags.Format, quiet),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("bulk push failed: %w", err)
	}

	if !quiet {
		pushFlags.Format = pullFlags.Format
		displayPushResults(result)
	}

	failed = result.Summary[repository.StatusError] + result.Summary[repository.StatusBlocked]
	return failed, result.TotalProcessed, nil
}
