// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package merge

import "errors"

var (
	// ErrInvalidBranch indicates an invalid branch reference.
	ErrInvalidBranch = errors.New("invalid branch reference")

	// ErrBranchNotFound indicates the specified branch does not exist.
	ErrBranchNotFound = errors.New("branch not found")

	// ErrNoMergeBase indicates no common ancestor found.
	ErrNoMergeBase = errors.New("no merge base found between branches")
)
