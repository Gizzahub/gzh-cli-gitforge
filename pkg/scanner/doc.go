// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package scanner provides local git repository discovery and scanning.
//
// This package finds git repositories in a directory tree by detecting
// .git directories. It supports configurable scan depth and filtering.
//
// # Usage
//
//	repos, err := scanner.Scan("/path/to/workspace", scanner.Options{
//	    MaxDepth: 2,
//	    Exclude:  []string{"vendor", "node_modules"},
//	})
package scanner
