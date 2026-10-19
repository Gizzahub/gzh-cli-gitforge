// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// A plan-v1 contract runs the target-owned runner once without the host slot
// to learn the resource class of the checks it is about to select. gz-git
// never interprets repository paths: the runner owns the selection, gz-git
// owns the slot decision and the binding between the plan and the execution.
// readinessPlanTimeout is the production plan budget; like the execution
// budget it is passed down as a parameter, so tests can shorten it.
const (
	readinessPlanTimeout    = 2 * time.Minute
	readinessResourceLight  = "light"
	readinessResourceHigh   = "high"
	readinessPlanDigestSize = 64
)

type readinessPlanResult struct {
	Version    int    `json:"version"`
	Resource   string `json:"resource"`
	PlanDigest string `json:"plan_digest"`
	Summary    string `json:"summary"`
}

// readinessObservation carries the contract-mode phase evidence from the
// runner pair back to the report. Plan time is its own process span and is
// not validation; preparation excludes any slot wait it contained.
type readinessObservation struct {
	PlanDuration  time.Duration
	PlanResource  string
	PlanDigest    string
	ResourceClass string
	PrepareInputs string
}

// observation returns the caller's phase evidence sink, or a discarded one
// for callers that do not report contract-mode phases.
func (o CheckOptions) observation() *readinessObservation {
	if o.readinessObservation == nil {
		return &readinessObservation{}
	}
	return o.readinessObservation
}

// selectReadinessResource decides the execution's resource class. The plan
// runs after preparation, so it observes the same prepared inputs as the
// execution it classifies, and before any slot is held. A static declaration
// can only add the slot; a plan can never remove it.
func selectReadinessResource(ctx context.Context, runner, targetDir, sourceDir string, plan TargetPlan, env []string, contract readinessContract, staticHighLoad bool, planTimeout time.Duration, observed *readinessObservation) (bind []string, out readinessProcessOutput, err error) {
	slotNeeded := staticHighLoad
	if contract.Decl.ResourcePlan == config.ReadinessResourcePlanV1 {
		planned, planOut, planErr := planReadinessResources(ctx, runner, targetDir, sourceDir, plan.BranchSHA, plan.TargetSHA, planTimeout, env, &observed.PlanDuration)
		if planErr != nil {
			return nil, planOut, fmt.Errorf("resource plan: %w", planErr)
		}
		observed.PlanResource, observed.PlanDigest = planned.Resource, planned.PlanDigest
		slotNeeded = slotNeeded || planned.Resource == readinessResourceHigh
		granted := readinessResourceLight
		if slotNeeded {
			granted = readinessResourceHigh
		}
		bind = readinessPlanArgs(granted, planned.PlanDigest)
	}
	observed.ResourceClass = readinessResourceLight
	if slotNeeded {
		observed.ResourceClass = readinessResourceHigh
	}
	return bind, readinessProcessOutput{}, nil
}

// readinessPlanArgs is the execution binding of a plan: the class gz-git
// granted and the digest the runner reported. A runner whose recomputed plan
// differs, or needs more than the granted class, must report unavailable.
func readinessPlanArgs(granted, digest string) []string {
	return []string{"--resource-class", granted, "--plan-digest", digest}
}

func parseReadinessPlan(data []byte) (readinessPlanResult, error) {
	if len(data) == 0 || len(data) > readinessMaxIO || !utf8.Valid(data) || bytes.IndexFunc(data, func(r rune) bool { return r < 0x20 && r != '\n' && r != '\r' && r != '\t' }) >= 0 {
		return readinessPlanResult{}, fmt.Errorf("runner emitted invalid plan JSON")
	}
	raw, err := decodeReadinessResultObject(data)
	if err != nil {
		return readinessPlanResult{}, err
	}
	if len(raw) != 4 || raw["version"] == nil || raw["resource"] == nil || raw["plan_digest"] == nil || raw["summary"] == nil {
		return readinessPlanResult{}, fmt.Errorf("runner plan has unknown or missing fields")
	}
	var plan readinessPlanResult
	if err := json.Unmarshal(data, &plan); err != nil {
		return readinessPlanResult{}, fmt.Errorf("decode runner plan: %w", err)
	}
	if plan.Version != 1 || (plan.Resource != readinessResourceLight && plan.Resource != readinessResourceHigh) || !validPlanDigest(plan.PlanDigest) || !validSummary(plan.Summary) {
		return readinessPlanResult{}, fmt.Errorf("runner plan violates plan-v1")
	}
	return plan, nil
}

// validPlanDigest accepts exactly a lowercase hex SHA-256, so the value can be
// passed back as one argument without quoting or normalization.
func validPlanDigest(digest string) bool {
	if len(digest) != readinessPlanDigestSize {
		return false
	}
	for _, r := range digest {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
