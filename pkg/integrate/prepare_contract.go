// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// prepareHighLoadForPlan reads both immutable commit declarations. Either side
// may require capacity; a dirty worktree and controller settings have no say.
func prepareHighLoadForPlan(ctx context.Context, g gitRepo, plan TargetPlan) (bool, error) {
	target, err := loadCommitPrepareHighLoad(ctx, g, plan.TargetSHA)
	if err != nil {
		return false, fmt.Errorf("target preparation resource declaration: %w", err)
	}
	source, err := loadCommitPrepareHighLoad(ctx, g, plan.BranchSHA)
	if err != nil {
		return false, fmt.Errorf("source preparation resource declaration: %w", err)
	}
	return target || source, nil
}

func loadCommitPrepareHighLoad(ctx context.Context, g gitRepo, sha string) (bool, error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		entry, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return false, err
		}
		if !present {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return false, fmt.Errorf("%s must be a regular file", name)
		}
		data, _, err := g.showFile(ctx, sha, name)
		if err != nil {
			return false, err
		}
		isJSON := strings.HasSuffix(name, ".json")
		value, err := config.ParsePrepareHighLoadDocument(data, isJSON)
		if err != nil {
			return false, fmt.Errorf("%s: %w", name, err)
		}
		if value && len(data) > readinessMaxManifest {
			return false, fmt.Errorf("%s exceeds preparation manifest size limit", name)
		}
		return value, nil
	}
	return false, nil
}
