// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// resolvePrepareProfile compares the two commit-owned declarations before
// selecting a fixed built-in executor. A first declaration in source may
// bootstrap itself because it cannot supply executable code to this gate.
func resolvePrepareProfile(ctx context.Context, g gitRepo, plan TargetPlan, controller *controllerBinding) (string, error) {
	target, targetPresent, err := loadCommitPrepareProfile(ctx, g, plan.TargetSHA)
	if err != nil {
		return "", fmt.Errorf("target preparation profile: %w", err)
	}
	source, sourcePresent, err := loadCommitPrepareProfile(ctx, g, plan.BranchSHA)
	if err != nil {
		return "", fmt.Errorf("source preparation profile: %w", err)
	}
	if targetPresent && !sourcePresent {
		return "", fmt.Errorf("source removed target-owned branch.prepareProfile %q", target)
	}
	if targetPresent && source != target {
		return "", fmt.Errorf("branch.prepareProfile differs between target %q and source %q", target, source)
	}
	if controller != nil && controller.PrepareProfile != "" {
		if sourcePresent && controller.PrepareProfile != source {
			return "", fmt.Errorf("controller preparation profile %q conflicts with repository profile %q", controller.PrepareProfile, source)
		}
		return controller.PrepareProfile, nil
	}
	if sourcePresent {
		return source, nil
	}
	return "", nil
}

func loadCommitPrepareProfile(ctx context.Context, g gitRepo, sha string) (profile string, declared bool, err error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		entry, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return "", false, err
		}
		if !present {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return "", false, fmt.Errorf("%s must be a regular file", name)
		}
		data, _, err := g.showFile(ctx, sha, name)
		if err != nil {
			return "", false, err
		}
		profile, declared, err := config.ParsePrepareProfileDocument(data, strings.HasSuffix(name, ".json"))
		if err != nil {
			return "", false, fmt.Errorf("%s: %w", name, err)
		}
		// A repository without this declaration retains its preexisting Make
		// behavior even if its root config is large. A declared profile has a
		// bounded manifest, like the readiness contract.
		if declared && len(data) > readinessMaxManifest {
			return "", false, fmt.Errorf("%s exceeds preparation manifest size limit", name)
		}
		return profile, declared, nil
	}
	return "", false, nil
}
