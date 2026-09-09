// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

const (
	// IntegrationBranchSourceNone means no integration branch participates.
	IntegrationBranchSourceNone = "none"
	// IntegrationBranchSourceHeuristic means the remote-HEAD fallback won.
	IntegrationBranchSourceHeuristic = "heuristic"
	// IntegrationBranchSourceConfigPrefix prefixes a declared candidate source.
	IntegrationBranchSourceConfigPrefix = "config["
)

// IntegrationBranchResolution is the integration-branch answer for one repository.
type IntegrationBranchResolution struct {
	Participates bool
	Name         string
	Source       string
}

// Resolution is the integration-branch answer for one repository.
type Resolution = IntegrationBranchResolution

// IntegrationBranchFacts are the inputs needed to resolve an integration branch.
type IntegrationBranchFacts struct {
	Config      []string
	Refs        []string
	Remotes     []string
	DefaultName string
}

// Facts are the inputs needed to resolve an integration branch.
type Facts = IntegrationBranchFacts

// ResolveFromFacts is the concise form of ResolveIntegrationBranchFromFacts.
func ResolveFromFacts(f Facts) Resolution {
	return ResolveIntegrationBranchFromFacts(f)
}

// NormalizeName is the concise form of NormalizeIntegrationBranchName.
func NormalizeName(raw string, remotes []string) string {
	return NormalizeIntegrationBranchName(raw, remotes)
}

// SplitRemoteBranch is the concise form of SplitIntegrationRemoteBranch.
func SplitRemoteBranch(raw string, remotes []string) (remote, branch string, ok bool) {
	return SplitIntegrationRemoteBranch(raw, remotes)
}

// UpstreamTargetsIntegration is the concise form of
// UpstreamTargetsIntegrationBranch.
func UpstreamTargetsIntegration(branch, upstream string, resolution Resolution, remotes []string) bool {
	return UpstreamTargetsIntegrationBranch(branch, upstream, resolution, remotes)
}

// ResolveIntegrationBranchFromFacts interprets integration-branch participation
// from already gathered facts. A declared name that does not exist does not fall
// back to the default branch.
func ResolveIntegrationBranchFromFacts(f IntegrationBranchFacts) IntegrationBranchResolution {
	refs := make(map[string]struct{}, len(f.Refs))
	for _, ref := range f.Refs {
		if ref = strings.TrimSpace(ref); ref != "" {
			refs[ref] = struct{}{}
		}
	}
	declared := false
	for i, raw := range f.Config {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		declared = true
		name := NormalizeIntegrationBranchName(raw, f.Remotes)
		if name != "" && integrationBranchRefExists(name, f.Remotes, refs) {
			return IntegrationBranchResolution{Participates: true, Name: name, Source: fmt.Sprintf("%s%d]", IntegrationBranchSourceConfigPrefix, i)}
		}
	}
	if declared {
		return IntegrationBranchResolution{Source: IntegrationBranchSourceNone}
	}
	defaultName := strings.TrimSpace(f.DefaultName)
	if defaultName != "" && integrationBranchRefExists(defaultName, f.Remotes, refs) {
		return IntegrationBranchResolution{Participates: true, Name: defaultName, Source: IntegrationBranchSourceHeuristic}
	}
	return IntegrationBranchResolution{Source: IntegrationBranchSourceNone}
}

// NormalizeIntegrationBranchName strips only a registered remote prefix.
func NormalizeIntegrationBranchName(raw string, remotes []string) string {
	name := strings.TrimSpace(raw)
	if _, branch, ok := SplitIntegrationRemoteBranch(name, remotes); ok {
		return branch
	}
	remoteRef := strings.HasPrefix(name, "refs/remotes/")
	switch {
	case strings.HasPrefix(name, "refs/heads/"):
		return strings.TrimPrefix(name, "refs/heads/")
	case remoteRef:
		name = strings.TrimPrefix(name, "refs/remotes/")
	}
	if remoteRef {
		if i := strings.IndexByte(name, '/'); i >= 0 {
			return name[i+1:]
		}
	}
	return name
}

