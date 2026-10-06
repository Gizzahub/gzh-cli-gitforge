// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

const makeOutcomeReportEnv = "GZ_GIT_MAKE_OUTCOME_REPORT_V1"

func captureMakeOutcomeReport(probe *makeProbe, path string) {
	probe.OutcomeDeclared = true
	// Never turn an unfinished execution or a crashed tool into a report.
	if probe.TimedOut || probe.ToolCrash != "" || probe.MissingCD != "" || probe.Unavailable != "" {
		return
	}
	if !probe.Defined || probe.Skipped {
		probe.Unavailable = "declared make outcome report requires every check to finish"
		return
	}
	if probe.Err != nil {
		var exitErr *exec.ExitError
		if !errors.As(probe.Err, &exitErr) || probe.Code <= 0 {
			probe.Unavailable = fmt.Sprintf("declared make outcome report has no completed exit status: %v", probe.Err)
			return
		}
	}
	report, err := readMakeOutcomeReport(path, probe.Target, probe.Code)
	if err != nil {
		probe.Unavailable = "invalid declared make outcome report: " + err.Error()
		return
	}
	probe.Outcomes = report
}

func evaluateMakeOutcomeProbes(ctx context.Context, g gitRepo, plan TargetPlan, branch, base makeProbe) (BaselineResult, error) {
	if !base.Defined || base.Skipped || base.Unavailable != "" || base.TimedOut || base.ToolCrash != "" || base.MissingCD != "" {
		if base.Err != nil {
			return BaselineResult{}, fmt.Errorf("make outcome baseline is incomplete: %s: %w", base.Unavailable, base.Err)
		}
		return BaselineResult{}, fmt.Errorf("make outcome baseline is incomplete: %s", base.Unavailable)
	}
	if branch.Outcomes == nil || base.Outcomes == nil {
		return BaselineResult{}, fmt.Errorf("target-owned make outcome report missing from a completed probe")
	}
	if branch.Target == "lint" {
		if err := foreignDiagnosticErrorForProbe("baseline", base); err != nil {
			return BaselineResult{}, err
		}
	}
	branchTracked, err := g.lsTreeNames(ctx, plan.BranchSHA)
	if err != nil {
		return BaselineResult{}, err
	}
	baseTracked, err := g.lsTreeNames(ctx, plan.TargetSHA)
	if err != nil {
		return BaselineResult{}, err
	}
	changed, err := g.diffNames(ctx, plan.TargetSHA, plan.BranchSHA)
	if err != nil {
		return BaselineResult{}, err
	}
	var branchLocations, baseLocations []string
	if branch.Err != nil {
		branchLocations = extractLocationsForProbe(branch, branchTracked)
	}
	if base.Err != nil {
		baseLocations = extractLocationsForProbe(base, baseTracked)
	}
	outcomes := compareMakeOutcomes(base.Outcomes, branch.Outcomes)
	return evaluateBaselineWithOutcome(BaselineInput{
		BranchLocations: branchLocations,
		BaseLocations:   baseLocations,
		ChangedPaths:    changed,
		BaseMeasurement: BaseMeasured,
	}, &outcomes), nil
}
