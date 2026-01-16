// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// ValidateProvider validates that the value is a valid provider name.
// Returns nil for empty values (optional field).
func ValidateProvider(v string) error {
	if v == "" {
		return nil
	}
	if !config.IsValidProvider(v) {
		return errors.New("must be github, gitlab, or gitea")
	}
	return nil
}

// ValidateProviderRequired validates that the value is a valid, non-empty provider.
func ValidateProviderRequired(v string) error {
	if v == "" {
		return errors.New("provider is required")
	}
	return ValidateProvider(v)
}

// ValidateCloneProto validates the clone protocol.
func ValidateCloneProto(v string) error {
	if v == "" {
		return nil
	}
	if !config.IsValidCloneProto(v) {
		return errors.New("must be ssh or https")
	}
	return nil
}

// ValidatePort validates a port number string.
func ValidatePort(v string) error {
	if v == "" {
		return nil
	}
	port, err := strconv.Atoi(v)
	if err != nil {
		return errors.New("must be a number")
	}
	if port < 0 || port > 65535 {
		return errors.New("must be between 0 and 65535")
	}
	return nil
}

// ValidateURL validates a URL string.
// Returns nil for empty values (optional field).
func ValidateURL(v string) error {
	if v == "" {
		return nil
	}

	// Allow environment variable references
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		return nil
	}

	u, err := url.Parse(v)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL must start with http:// or https://")
	}

	if u.Host == "" {
		return errors.New("URL must include a host")
	}

	return nil
}

// ValidateURLRequired validates a non-empty URL.
func ValidateURLRequired(v string) error {
	if v == "" {
		return errors.New("URL is required")
	}
	return ValidateURL(v)
}

// ValidatePath validates a directory path.
// Returns nil for empty values (optional field).
func ValidatePath(v string) error {
	if v == "" {
		return nil
	}

	// Expand ~ to home directory for validation
	path := v
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + path[1:]
		}
	}

	// Check if parent directory exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if parent exists (directory will be created)
			parent := path[:strings.LastIndex(path, "/")]
			if parent == "" {
				parent = "/"
			}
			if _, parentErr := os.Stat(parent); parentErr == nil {
				return nil // Parent exists, directory can be created
			}
			return errors.New("path does not exist and parent directory not found")
		}
		return fmt.Errorf("cannot access path: %w", err)
	}

	// If it exists, it should be a directory
	if !info.IsDir() {
		return errors.New("path exists but is not a directory")
	}

	return nil
}

// ValidatePathRequired validates a non-empty path.
func ValidatePathRequired(v string) error {
	if v == "" {
		return errors.New("path is required")
	}
	return ValidatePath(v)
}

// ValidateNotEmpty validates that a string is not empty.
func ValidateNotEmpty(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("this field is required")
	}
	return nil
}

// ValidateOrganization validates an organization/group name.
func ValidateOrganization(v string) error {
	if v == "" {
		return errors.New("organization/group name is required")
	}

	// Basic validation - no spaces, no special chars except - and _
	for _, r := range v {
		if !(r >= 'a' && r <= 'z') &&
			!(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') &&
			r != '-' && r != '_' && r != '/' {
			return errors.New("invalid character in organization name")
		}
	}

	return nil
}

// ValidateParallel validates a parallel count string.
func ValidateParallel(v string) error {
	if v == "" {
		return nil
	}

	parallel, err := strconv.Atoi(v)
	if err != nil {
		return errors.New("must be a number")
	}

	if parallel < 1 || parallel > 50 {
		return errors.New("must be between 1 and 50")
	}

	return nil
}

// ValidateSubgroupMode validates the subgroup mode.
func ValidateSubgroupMode(v string) error {
	if v == "" {
		return nil
	}
	if v != "flat" && v != "nested" {
		return errors.New("must be flat or nested")
	}
	return nil
}

// ValidateToken validates a token (can be actual token or env var reference).
func ValidateToken(v string) error {
	// Token is optional in some contexts
	if v == "" {
		return nil
	}

	// Environment variable reference is always valid
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		// Extract var name and validate it
		varName := v[2 : len(v)-1]
		if varName == "" {
			return errors.New("empty environment variable name")
		}
		// Check for valid env var name
		for i, r := range varName {
			if i == 0 && r >= '0' && r <= '9' {
				return errors.New("environment variable name cannot start with a number")
			}
			if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return errors.New("invalid environment variable name")
			}
		}
		return nil
	}

	// Plain token should have reasonable length
	if len(v) < 10 {
		return errors.New("token seems too short")
	}

	return nil
}

// ValidateProfileName validates a profile name.
func ValidateProfileName(v string) error {
	if v == "" {
		return errors.New("profile name is required")
	}
	if !config.IsValidProfileName(v) {
		return errors.New("must contain only alphanumeric, dash, or underscore")
	}
	return nil
}

// ParsePort parses a port string to int, returning 0 for empty/invalid.
func ParsePort(v string) int {
	if v == "" {
		return 0
	}
	port, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return port
}

// ParseParallel parses a parallel count string to int, returning default for empty/invalid.
func ParseParallel(v string, defaultVal int) int {
	if v == "" {
		return defaultVal
	}
	parallel, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return parallel
}
