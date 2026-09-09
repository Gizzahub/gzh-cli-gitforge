// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

const (
	// SourceNone means no integration branch participates.
	SourceNone = config.IntegrationBranchSourceNone
	// SourceHeuristic means the remote-HEAD fallback won.
	SourceHeuristic = config.IntegrationBranchSourceHeuristic
	// SourceConfigPrefix prefixes a declared candidate source.
	SourceConfigPrefix = config.IntegrationBranchSourceConfigPrefix
)

// Resolution remains an alias for the integrate command's established API.
type Resolution = config.IntegrationBranchResolution

// Facts remains an alias for the integrate command's established API.
type Facts = config.IntegrationBranchFacts

// ResolveFromFacts preserves the integrate package's facts-only API.
func ResolveFromFacts(f Facts) Resolution {
	return config.ResolveIntegrationBranchFromFacts(f)
}

// NormalizeName preserves the integrate package's branch normalization API.
func NormalizeName(raw string, remotes []string) string {
	return config.NormalizeIntegrationBranchName(raw, remotes)
}

// SplitRemoteBranch preserves the integrate package's remote split API.
func SplitRemoteBranch(raw string, remotes []string) (remote, branch string, ok bool) {
	return config.SplitIntegrationRemoteBranch(raw, remotes)
}

// UpstreamTargetsIntegration preserves the integrate package's safety check API.
func UpstreamTargetsIntegration(branch, upstream string, resolution Resolution, remotes []string) bool {
	return config.UpstreamTargetsIntegrationBranch(branch, upstream, resolution, remotes)
}

// ResolveIntegrationBranch preserves the integrate package's Git-backed API.
func ResolveIntegrationBranch(ctx context.Context, exec *gitcmd.Executor, repoPath string, configValues []string) (Resolution, error) {
	return config.ResolveIntegrationBranch(ctx, exec, repoPath, configValues)
}

func isRemoteName(candidate string, remotes []string) bool {
	for _, remote := range remotes {
		if remote == candidate {
			return true
		}
	}
	return false
}
