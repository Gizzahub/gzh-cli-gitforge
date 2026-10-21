// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"slices"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// validateOwnedCheckPlan binds the post-fetch plan to the declaration read by
// the preflight. The fetch between them may let another writer change either
// the source or its controller; no git check may consume that stale policy.
// resolveTarget runs first so explicit source/target approval errors keep their
// precedence. Caller-supplied IntegrationConfig is not a repository declaration.
func validateOwnedCheckPlan(ctx context.Context, g gitRepo, root string, before targetPreflight, plan TargetPlan, controller *controllerBinding, decl config.TaskPatternDecl, repositoryTarget bool) error {
	if actual := "refs/heads/" + targetBranchName(plan.Target, plan.Remote); actual != before.targetRef || plan.Branch != before.branch || plan.Remote != before.remote {
		return fmt.Errorf("integration source or target changed between preflight and fetch; re-run check")
	}
	if err := revalidateController(ctx, g, &CheckReport{Plan: plan, Controller: controller}); err != nil {
		return err
	}
	if !repositoryTarget {
		return nil
	}
	current, err := config.LoadRepoRootTaskPattern(root)
	if err != nil {
		return err
	}
	if current.Source != decl.Source || !slices.Equal(current.IntegrationBranch, decl.IntegrationBranch) {
		return fmt.Errorf("integration declaration changed between preflight and fetch; re-run check")
	}
	return nil
}
