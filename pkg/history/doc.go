// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package history provides git history analysis and contributor statistics.
//
// This package analyzes git commit history to extract insights such as
// contributor statistics, commit frequency, and file change patterns.
//
// # Features
//
//   - Contributor statistics (commits, lines added/removed)
//   - Commit frequency analysis
//   - File evolution tracking
//   - Blame analysis
//
// # Usage
//
//	analyzer := history.NewAnalyzer(repoPath)
//	stats, err := analyzer.ContributorStats()
package history
