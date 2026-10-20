// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// readinessRunnerEnv returns the runner environment, prepared by the
// contract's profile when it declares one, and records the input evidence.
func readinessRunnerEnv(ctx context.Context, g gitRepo, plan TargetPlan, profile, root, targetDir, sourceDir string, leases []SiblingLease, opts CheckOptions, observed *readinessObservation) ([]string, error) {
	if profile == "" {
		return readinessEnv(os.Environ()), nil
	}
	env, inputs, err := prepareReadinessPair(ctx, g, plan, profile, root, targetDir, sourceDir, leases, opts)
	if err != nil {
		return nil, err
	}
	observed.PrepareInputs = inputs
	return env, nil
}

// prepareReadinessPair runs the contract-declared preparation profile and
// returns the target-owned runner's environment with the input evidence. Each
// profile keeps its own side policy; the runner itself never changes.
func prepareReadinessPair(ctx context.Context, g gitRepo, plan TargetPlan, profile, root, targetDir, sourceDir string, leases []SiblingLease, opts CheckOptions) (env []string, evidence string, err error) {
	switch profile {
	case config.NDStackCheckedTestsV1:
		cargoTarget, err := prepareNDReadinessPair(ctx, g, plan, profile, root, targetDir, sourceDir, leases, opts)
		if err != nil {
			return nil, "", err
		}
		return ndPreparationEnv(os.Environ(), cargoTarget), profile, nil
	case config.PrepareProfileInfraOpsPythonUVV1:
		return prepareInfraOpsReadinessSource(ctx, g, plan, sourceDir, opts)
	}
	return nil, "", fmt.Errorf("unsupported readiness preparation profile: %s", profile)
}

// prepareInfraOpsReadinessSource prepares only the source snapshot. The V1
// runner validates --source-dir and receives one environment, so a prepared
// target would be measured by nothing. The owned root lands beside the
// source worktree, inside the readiness workspace that runContractPair removes.
func prepareInfraOpsReadinessSource(ctx context.Context, g gitRepo, plan TargetPlan, sourceDir string, opts CheckOptions) (env []string, evidence string, err error) {
	profile := config.PrepareProfileInfraOpsPythonUVV1
	inputs, err := snapshotInfraOpsPythonUVInputs(ctx, g, plan.BranchSHA)
	if err != nil {
		return nil, "", fmt.Errorf("snapshot readiness preparation inputs: %w", err)
	}
	lockInputs, err := snapshotPrepareLockInputs(ctx, g, profile, plan.BranchSHA, "source")
	if err != nil {
		return nil, "", fmt.Errorf("snapshot readiness preparation lockfiles: %w", err)
	}
	highLoad, err := prepareHighLoadForPlan(ctx, g, plan)
	if err != nil {
		return nil, "", err
	}
	guard, err := newPreparedRefGuard(ctx, g, plan, plan.BranchSHA)
	if err != nil {
		return nil, "", err
	}
	if err := runPrepareProfileWithResource(ctx, g, sourceDir, profile, inputs, guard, highLoad, opts); err != nil {
		return nil, "", fmt.Errorf("readiness preparation %s at %s: %w", profile, plan.BranchSHA, err)
	}
	env = readinessEnv(os.Environ())
	overlay := preparationEnv(withInfraOpsPreparationEnv(ctx, sourceDir, profile))
	// The runner sees the same uv state the preparation used: caller values
	// could otherwise point uv at a different project or environment.
	for _, key := range []string{"UV_PROJECT_ENVIRONMENT", "VIRTUAL_ENV", "UV_NO_SYNC", "UV_CACHE_DIR", "UV_PROJECT"} {
		env = withoutEnv(env, key)
	}
	parts := []string{profile}
	for _, input := range inputs {
		parts = append(parts, "source:"+input.Name+"@"+input.OID)
	}
	for _, input := range lockInputs {
		parts = append(parts, input.Name+"@"+input.OID)
	}
	return append(env, overlay...), strings.Join(parts, " "), nil
}
