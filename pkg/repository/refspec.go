// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"regexp"
	"strings"
)

// refspecPattern matches valid Git refspec formats
// Supports: branch, local:remote, refs/heads/branch:refs/heads/branch
var refspecPattern = regexp.MustCompile(`^(?:[+])?(?:([^:]+)(?::([^:]+))?)?$`)

// branchNamePattern matches valid Git branch names
// Git branch names cannot contain: .., ~, ^, :, ?, *, [, \, control characters
// Cannot start or end with /, cannot have consecutive slashes
var branchNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9._/]*[a-zA-Z0-9]$`)

// ParsedRefspec represents a parsed Git refspec.
type ParsedRefspec struct {
	// Source is the local branch/ref (left side of :)
	Source string
	// Destination is the remote branch/ref (right side of :), empty if not specified
	Destination string
	// Force indicates if this is a force push (+ prefix)
	Force bool
}

// ValidateRefspec validates a Git refspec string and returns parsed components.
// Returns an error if the refspec is invalid.
//
// Valid formats:
//   - "branch"                    -> push local branch to remote branch with same name
//   - "local:remote"              -> push local branch to remote branch
//   - "+local:remote"             -> force push local to remote
//   - "refs/heads/main:refs/heads/master" -> full ref path
//
// Invalid formats:
//   - ""                          -> empty refspec
//   - "local::remote"             -> double colon
//   - "local:remote:extra"        -> too many colons
//   - "-invalid"                  -> branch name starting with -
//   - "branch."                   -> branch name ending with .
//   - "branch..name"              -> consecutive dots
//   - "branch name"               -> contains space
func ValidateRefspec(refspec string) (*ParsedRefspec, error) {
	if refspec == "" {
		return nil, fmt.Errorf("refspec cannot be empty")
	}

	// Check for invalid characters that git doesn't allow
	invalidChars := []string{" ", "\t", "\n", "~", "^", ":", "?", "*", "[", "\\", "\x00"}
	for _, char := range invalidChars {
		if strings.Contains(refspec, char) && char != ":" {
			return nil, fmt.Errorf("refspec contains invalid character: %q", char)
		}
	}

	parsed := &ParsedRefspec{}

	// Check for force prefix
	cleanRefspec := refspec
	if strings.HasPrefix(cleanRefspec, "+") {
		parsed.Force = true
		cleanRefspec = strings.TrimPrefix(cleanRefspec, "+")
	}

	// Split on colon
	parts := strings.SplitN(cleanRefspec, ":", 2)
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("refspec source (left side) cannot be empty")
	}

	parsed.Source = parts[0]

	if len(parts) == 2 {
		if parts[1] == "" {
			return nil, fmt.Errorf("refspec destination (right side) cannot be empty when : is present")
		}
		parsed.Destination = parts[1]
	} else if len(parts) > 2 {
		return nil, fmt.Errorf("invalid refspec format: %q (too many colons)", refspec)
	}

	// Validate source branch name (unless it's a full ref path)
	if !strings.HasPrefix(parsed.Source, "refs/") {
		if err := validateBranchName(parsed.Source); err != nil {
			return nil, fmt.Errorf("invalid source branch name %q: %w", parsed.Source, err)
		}
	}

	// Validate destination branch name if specified (unless it's a full ref path)
	if parsed.Destination != "" && !strings.HasPrefix(parsed.Destination, "refs/") {
		if err := validateBranchName(parsed.Destination); err != nil {
			return nil, fmt.Errorf("invalid destination branch name %q: %w", parsed.Destination, err)
		}
	}

	return parsed, nil
}

// validateBranchName validates a Git branch name.
func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Check length (Git limit is 255, but be conservative)
	if len(name) > 255 {
		return fmt.Errorf("branch name too long (max 255 characters)")
	}

	// Check for invalid patterns (must come before individual character checks)
	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name contains invalid pattern \"..\"")
	}
	if strings.Contains(name, "//") {
		return fmt.Errorf("branch name contains invalid pattern \"//\"")
	}
	if strings.Contains(name, "@{") {
		return fmt.Errorf("branch name contains invalid pattern \"@{\"")
	}

	// Check if starts with invalid characters
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("branch name cannot start with -")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("branch name cannot start with .")
	}
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("branch name cannot start with /")
	}

	// Check if ends with invalid characters/patterns
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("branch name cannot end with .")
	}
	if strings.HasSuffix(name, "/") {
		return fmt.Errorf("branch name cannot end with /")
	}
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("branch name cannot end with .lock")
	}

	// Check for control characters and other invalid chars
	for _, char := range name {
		if char < 32 || char == 127 { // Control characters
			return fmt.Errorf("branch name contains control character")
		}
		// Git doesn't allow: space, ~, ^, :, ?, *, [, \
		if char == ' ' || char == '~' || char == '^' || char == ':' || char == '?' || char == '*' || char == '[' || char == '\\' {
			return fmt.Errorf("branch name contains invalid character: %c", char)
		}
	}

	return nil
}

// String returns the string representation of a refspec.
func (r *ParsedRefspec) String() string {
	var result strings.Builder

	if r.Force {
		result.WriteString("+")
	}

	result.WriteString(r.Source)

	if r.Destination != "" {
		result.WriteString(":")
		result.WriteString(r.Destination)
	}

	return result.String()
}

// GetSourceBranch returns the source branch name.
// If the source is a full ref path (refs/heads/branch), it extracts the branch name.
func (r *ParsedRefspec) GetSourceBranch() string {
	if strings.HasPrefix(r.Source, "refs/heads/") {
		return strings.TrimPrefix(r.Source, "refs/heads/")
	}
	return r.Source
}

// GetDestinationBranch returns the destination branch name.
// If destination is empty, returns source branch name.
// If the destination is a full ref path (refs/heads/branch), it extracts the branch name.
func (r *ParsedRefspec) GetDestinationBranch() string {
	dest := r.Destination
	if dest == "" {
		dest = r.Source
	}

	if strings.HasPrefix(dest, "refs/heads/") {
		return strings.TrimPrefix(dest, "refs/heads/")
	}
	return dest
}
