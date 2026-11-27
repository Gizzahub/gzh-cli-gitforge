// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// branch.integrationBranch decides which branch takes task branches directly
// and which one only a release may change. An integration that is not a
// release lands a task branch whose working tree may itself change that
// value, so the branch being judged must not supply it:
//
//   - A branch that declares the default branch would land there without a
//     release, although the default branch still names another integration
//     branch.
//   - A branch that first declares a separate integration branch could never
//     reach the default branch: this check would call it a release, and the
//     release would refuse it because the default branch does not declare
//     that branch yet.
//
// The gate is therefore the integrationBranch committed on the default
// branch, which is also what a release reads, since a release target is the
// default branch (release_declaration.go). The target commit is not the
// authority when it is the integration branch: an integration branch that
// still carries an older value would refuse the merge that updates it.
//
// The working tree keeps supplying everything else, and the default target
// when none is given. Its integrationBranch still gates when the default
// branch offers none: it is unknown, has no root declaration, declares no
// integration branch, or carries a declaration that cannot be used. The first
// three are a repository adopting the declaration. The last must not refuse,
// because an integration is then the only way to repair the declaration; the
// check reports it as a warning instead.

// applyDefaultBranchDeclaration replaces the plan's integration branch with
// the one committed on the default branch, when that commit declares one.
func applyDefaultBranchDeclaration(ctx context.Context, g gitRepo, exec *gitcmd.Executor, plan *TargetPlan) error {
	if plan.DefaultRef == "" {
		return nil
	}
	// DefaultRef is a remote-tracking short name (planFetchDefault). The full
	// ref keeps a local branch spelled "origin/main" from shadowing it and
	// supplying the gate.
	commit, ok, err := g.revParse(ctx, "refs/remotes/"+plan.DefaultRef)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	integration, source, err := committedIntegrationBranch(ctx, g, commit)
	var unusable declarationContentError
	if errors.As(err, &unusable) {
		plan.gateDeclarationProblem = unusable.Error()
		return nil
	}
	if err != nil {
		return fmt.Errorf("default branch declaration: %w", err)
	}
	if len(integration) == 0 {
		return nil
	}
	resolved, err := ResolveIntegrationBranch(ctx, exec, g.dir, integration)
	if err != nil {
		return err
	}
	plan.Integration = resolved
	plan.gateIntegration, plan.gateDeclarationSource, plan.gateCommit = integration, source, commit
	return nil
}

// gateDeclarationLabel names the committed declaration that gates the plan,
// or "" when the working tree's does.
func gateDeclarationLabel(plan TargetPlan) string {
	if plan.gateDeclarationSource == "" {
		return ""
	}
	return fmt.Sprintf("integrationBranch %s from %s at %s commit %s",
		formatDeclaredBranches(plan.gateIntegration), plan.gateDeclarationSource, plan.DefaultRef, config.ShortCommit(plan.gateCommit))
}

// gateError names the declaration behind a target refusal, so a refusal that
// follows the default branch's value is not read against the working tree's.
func gateError(plan TargetPlan, err error) error {
	label := gateDeclarationLabel(plan)
	if label == "" {
		return err
	}
	return fmt.Errorf("%w (%s gates this integration)", err, label)
}

// addGateDeclarationWarning reports a default-branch declaration that could
// not gate the integration.
func addGateDeclarationWarning(plan TargetPlan, add func(CheckItem)) {
	if plan.gateDeclarationProblem == "" {
		return
	}
	add(CheckItem{
		Name:   "default branch declaration",
		Status: checkWarn,
		Detail: fmt.Sprintf("%s; the working-tree integrationBranch gates instead", plan.gateDeclarationProblem),
	})
}
