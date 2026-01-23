// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package branch provides git branch management operations.
//
// This package handles branch listing, switching, cleanup, and worktree
// operations across single and multiple repositories.
//
// # Features
//
//   - Branch listing and filtering
//   - Safe branch switching
//   - Cleanup of merged/stale/gone branches
//   - Worktree management
//
// # Usage
//
//	service := branch.NewService(repoPath)
//	branches, err := service.List(branch.ListOptions{Remote: true})
//	err = service.Cleanup(branch.CleanupOptions{DryRun: true})
package branch
