// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

// Package merge provides merge conflict detection and analysis.
//
// This package detects potential merge conflicts between branches
// before attempting the actual merge operation. Merge/rebase execution
// is intentionally out of scope (use plain git); gz-git's value is
// bulk-first diagnostics via ConflictDetector.
//
// # Features
//
//   - Conflict detection between branches
//   - Conflict file listing
//   - Merge base calculation
//   - Fast-forward checks and merge previews
//
// # Usage
//
//	detector := merge.NewConflictDetector(gitcmd.NewExecutor())
//	report, err := detector.Detect(ctx, repo, merge.DetectOptions{
//	    Source: "feature",
//	    Target: "main",
//	})
//	if err != nil {
//	    // handle
//	}
//	if report.TotalConflicts > 0 {
//	    // inspect report.Conflicts
//	}
package merge
