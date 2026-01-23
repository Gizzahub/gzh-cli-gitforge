// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package templates provides configuration file template generation.
//
// This package contains templates for generating workspace and repository
// configuration files in various formats.
//
// # Templates
//
//   - Repository config (.gz-git.yaml)
//   - Workspace config (hierarchical)
//   - Profile templates
//
// # Usage
//
//	content, err := templates.Render(templates.RepositoriesConfig, data)
package templates
