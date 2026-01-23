// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package merge provides merge conflict detection and analysis.
//
// This package detects potential merge conflicts between branches
// before attempting the actual merge operation.
//
// # Features
//
//   - Conflict detection between branches
//   - Conflict file listing
//   - Merge base calculation
//   - Conflict resolution suggestions
//
// # Usage
//
//	detector := merge.NewDetector(repoPath)
//	conflicts, err := detector.Detect("feature", "main")
//	if len(conflicts) > 0 {
//	    // Handle conflicts
//	}
package merge
