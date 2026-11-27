// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// A hotfix is the one landing that changes the default branch without a
// release: a task branch cut from the default branch lands on it while the
// repository keeps a separate integration branch. It is opt-in (--hotfix) and
// every condition below must hold; any violation pushes nothing.
//
//   - It runs only with an execution context and a pinned target, and never
//     combines with --release, --direct-to-default, --controller-config or
//     --no-fetch.
//   - The gate is the declaration committed on the default branch, read at the
//     very commit the task lands on. The working tree never gates a hotfix.
//   - The source is a task branch that carries no unreleased history of a
//     declared integration branch and does not change that declaration.
//
// Bringing the hotfix back into the integration branch is the caller's job.

// ErrHotfixRefused reports a --hotfix landing that one of its conditions
// refused. Nothing is pushed.
var ErrHotfixRefused = errors.New("hotfix landing refused")

func hotfixRefusal(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrHotfixRefused, fmt.Sprintf(format, args...))
}

// validateHotfixOptions refuses option sets a hotfix cannot be judged under.
// It reads nothing, so it holds before any repository access or fetch.
func validateHotfixOptions(opts CheckOptions) error {
	if !opts.Hotfix {
		return nil
	}
	for _, c := range []struct {
		refused bool
		reason  string
	}{
		{opts.Release, "--hotfix cannot be combined with --release"},
		{opts.DirectToDefault, "--hotfix cannot be combined with --direct-to-default"},
		{strings.TrimSpace(opts.ControllerConfig) != "", "--hotfix cannot be combined with --controller-config"},
		{opts.NoFetch, "--hotfix cannot be combined with --no-fetch"},
		{strings.TrimSpace(opts.Target) == "" || opts.ExpectTarget == "", "--hotfix requires a pinned --target <default branch>@<sha>"},
		{opts.ExpectSource == "", "--hotfix requires --source <sha>"},
	} {
		if c.refused {
			return hotfixRefusal("%s", c.reason)
		}
	}
	return nil
}

// requireHotfixRun holds a hotfix run to the durable execution context before
// the run reads or locks anything.
func requireHotfixRun(opts RunOptions) error {
	if !opts.Hotfix {
		return nil
	}
	if opts.Execution == nil {
		return hotfixRefusal("--hotfix requires --source, --record and --owner")
	}
	return validateHotfixOptions(opts.CheckOptions)
}

// asHotfixRefusal classifies an error that stopped a hotfix from being judged
// as a refusal: a condition that cannot be established does not hold. An error
// under a canceled or expired context keeps its own class.
func asHotfixRefusal(ctx context.Context, hotfix bool, err error) error {
	if err == nil || !hotfix || ctx.Err() != nil || errors.Is(err, ErrHotfixRefused) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrHotfixRefused, err)
}

// requireLandingRules holds the pinned plan to the rules of its landing kind.
func requireLandingRules(ctx context.Context, g gitRepo, plan TargetPlan) error {
	if plan.Hotfix {
		return asHotfixRefusal(ctx, true, requireHotfixLanding(ctx, g, plan))
	}
	return requireRemoteReleaseRef(ctx, g, plan)
}

// requireHotfixLanding judges a hotfix after the fetch pinned both commits.
func requireHotfixLanding(ctx context.Context, g gitRepo, plan TargetPlan) error {
	defaultName, err := remoteDefaultBranch(ctx, g, plan.Remote)
	if err != nil {
		return err
	}
	if err := requireHotfixGate(plan, defaultName); err != nil {
		return err
	}
	candidates, err := hotfixIntegrationNames(ctx, g, plan)
	if err != nil {
		return err
	}
	if err := requireHotfixSource(ctx, g, plan, defaultName, candidates); err != nil {
		return err
	}
	if err := requireHotfixHistory(ctx, g, plan, candidates); err != nil {
		return err
	}
	return requireHotfixDeclaration(ctx, g, plan)
}

