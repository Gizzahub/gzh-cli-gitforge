// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package watch provides repository monitoring and change detection.
//
// This package monitors git repositories for changes including file
// modifications, branch updates, and remote changes.
//
// # Features
//
//   - File system change detection
//   - Branch change monitoring
//   - Remote update detection
//   - Configurable polling intervals
//
// # Usage
//
//	watcher := watch.NewWatcher(repoPath, watch.Options{
//	    Interval: 5 * time.Second,
//	})
//	events := watcher.Watch(ctx)
package watch
