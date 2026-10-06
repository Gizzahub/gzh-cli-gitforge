// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

type makeOutcomePolicy struct {
	source *config.MakeOutcomeReport
	target *config.MakeOutcomeReport
}

// A source-only declaration is a bootstrap, not permission to judge that
// source using its new report. Every already-adopted target remains adopted.
func resolveMakeOutcomePolicy(ctx context.Context, g gitRepo, plan TargetPlan) (makeOutcomePolicy, error) {
	target, err := loadCommitMakeOutcomeReport(ctx, g, plan.TargetSHA)
	if err != nil {
		return makeOutcomePolicy{}, fmt.Errorf("target make outcome declaration: %w", err)
	}
	source, err := loadCommitMakeOutcomeReport(ctx, g, plan.BranchSHA)
	if err != nil {
		return makeOutcomePolicy{}, fmt.Errorf("source make outcome declaration: %w", err)
	}
	if target != nil {
		for _, name := range target.Targets {
			if !source.Includes(name) {
				return makeOutcomePolicy{}, fmt.Errorf("source removed target-owned make outcome report for %s", name)
			}
		}
	}
	return makeOutcomePolicy{source: source, target: target}, nil
}

func loadCommitMakeOutcomeReport(ctx context.Context, g gitRepo, sha string) (*config.MakeOutcomeReport, error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		entry, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return nil, fmt.Errorf("%s must be a regular file", name)
		}
		data, _, err := g.showFile(ctx, sha, name)
		if err != nil {
			return nil, err
		}
		decl, err := config.ParseMakeOutcomeReportDocument(data, strings.HasSuffix(name, ".json"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if decl != nil && len(data) > readinessMaxManifest {
			return nil, fmt.Errorf("%s exceeds make outcome manifest size limit", name)
		}
		return decl, nil
	}
	return nil, nil //nolint:nilnil // nil means the optional root declaration is absent
}
