// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// resourceHighLoadForPlan reads both committed endpoints. A working-tree
// config is not authority for a gate that is about to promote committed Git
// objects. Either endpoint opting in is conservative and prevents a source
// change from bypassing target policy while the check is in flight.
func resourceHighLoadForPlan(ctx context.Context, g gitRepo, plan TargetPlan) (bool, error) {
	var highLoad bool
	for _, sha := range []string{plan.TargetSHA, plan.BranchSHA} {
		high, err := resourceHighLoadAt(ctx, g, sha)
		if err != nil {
			return false, err
		}
		highLoad = highLoad || high
	}
	return highLoad, nil
}

func resourceHighLoadAt(ctx context.Context, g gitRepo, sha string) (bool, error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		_, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return false, err
		}
		if !present {
			continue
		}
		data, present, err := g.showFile(ctx, sha, name)
		if err != nil {
			return false, err
		}
		if !present {
			continue
		}
		high, err := config.ParseResourceHighLoadDocument(data, strings.HasSuffix(name, ".json"))
		if err != nil {
			return false, fmt.Errorf("%s at %s: branch.resourceHighLoad: %w", name, sha, err)
		}
		return high, nil
	}
	return false, nil
}
