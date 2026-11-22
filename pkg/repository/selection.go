// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package repository

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ErrSelectionUnmatched marks a listed selection entry that the scan could not
// satisfy. A listed run refuses before touching any repository instead of
// running on the subset it did find.
var ErrSelectionUnmatched = errors.New("listed repository selection not satisfied")

// RepoSelection restricts a bulk operation to an explicit list of
// repositories. The zero value selects every scanned repository, which keeps
// existing callers unchanged.
//
// Listed mode is all-or-nothing: every entry must resolve to at least one
// scanned repository that also passes the include/exclude filters, or the
// operation fails before any repository is processed. Selecting a subset
// silently would let a typo or a missing checkout turn into a partial run.
type RepoSelection struct {
	// Listed switches the operation from "every scanned repository" to "only
	// the repositories named below". Listed with no entries selects nothing.
	Listed bool

	// Paths are exact repository paths, slash separated and relative to the
	// scan directory. "." is the scan directory itself.
	Paths []string

	// Prefixes are directories whose scanned repositories are all selected,
	// relative to the scan directory. A declared forge or config workspace is
	// a directory of repositories rather than one repository.
	Prefixes []string
}

// NormalizeSelectionPath validates one selection entry and returns its
// canonical form: slash separated, cleaned, relative to the scan directory,
// with "." for the scan directory itself. Absolute paths and entries that
// escape the scan directory are refused, because the scan can never reach
// them and the entry would only ever fail later with a less precise error.
func NormalizeSelectionPath(entry string) (string, error) {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return "", fmt.Errorf("empty repository path")
	}
	slashed := filepath.ToSlash(trimmed)
	if path.IsAbs(slashed) || filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("repository path %q must be relative to the scan directory", entry)
	}
	cleaned := path.Clean(slashed)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("repository path %q escapes the scan directory", entry)
	}
	return cleaned, nil
}

// requiredDepth returns the scan depth that reaches every entry. Depth counts
// path segments below the scan directory, matching walkDirectoryWithConfig:
// "a" is at depth 1 and "a/b" at depth 2. A prefix needs one more level so its
// direct child repositories are reached.
func (s RepoSelection) requiredDepth() int {
	depth := 0
	for _, p := range s.Paths {
		if d := segmentCount(p); d > depth {
			depth = d
		}
	}
	for _, p := range s.Prefixes {
		if d := segmentCount(p) + 1; d > depth {
			depth = d
		}
	}
	return depth
}

func segmentCount(p string) int {
	if p == "." || p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}

// relativeRepoPath returns repo relative to root in the selection's canonical
// form ("." for the root itself).
func relativeRepoPath(root, repo string) string {
	rel, err := filepath.Rel(root, repo)
	if err != nil {
		return filepath.ToSlash(repo)
	}
	return filepath.ToSlash(rel)
}

func underPrefix(rel, prefix string) bool {
	if prefix == "." {
		return true
	}
	return rel == prefix || strings.HasPrefix(rel, prefix+"/")
}

// applySelection keeps the scanned repositories the selection names. It
// returns ErrSelectionUnmatched naming every entry no scanned repository
// satisfied.
func applySelection(root string, repos []string, sel RepoSelection) ([]string, error) {
	if !sel.Listed {
		return repos, nil
	}
	exact := make(map[string]bool, len(sel.Paths))
	for _, p := range sel.Paths {
		exact[p] = false
	}
	prefixHit := make(map[string]bool, len(sel.Prefixes))
	for _, p := range sel.Prefixes {
		prefixHit[p] = false
	}

	selected := make([]string, 0, len(repos))
	for _, repo := range repos {
		rel := relativeRepoPath(root, repo)
		keep := false
		if _, ok := exact[rel]; ok {
			exact[rel] = true
			keep = true
		}
		for prefix := range prefixHit {
			if underPrefix(rel, prefix) {
				prefixHit[prefix] = true
				keep = true
			}
		}
		if keep {
			selected = append(selected, repo)
		}
	}

	var missing []string
	for p, hit := range exact {
		if !hit {
			missing = append(missing, p)
		}
	}
	for p, hit := range prefixHit {
		if !hit {
			missing = append(missing, p+"/")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("%w: no git repository found at %s under %s (not cloned, not a repository, or skipped by the scanner); nothing was run",
			ErrSelectionUnmatched, strings.Join(missing, ", "), root)
	}
	return selected, nil
}

// checkSelectionFiltered refuses a listed run whose include/exclude filters
// contradict the list. An exact path the filters dropped is a contradiction
// the operator has to resolve, not a subset to run. A prefix (a forge or
// config workspace) is a directory of repositories, and the filters may
// narrow it, so defaults.scan.exclude keeps working inside workspaces; only a
// prefix the filters emptied is refused.
func checkSelectionFiltered(root string, selected, filtered []string, sel RepoSelection) error {
	if !sel.Listed || len(selected) == len(filtered) {
		return nil
	}
	exact := make(map[string]bool, len(sel.Paths))
	for _, p := range sel.Paths {
		exact[p] = true
	}
	var keptRel []string
	kept := make(map[string]bool, len(filtered))
	for _, repo := range filtered {
		kept[repo] = true
		keptRel = append(keptRel, relativeRepoPath(root, repo))
	}
	var dropped []string
	for _, repo := range selected {
		if rel := relativeRepoPath(root, repo); !kept[repo] && exact[rel] {
			dropped = append(dropped, rel)
		}
	}
	for _, prefix := range sel.Prefixes {
		if anyUnderPrefix(keptRel, prefix) {
			continue
		}
		if prefix == "." {
			dropped = append(dropped, "every repository under the scan directory")
		} else {
			dropped = append(dropped, prefix+"/ (every repository under it)")
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	sort.Strings(dropped)
	return fmt.Errorf("%w: %s excluded by --include/--exclude or defaults.scan.exclude; nothing was run",
		ErrSelectionUnmatched, strings.Join(dropped, ", "))
}

func anyUnderPrefix(rels []string, prefix string) bool {
	for _, rel := range rels {
		if underPrefix(rel, prefix) {
			return true
		}
	}
	return false
}
