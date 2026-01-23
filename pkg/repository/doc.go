// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package repository provides git repository abstraction and bulk operations.
//
// This package implements the core repository operations including cloning,
// fetching, pulling, pushing, and status checking. It supports both single
// repository and bulk (multi-repository) operations.
//
// # Bulk Operations
//
// By default, gz-git operates in bulk mode, scanning directories and
// processing multiple repositories in parallel.
//
// Defaults:
//   - Scan depth: 1 (current directory + 1 level)
//   - Parallel jobs: 10
//
// # Features
//
//   - Repository state management
//   - Bulk clone/fetch/pull/push
//   - Status checking with divergence detection
//   - Branch operations across repositories
//   - Refspec support for push
//
// # Usage
//
//	client := repository.NewClient()
//	results, err := client.BulkStatus(ctx, "/path/to/workspace", repository.BulkOptions{
//	    ScanDepth: 1,
//	    Parallel:  10,
//	})
package repository
