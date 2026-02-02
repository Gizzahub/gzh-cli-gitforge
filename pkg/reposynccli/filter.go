// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// MetadataFilter filters repositories by metadata fields.
type MetadataFilter struct {
	Languages     []string  // Lowercase language names to include (e.g., ["go", "rust"])
	MinStars      int       // Minimum star count (0 = no minimum)
	MaxStars      int       // Maximum star count (0 = unlimited)
	LastPushAfter time.Time // Only include repos pushed after this time
}

// IsEmpty returns true if no filters are configured.
func (f *MetadataFilter) IsEmpty() bool {
	return len(f.Languages) == 0 &&
		f.MinStars == 0 &&
		f.MaxStars == 0 &&
		f.LastPushAfter.IsZero()
}

// Match returns true if the repository passes all filter criteria.
func (f *MetadataFilter) Match(repo *provider.Repository) bool {
	// Language filter
	if len(f.Languages) > 0 {
		repoLang := strings.ToLower(repo.Language)
		if repoLang == "" || !containsString(f.Languages, repoLang) {
			return false
		}
	}

	// Stars filter (minimum)
	if f.MinStars > 0 && repo.Stars < f.MinStars {
		return false
	}

	// Stars filter (maximum, 0 = unlimited)
	if f.MaxStars > 0 && repo.Stars > f.MaxStars {
		return false
	}

	// Activity filter (last push)
	if !f.LastPushAfter.IsZero() && repo.PushedAt.Before(f.LastPushAfter) {
		return false
	}

	return true
}

// containsString checks if slice contains the target string.
func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// ParseDuration parses duration strings with extended unit support.
// Supports: s, m (minute), h, d (day), w (week), M (month), y (year)
// Examples: "30d", "6M", "1y", "2w"
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Find the unit suffix
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	// Extract number and unit
	unit := s[len(s)-1:]
	numStr := s[:len(s)-1]

	// Handle 2-char units (not needed for now, but extensible)
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %s", numStr)
	}

	if num < 0 {
		return 0, fmt.Errorf("duration cannot be negative: %d", num)
	}

	const (
		day   = 24 * time.Hour
		week  = 7 * day
		month = 30 * day  // Approximate
		year  = 365 * day // Approximate
	)

	switch unit {
	case "s":
		return time.Duration(num) * time.Second, nil
	case "m":
		return time.Duration(num) * time.Minute, nil
	case "h":
		return time.Duration(num) * time.Hour, nil
	case "d":
		return time.Duration(num) * day, nil
	case "w":
		return time.Duration(num) * week, nil
	case "M":
		return time.Duration(num) * month, nil
	case "y":
		return time.Duration(num) * year, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s (use s, m, h, d, w, M, y)", unit)
	}
}

// ParseLanguages parses a comma-separated list of languages.
// Returns lowercase language names.
// Examples: "go,rust" -> ["go", "rust"], "Go, Rust" -> ["go", "rust"]
func ParseLanguages(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	languages := make([]string, 0, len(parts))

	for _, p := range parts {
		lang := strings.TrimSpace(p)
		if lang != "" {
			languages = append(languages, strings.ToLower(lang))
		}
	}

	return languages
}

// PrintWarningsForProvider prints warnings about filter limitations for specific providers.
func PrintWarningsForProvider(providerName string, filter *MetadataFilter, w io.Writer) {
	if filter == nil || filter.IsEmpty() {
		return
	}

	switch providerName {
	case "gitlab":
		if len(filter.Languages) > 0 {
			fmt.Fprintf(w, "⚠ Warning: GitLab API does not provide language information. "+
				"--language filter may not work as expected.\n")
		}
	case "gitea":
		if len(filter.Languages) > 0 {
			fmt.Fprintf(w, "⚠ Warning: Gitea SDK does not expose language information. "+
				"--language filter may not work as expected.\n")
		}
	}
}

// BuildFilterFromOptions creates a MetadataFilter from CLI option values.
func BuildFilterFromOptions(language string, minStars, maxStars int, lastPushWithin string) (*MetadataFilter, error) {
	filter := &MetadataFilter{
		Languages: ParseLanguages(language),
		MinStars:  minStars,
		MaxStars:  maxStars,
	}

	// Parse last push duration
	if lastPushWithin != "" {
		duration, err := ParseDuration(lastPushWithin)
		if err != nil {
			return nil, fmt.Errorf("invalid --last-push-within: %w", err)
		}
		filter.LastPushAfter = time.Now().Add(-duration)
	}

	return filter, nil
}
