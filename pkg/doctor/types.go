// Copyright (c) 2026 Archmagece
// SPDX-License-Identifier: MIT

// Package doctor provides health check diagnostics for gz-git.
// It verifies system dependencies, configuration integrity, authentication,
// and repository state to help users diagnose issues.
package doctor

import "time"

// Status represents the result status of a single check.
type Status string

const (
	// StatusOK indicates the check passed.
	StatusOK Status = "ok"
	// StatusWarning indicates a non-critical issue.
	StatusWarning Status = "warning"
	// StatusError indicates a critical issue.
	StatusError Status = "error"
	// StatusUnreachable indicates the target cannot be reached.
	StatusUnreachable Status = "unreachable"
	// StatusSkipped indicates the check was skipped.
	StatusSkipped Status = "skipped"
)

// Category groups related checks together.
type Category string

const (
	// CategorySystem groups system dependency checks.
	CategorySystem Category = "system"
	// CategoryConfig groups configuration file checks.
	CategoryConfig Category = "config"
	// CategoryAuth groups authentication checks.
	CategoryAuth Category = "auth"
	// CategoryForge groups forge API connectivity checks.
	CategoryForge Category = "forge"
	// CategoryRepo groups repository state checks.
	CategoryRepo Category = "repo"
)

// CheckResult represents the outcome of a single diagnostic check.
type CheckResult struct {
	Name     string   `json:"name"`
	Category Category `json:"category"`
	Status   Status   `json:"status"`
	Message  string   `json:"message"`
	Detail   string   `json:"detail,omitempty"`
}

// Report holds the complete doctor diagnostic report.
type Report struct {
	Checks   []CheckResult `json:"checks"`
	Summary  Summary       `json:"summary"`
	Duration time.Duration `json:"duration"`
}

// Summary counts results by status.
type Summary struct {
	OK          int `json:"ok"`
	Warning     int `json:"warning"`
	Error       int `json:"error"`
	Unreachable int `json:"unreachable"`
	Skipped     int `json:"skipped"`
	Total       int `json:"total"`
}

// Options configures which checks to run.
type Options struct {
	SkipForge bool
	SkipRepo  bool
	Verbose   bool
	Directory string // working directory for repo checks
	ScanDepth int    // max directory depth for repo scanning (default 1)
}
