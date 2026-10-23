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
// It returns the declaration the checks apply: for a release, the one
// committed at the fetched source, which the target carries afterwards.
func validateOwnedCheckPlan(ctx context.Context, g gitRepo, root string, before targetPreflight, plan TargetPlan, controller *controllerBinding, decl config.TaskPatternDecl, repositoryTarget bool) (config.TaskPatternDecl, error) {
	if actual := "refs/heads/" + targetBranchName(plan.Target, plan.Remote); actual != before.targetRef || plan.Branch != before.branch || plan.Remote != before.remote {
		return decl, fmt.Errorf("integration source or target changed between preflight and fetch; re-run check")
	}
	if err := revalidateController(ctx, g, &CheckReport{Plan: plan, Controller: controller}); err != nil {
		return decl, err
	}
	if !repositoryTarget && !plan.Release {
		return decl, nil
	}
	current, err := loadPlanDeclaration(ctx, g, root, plan.Release, plan.BranchSHA)
	if err != nil {
		return decl, err
	}
	if repositoryTarget && (current.Source != decl.Source || !slices.Equal(current.IntegrationBranch, decl.IntegrationBranch)) {
		return decl, fmt.Errorf("integration declaration changed between preflight and fetch; re-run check")
	}
	return current, nil
}
