// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package stash provides git stash management operations.
//
// This package handles stash creation, listing, application, and cleanup
// for git repositories.
//
// # Features
//
//   - Stash creation with messages
//   - Stash listing and inspection
//   - Stash application and popping
//   - Bulk stash operations
//
// # Usage
//
//	manager := stash.NewManager(repoPath)
//	err := manager.Push("WIP: feature work")
//	stashes, err := manager.List()
//	err = manager.Pop()
package stash
