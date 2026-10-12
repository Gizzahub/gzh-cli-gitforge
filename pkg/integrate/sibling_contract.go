// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// Resolve only committed declarations. A first closed declaration can prepare
// both probes; changing or removing an established declaration fails closed.
func resolveSiblingContractPair(ctx context.Context, g gitRepo, plan TargetPlan) (targetContract, sourceContract config.SiblingProvision, declared bool, resultErr error) {
	target, targetPresent, err := loadCommitSiblingContract(ctx, g, plan.TargetSHA)
	if err != nil {
		return target, target, false, fmt.Errorf("target sibling contract: %w", err)
	}
	source, sourcePresent, err := loadCommitSiblingContract(ctx, g, plan.BranchSHA)
	if err != nil {
		return target, source, false, fmt.Errorf("source sibling contract: %w", err)
	}
	if targetPresent && (!sourcePresent || !sameSiblingPolicy(target, source)) {
		return target, source, false, fmt.Errorf("source changed or removed the target-owned sibling contract")
	}
	if !targetPresent {
		target = source
	}
	return target, source, sourcePresent, nil
}

// An exact commit is dependency data, so a reviewed pin upgrade prepares each
// side at its own pin. Identity, placement and revision mode remain fixed.
func sameSiblingPolicy(a, b config.SiblingProvision) bool {
	if a.Version != b.Version || len(a.Siblings) != len(b.Siblings) {
		return false
	}
	for i, before := range a.Siblings {
		after := b.Siblings[i]
		if before.Name != after.Name || before.Remote != after.Remote || before.Path != after.Path || before.SourceBranch != after.SourceBranch {
			return false
		}
		if (before.Revision == "source-tip") != (after.Revision == "source-tip") {
			return false
		}
	}
	return true
}

func loadCommitSiblingContract(ctx context.Context, g gitRepo, sha string) (config.SiblingProvision, bool, error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		entry, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return config.SiblingProvision{}, false, err
		}
		if !present {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return config.SiblingProvision{}, false, fmt.Errorf("%s must be a regular file", name)
		}
		data, _, err := g.showFile(ctx, sha, name)
		if err != nil {
			return config.SiblingProvision{}, false, err
		}
		declaration, declared, err := config.ParseSiblingProvisionDocument(data, strings.HasSuffix(name, ".json"))
		if err != nil {
			return declaration, declared, err
		}
		if declared && len(data) > readinessMaxManifest {
			return declaration, declared, fmt.Errorf("sibling manifest exceeds size limit")
		}
		return declaration, declared, nil
	}
	return config.SiblingProvision{}, false, nil
}

func releaseReadinessSiblings(leases []SiblingLease) error {
	var err error
	for i := len(leases) - 1; i >= 0; i-- {
		if releaseErr := ReleaseSibling(leases[i]); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("release readiness sibling: %w", releaseErr))
		}
	}
	return err
}

func prepareReadinessSiblings(ctx context.Context, g gitRepo, targetDeclaration, sourceDeclaration config.SiblingProvision, targetDir, sourceDir string) ([]SiblingLease, string, error) {
	var leases []SiblingLease
	var evidence []string
	for i, dependency := range targetDeclaration.Siblings {
		primary, err := DiscoverSiblingPrimary(ctx, g.dir, dependency)
		if err != nil {
			return nil, "", errors.Join(err, releaseReadinessSiblings(leases))
		}
		target, err := ProvisionSibling(ctx, targetDir, primary, dependency)
		if err != nil {
			return nil, "", errors.Join(err, releaseReadinessSiblings(leases))
		}
		leases = append(leases, target)
		sourceDependency := sourceDeclaration.Siblings[i]
		source, err := ProvisionSibling(ctx, sourceDir, primary, sourceDependency)
		if err != nil {
			return nil, "", errors.Join(err, releaseReadinessSiblings(leases))
		}
		leases = append(leases, source)
		if sourceDependency.Revision == dependency.Revision && (source.OID != target.OID || source.Manifest != target.Manifest) {
			return nil, "", errors.Join(fmt.Errorf("sibling snapshot drift between readiness probes"), releaseReadinessSiblings(leases))
		}
		evidence = append(evidence, "target:"+dependency.Name+"@"+target.OID, "source:"+dependency.Name+"@"+source.OID)
	}
	return leases, strings.Join(evidence, " "), nil
}