// hotfixIntegrationNames lists the branches the gating declaration offers as
// integration candidates, the resolved one first. Every candidate counts, not
// only the one that resolved. Names are normalized against all remotes, as the
// resolver does.
func hotfixIntegrationNames(ctx context.Context, g gitRepo, plan TargetPlan) ([]string, error) {
	remotes, err := g.remotes(ctx)
	if err != nil {
		return nil, err
	}
	names := []string{plan.Integration.Name}
	for _, raw := range plan.gateIntegration {
		name := config.NormalizeIntegrationBranchName(raw, remotes)
		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names, nil
}

// remoteDefaultBranch names the branch <remote>/HEAD points at. It reads the
// full ref name: the short form changes spelling when a local branch is named
// like the remote-tracking ref.
func remoteDefaultBranch(ctx context.Context, g gitRepo, remote string) (string, error) {
	head := "refs/remotes/" + remote + "/HEAD"
	full, err := g.output(ctx, "symbolic-ref", "--quiet", head)
	if err != nil {
		return "", hotfixRefusal("the default branch is unknown: %s: %v", head, err)
	}
	name, ok := strings.CutPrefix(full, "refs/remotes/"+remote+"/")
	if !ok || name == "" {
		return "", hotfixRefusal("the default branch is unknown: %s names %q", head, full)
	}
	return name, nil
}

// requireHotfixGate requires a committed default-branch declaration that names
// a separate integration branch, read at the commit the task lands on.
func requireHotfixGate(plan TargetPlan, defaultName string) error {
	if plan.gateCommit == "" {
		detail := ""
		if plan.gateDeclarationProblem != "" {
			detail = ": " + plan.gateDeclarationProblem
		}
		return hotfixRefusal("no usable integrationBranch declaration is committed on the default branch %s, and no other declaration gates a hotfix%s", defaultName, detail)
	}
	if !plan.Integration.Participates {
		return hotfixRefusal("declared integration branch %s does not exist", formatDeclaredBranches(plan.gateIntegration))
	}
	if plan.Integration.Name == defaultName {
		return hotfixRefusal("the declared integration branch is the default branch %s; a hotfix needs a separate one", defaultName)
	}
	if got := targetBranchName(plan.Target, plan.Remote); got != defaultName {
		return hotfixRefusal("--hotfix lands only on the default branch %s, not %s", defaultName, plan.Target)
	}
	if plan.gateCommit != plan.TargetSHA {
		return hotfixRefusal("target %s is %s, but the gating declaration was read at default branch commit %s", plan.Target, plan.TargetSHA, plan.gateCommit)
	}
	return nil
}

// requireHotfixSource requires a local task branch as the source.
func requireHotfixSource(ctx context.Context, g gitRepo, plan TargetPlan, defaultName string, candidates []string) error {
	full, err := g.output(ctx, "rev-parse", "--symbolic-full-name", plan.Branch)
	if err != nil {
		return fmt.Errorf("hotfix source ref: %w", err)
	}
	name, local := strings.CutPrefix(full, "refs/heads/")
	if !local {
		return hotfixRefusal("source %s is not a local task branch", plan.Branch)
	}
	if name == defaultName || slices.Contains(candidates, name) {
		return hotfixRefusal("source %s is not a task branch; the default and integration branches cannot be a hotfix source", name)
	}
	return nil
}

// remoteIntegrationRefs reads the candidates' remote-tracking refs and returns
// each existing one with the revision that excludes it. The resolved branch,
// candidates[0], must exist; a later candidate of a list is optional.
func remoteIntegrationRefs(ctx context.Context, g gitRepo, remote string, candidates []string) (refs, excluded []string, err error) {
	for i, name := range candidates {
		// show-ref matches the exact name only. rev-parse would fall back to
		// a local branch spelled like the missing remote-tracking ref.
		ref := "refs/remotes/" + remote + "/" + name
		res, err := g.run(ctx, "show-ref", "--verify", "--hash", ref)
		if err != nil {
			return nil, nil, err
		}
		if res.ExitCode != 0 {
			if i == 0 {
				return nil, nil, hotfixRefusal("remote integration ref %s is missing", ref)
			}
			continue
		}
		refs = append(refs, ref)
		excluded = append(excluded, "^"+strings.TrimSpace(res.Stdout))
	}
	return refs, excluded, nil
}

// requireHotfixHistory refuses a source carrying any commit between the target
// and the source that a remote integration branch also reaches: landing it
// would release unreleased integration work. A cherry-pick is a new commit and
// passes.
func requireHotfixHistory(ctx context.Context, g gitRepo, plan TargetPlan, candidates []string) error {
	refs, excluded, err := remoteIntegrationRefs(ctx, g, plan.Remote, candidates)
	if err != nil {
		return err
	}
	beyond := []string{"rev-list", "--count", plan.BranchSHA, "^" + plan.TargetSHA}
	all, err := g.output(ctx, beyond...)
	if err != nil {
		return err
	}
	own, err := g.output(ctx, append(beyond, excluded...)...)
	if err != nil {
		return err
	}
	if all != own {
		return hotfixRefusal("source %s carries history from %s (%s commits beyond the target, only %s of them its own); cut the hotfix from the default branch and cherry-pick", plan.Branch, strings.Join(refs, " or "), all, own)
	}
	return nil
}

// requireHotfixDeclaration refuses a source whose committed integrationBranch
// differs from the one that gates it.
func requireHotfixDeclaration(ctx context.Context, g gitRepo, plan TargetPlan) error {
	declared, _, err := committedIntegrationBranch(ctx, g, plan.BranchSHA)
	var unusable declarationContentError
	if errors.As(err, &unusable) {
		return hotfixRefusal("source %s commits an unusable declaration: %s", plan.Branch, unusable.Error())
	}
	if err != nil {
		return fmt.Errorf("hotfix source declaration: %w", err)
	}
	if !slices.Equal(declared, plan.gateIntegration) {
		return hotfixRefusal("source %s changes the declared integration branch from %s to %s; a hotfix cannot change it",
			plan.Branch, formatDeclaredBranches(plan.gateIntegration), formatDeclaredBranches(declared))
	}
	return nil
}
