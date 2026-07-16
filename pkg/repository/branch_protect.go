// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package repository

// ProtectedBranches lists the branch names and patterns that require --force to
// delete. It is the single source of truth for protected-branch judgment: both
// the single-repo path (pkg/branch) and the bulk path (this package) resolve
// protection through IsProtected, so adding a pattern here closes it on every
// deletion path at once.
var ProtectedBranches = []string{
	"main",
	"master",
	"develop",
	"development",
	"release/*",
	"hotfix/*",
}

// IsProtected reports whether name matches a built-in protected branch pattern.
func IsProtected(name string) bool {
	for _, pattern := range ProtectedBranches {
		if matchBranchPattern(name, pattern) {
			return true
		}
	}
	return false
}

// matchBranchPattern checks if name matches pattern (supports a trailing *
// wildcard, e.g. "release/*").
func matchBranchPattern(name, pattern string) bool {
	if pattern == name {
		return true
	}
	if pattern != "" && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}
	return false
}
