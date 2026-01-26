// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

// This file defines default values for repository operations, grouped by usage context.
// Using grouped constants makes the intended usage clear and allows different
// defaults for different operation types (e.g., local vs forge API operations).

// === Group 1: Local Repository Operations ===
// Used for local Git operations: status, fetch, pull, push, switch, stash, tag, branch, etc.
// These operations don't have rate limits, so higher parallelism is safe.
const (
	// DefaultLocalScanDepth is the default directory depth to scan for repositories.
	// maxDepth=1 means scan only direct children of root (depth 0 -> depth 1)
	// maxDepth=2 means scan root + 2 levels (depth 0 -> depth 1 -> depth 2)
	DefaultLocalScanDepth = 1

	// DefaultLocalParallel is the default number of parallel workers for local Git operations.
	DefaultLocalParallel = 10
)

// === Group 2: Remote/Forge API Operations ===
// Used for GitHub/GitLab/Gitea API calls (sync from-forge, config generate, etc.)
// Lower parallelism to respect rate limits and avoid API throttling.
const (
	// DefaultForgeParallel is the default number of parallel workers for forge API operations.
	// Set lower than local operations to respect API rate limits.
	DefaultForgeParallel = 4
)

// === Group 3: Clone Operations ===
// Used for bulk clone operations (workspace sync, clone command).
const (
	// DefaultCloneParallel is the default number of parallel clone operations.
	DefaultCloneParallel = 10

	// DefaultCloneRetries is the default number of retry attempts for failed clone operations.
	DefaultCloneRetries = 3
)

// === Legacy Aliases (for backward compatibility) ===
// These maintain compatibility with existing code that uses the old constant names.
// New code should prefer the group-specific constants above.
const (
	// DefaultBulkParallel is an alias for DefaultLocalParallel.
	// Deprecated: Use DefaultLocalParallel for local operations or DefaultForgeParallel for API operations.
	DefaultBulkParallel = DefaultLocalParallel

	// DefaultBulkMaxDepth is an alias for DefaultLocalScanDepth.
	// Deprecated: Use DefaultLocalScanDepth instead.
	DefaultBulkMaxDepth = DefaultLocalScanDepth
)
