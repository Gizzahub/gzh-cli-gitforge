// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposync

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// RepositoryPatternFilter matches repositories by name or forge full name.
type RepositoryPatternFilter struct {
	include []*regexp.Regexp
	exclude []*regexp.Regexp
}

// NewRepositoryPatternFilter compiles include/exclude regex patterns.
func NewRepositoryPatternFilter(includePatterns, excludePatterns []string) (*RepositoryPatternFilter, error) {
	include, err := compileRepositoryPatterns("include", includePatterns)
	if err != nil {
		return nil, err
	}

	exclude, err := compileRepositoryPatterns("exclude", excludePatterns)
	if err != nil {
		return nil, err
	}

	return &RepositoryPatternFilter{
		include: include,
		exclude: exclude,
	}, nil
}

func compileRepositoryPatterns(kind string, patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid %s pattern %q: %w", kind, pattern, err)
		}
		compiled = append(compiled, re)
	}

	return compiled, nil
}

// Match returns true when the repository passes include/exclude filtering.
func (f *RepositoryPatternFilter) Match(repo *provider.Repository) bool {
	if f == nil || repo == nil {
		return true
	}

	candidates := repositoryPatternCandidates(repo)

	if len(f.include) > 0 && !matchesAnyRepositoryPattern(f.include, candidates) {
		return false
	}

	if len(f.exclude) > 0 && matchesAnyRepositoryPattern(f.exclude, candidates) {
		return false
	}

	return true
}

func repositoryPatternCandidates(repo *provider.Repository) []string {
	candidates := []string{repo.Name}
	if repo.FullName != "" && repo.FullName != repo.Name {
		candidates = append(candidates, repo.FullName)
	}
	return candidates
}

func matchesAnyRepositoryPattern(patterns []*regexp.Regexp, candidates []string) bool {
	for _, pattern := range patterns {
		if slices.ContainsFunc(candidates, pattern.MatchString) {
			return true
		}
	}
	return false
}
