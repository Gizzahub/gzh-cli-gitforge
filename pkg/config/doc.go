// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package config provides configuration management for gz-git CLI.
//
// # Overview
//
// This package implements a flexible configuration system with profiles,
// global defaults, and project-specific overrides. It follows a 5-layer
// precedence system to resolve configuration values.
//
// # Precedence Order (Highest to Lowest)
//
//  1. Command flags (e.g., --provider gitlab)
//  2. Project config (.gz-git.yaml in current dir or parent)
//  3. Active profile (~/.config/gz-git/profiles/{active}.yaml)
//  4. Global config (~/.config/gz-git/config.yaml)
//  5. Built-in defaults
//
// # File Locations
//
// Global configuration directory: ~/.config/gz-git/
//
//	~/.config/gz-git/
//	├── config.yaml              # Global config
//	├── profiles/
//	│   ├── default.yaml        # Default profile
//	│   ├── work.yaml           # User profiles
//	│   └── personal.yaml
//	└── state/
//	    └── active-profile.txt  # Currently active profile
//
// Project configuration file: .gz-git.yaml
//
// This file is auto-detected by walking up the directory tree from the
// current working directory to the home directory.
//
// # Usage Example
//
// Basic usage:
//
//	// Create a manager
//	mgr, err := config.NewManager()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Initialize config directory
//	if err := mgr.Initialize(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create a profile
//	profile := &config.Profile{
//	    Name:       "work",
//	    Provider:   "gitlab",
//	    BaseURL:    "https://gitlab.company.com",
//	    Token:      "${WORK_GITLAB_TOKEN}",
//	    CloneProto: "ssh",
//	    SSHPort:    2224,
//	    Parallel:   10,
//	}
//	if err := mgr.CreateProfile(profile); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Set active profile
//	if err := mgr.SetActiveProfile("work"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Load configuration with precedence
//	loader, err := config.NewLoader()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := loader.Load(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Resolve effective config (merge flags if any)
//	flags := map[string]interface{}{
//	    "parallel": 20, // Override profile value
//	}
//	effective, err := loader.ResolveConfig(flags)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use effective config
//	fmt.Printf("Provider: %s (from %s)\n", effective.Provider, effective.GetSource("provider"))
//	fmt.Printf("Parallel: %d (from %s)\n", effective.Parallel, effective.GetSource("parallel"))
//
// # Environment Variables
//
// Configuration files support environment variable expansion using ${VAR_NAME} syntax:
//
//	token: ${GITLAB_TOKEN}
//	baseURL: ${GITLAB_BASE_URL}
//
// This is the recommended way to store sensitive values like API tokens.
//
// # Security
//
// - Profile files are created with 0600 permissions (user read/write only)
// - Config directories are created with 0700 permissions (user access only)
// - Environment variable expansion is safe (no shell command execution)
// - Tokens should use ${ENV_VAR} syntax, not plain text
//
// # Validation
//
// All configuration structures are validated on load:
// - Required fields are checked
// - Enum values (provider, clone protocol, etc.) are validated
// - Port numbers and counts are range-checked
// - Profile names must be alphanumeric with dash/underscore
//
// Validation errors are returned with clear messages indicating the problem.
package config
