// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package tag provides git tag management and semantic versioning.
//
// This package handles tag creation, listing, and semantic version
// calculations for release management.
//
// # Features
//
//   - Tag creation (lightweight and annotated)
//   - Tag listing and filtering
//   - Semantic version parsing and incrementing
//   - Next version suggestions
//
// # Usage
//
//	manager := tag.NewManager(repoPath)
//	tags, err := manager.List()
//	nextVersion := tag.NextVersion(currentVersion, tag.Minor)
package tag
