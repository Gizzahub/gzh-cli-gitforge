// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package workspacecli provides CLI commands for managing local workspaces
// with config-based repository synchronization.
//
// This package handles local config file operations:
//   - workspace init: Create empty config file
//   - workspace scan: Scan directory for git repos and generate config
//   - workspace sync: Clone/update repos based on config
//   - workspace status: Check workspace health
//   - workspace add: Add repository to config
//   - workspace validate: Validate config file
//
// For Forge API operations (GitHub, GitLab, Gitea), use reposynccli package.
package workspacecli