// SplitIntegrationRemoteBranch separates a tracking ref using the longest
// registered remote prefix, preserving slash-containing remote and branch names.
func SplitIntegrationRemoteBranch(raw string, remotes []string) (remote, branch string, ok bool) {
	name := strings.TrimSpace(raw)
	if remote, branch, ok = splitIntegrationRegisteredRemote(name, remotes); ok {
		return remote, branch, true
	}
	if stripped, found := strings.CutPrefix(name, "refs/remotes/"); found {
		return splitIntegrationRegisteredRemote(stripped, remotes)
	}
	return "", "", false
}

// UpstreamTargetsIntegrationBranch reports a non-integration branch tracking
// the integration branch itself.
func UpstreamTargetsIntegrationBranch(branch, upstream string, resolution IntegrationBranchResolution, remotes []string) bool {
	if !resolution.Participates || strings.TrimSpace(branch) == "" || strings.TrimSpace(upstream) == "" {
		return false
	}
	if NormalizeIntegrationBranchName(branch, remotes) == resolution.Name {
		return false
	}
	return NormalizeIntegrationBranchName(upstream, remotes) == resolution.Name
}

// ResolveIntegrationBranch reads Git facts through exec. Missing refs and
// remote HEAD are reportable non-participation, not errors.
func ResolveIntegrationBranch(ctx context.Context, exec *gitcmd.Executor, repoPath string, configValues []string) (IntegrationBranchResolution, error) {
	if exec == nil {
		return IntegrationBranchResolution{Source: IntegrationBranchSourceNone}, fmt.Errorf("git executor is nil")
	}
	remotes, err := integrationGitLines(ctx, exec, repoPath, "remote")
	if err != nil {
		return IntegrationBranchResolution{Source: IntegrationBranchSourceNone}, err
	}
	refs, err := integrationGitLines(ctx, exec, repoPath, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return IntegrationBranchResolution{Source: IntegrationBranchSourceNone}, err
	}
	defaultName := ""
	if remote := preferredIntegrationRemote(remotes); remote != "" {
		res, err := exec.Run(ctx, repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/"+remote+"/HEAD")
		if err != nil {
			return IntegrationBranchResolution{Source: IntegrationBranchSourceNone}, fmt.Errorf("git symbolic-ref: %w", err)
		}
		if res.ExitCode == 0 {
			defaultName = NormalizeIntegrationBranchName(res.Stdout, remotes)
		}
	}
	return ResolveIntegrationBranchFromFacts(IntegrationBranchFacts{Config: configValues, Refs: refs, Remotes: remotes, DefaultName: defaultName}), nil
}

func integrationGitLines(ctx context.Context, exec *gitcmd.Executor, repoPath string, args ...string) ([]string, error) {
	res, err := exec.Run(ctx, repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(res.Stderr))
	}
	return splitIntegrationLines(res.Stdout), nil
}

func integrationBranchRefExists(name string, remotes []string, refs map[string]struct{}) bool {
	if _, ok := refs["refs/heads/"+name]; ok {
		return true
	}
	for _, remote := range remotes {
		if remote != "" {
			if _, ok := refs["refs/remotes/"+remote+"/"+name]; ok {
				return true
			}
		}
	}
	return false
}

func splitIntegrationRegisteredRemote(name string, remotes []string) (remote, branch string, ok bool) {
	best := ""
	for _, candidate := range remotes {
		candidate = strings.TrimSpace(candidate)
		if len(candidate) > len(best) && strings.HasPrefix(name, candidate+"/") {
			best = candidate
		}
	}
	if best == "" {
		return "", "", false
	}
	return best, strings.TrimPrefix(name, best+"/"), true
}

func preferredIntegrationRemote(remotes []string) string {
	for _, remote := range remotes {
		if remote == "origin" {
			return remote
		}
	}
	if len(remotes) > 0 {
		return remotes[0]
	}
	return ""
}

func splitIntegrationLines(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
