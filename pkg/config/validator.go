// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	// envVarPattern matches ${VAR_NAME} syntax
	envVarPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

	// validProfileName matches valid profile names (alphanumeric, dash, underscore)
	validProfileName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// validProviders lists supported forge providers
	validProviders = map[string]bool{
		"github": true,
		"gitlab": true,
		"gitea":  true,
	}

	// validCloneProtos lists supported clone protocols
	validCloneProtos = map[string]bool{
		"ssh":   true,
		"https": true,
	}

	// validSubgroupModes lists supported subgroup modes
	validSubgroupModes = map[string]bool{
		"flat":   true,
		"nested": true,
	}

	// validSyncStrategies lists supported sync strategies
	validSyncStrategies = map[string]bool{
		"pull":  true,
		"reset": true,
		"skip":  true,
	}
)

// Validator handles configuration validation and environment variable expansion.
type Validator struct {
	// ExpandEnvVars enables environment variable expansion
	ExpandEnvVars bool
}

// NewValidator creates a new Validator with default settings.
func NewValidator() *Validator {
	return &Validator{
		ExpandEnvVars: true,
	}
}

// ValidateProfile validates a profile configuration.
func (v *Validator) ValidateProfile(p *Profile) error {
	if p == nil {
		return fmt.Errorf("profile is nil")
	}

	// Validate profile name
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if !validProfileName.MatchString(p.Name) {
		return fmt.Errorf("invalid profile name '%s': must contain only alphanumeric, dash, or underscore", p.Name)
	}

	// Validate provider if set
	if p.Provider != "" && !validProviders[p.Provider] {
		return fmt.Errorf("invalid provider '%s': must be github, gitlab, or gitea", p.Provider)
	}

	// Validate clone protocol if set
	if p.CloneProto != "" && !validCloneProtos[p.CloneProto] {
		return fmt.Errorf("invalid clone protocol '%s': must be ssh or https", p.CloneProto)
	}

	// Validate SSH port if set
	if p.SSHPort < 0 || p.SSHPort > 65535 {
		return fmt.Errorf("invalid SSH port %d: must be between 0 and 65535", p.SSHPort)
	}

	// Validate parallel count if set
	if p.Parallel < 0 {
		return fmt.Errorf("invalid parallel count %d: must be >= 0", p.Parallel)
	}

	// Validate subgroup mode if set
	if p.SubgroupMode != "" && !validSubgroupModes[p.SubgroupMode] {
		return fmt.Errorf("invalid subgroup mode '%s': must be flat or nested", p.SubgroupMode)
	}

	// Validate sync config if set
	if p.Sync != nil {
		if err := v.ValidateSyncConfig(p.Sync); err != nil {
			return fmt.Errorf("invalid sync config: %w", err)
		}
	}

	return nil
}

// ValidateSyncConfig validates sync configuration.
func (v *Validator) ValidateSyncConfig(s *SyncConfig) error {
	if s == nil {
		return nil
	}

	// Validate strategy if set
	if s.Strategy != "" && !validSyncStrategies[s.Strategy] {
		return fmt.Errorf("invalid sync strategy '%s': must be pull, reset, or skip", s.Strategy)
	}

	// Validate max retries if set
	if s.MaxRetries < 0 {
		return fmt.Errorf("invalid max retries %d: must be >= 0", s.MaxRetries)
	}

	return nil
}

// ValidateGlobalConfig validates global configuration.
func (v *Validator) ValidateGlobalConfig(g *GlobalConfig) error {
	if g == nil {
		return fmt.Errorf("global config is nil")
	}

	// Validate active profile name if set
	if g.ActiveProfile != "" && !validProfileName.MatchString(g.ActiveProfile) {
		return fmt.Errorf("invalid active profile name '%s': must contain only alphanumeric, dash, or underscore", g.ActiveProfile)
	}

	return nil
}

// ValidateProjectConfig validates project configuration.
func (v *Validator) ValidateProjectConfig(p *ProjectConfig) error {
	if p == nil {
		return fmt.Errorf("project config is nil")
	}

	// Validate profile name if set
	if p.Profile != "" && !validProfileName.MatchString(p.Profile) {
		return fmt.Errorf("invalid profile name '%s': must contain only alphanumeric, dash, or underscore", p.Profile)
	}

	// Validate sync config if set
	if p.Sync != nil {
		if err := v.ValidateSyncConfig(p.Sync); err != nil {
			return fmt.Errorf("invalid sync config: %w", err)
		}
	}

	return nil
}

// ExpandEnvVarsInProfile expands environment variables in a profile.
// Variables use ${VAR_NAME} syntax.
func (v *Validator) ExpandEnvVarsInProfile(p *Profile) error {
	if !v.ExpandEnvVars || p == nil {
		return nil
	}

	var err error

	// Expand token
	p.Token, err = v.expandString(p.Token)
	if err != nil {
		return fmt.Errorf("failed to expand token: %w", err)
	}

	// Expand base URL
	p.BaseURL, err = v.expandString(p.BaseURL)
	if err != nil {
		return fmt.Errorf("failed to expand baseURL: %w", err)
	}

	return nil
}

// ExpandEnvVarsInGlobalConfig expands environment variables in global config.
func (v *Validator) ExpandEnvVarsInGlobalConfig(g *GlobalConfig) error {
	if !v.ExpandEnvVars || g == nil {
		return nil
	}

	// Expand environment tokens
	for envName, env := range g.Environments {
		var err error
		env.GitHubToken, err = v.expandString(env.GitHubToken)
		if err != nil {
			return fmt.Errorf("failed to expand %s.githubToken: %w", envName, err)
		}

		env.GitLabToken, err = v.expandString(env.GitLabToken)
		if err != nil {
			return fmt.Errorf("failed to expand %s.gitlabToken: %w", envName, err)
		}

		env.GiteaToken, err = v.expandString(env.GiteaToken)
		if err != nil {
			return fmt.Errorf("failed to expand %s.giteaToken: %w", envName, err)
		}

		g.Environments[envName] = env
	}

	return nil
}

// expandString expands environment variables in a string.
// Returns the expanded string or an error if expansion fails.
func (v *Validator) expandString(s string) (string, error) {
	if s == "" {
		return s, nil
	}

	// Find all ${VAR} occurrences
	result := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]

		// Get value from environment
		value := os.Getenv(varName)
		if value == "" {
			// Warn about missing env var (but don't fail)
			fmt.Fprintf(os.Stderr, "Warning: environment variable %s is not set\n", varName)
		}

		return value
	})

	return result, nil
}

// IsValidProfileName checks if a profile name is valid.
func IsValidProfileName(name string) bool {
	return validProfileName.MatchString(name)
}

// IsValidProvider checks if a provider name is valid.
func IsValidProvider(provider string) bool {
	return validProviders[provider]
}

// IsValidCloneProto checks if a clone protocol is valid.
func IsValidCloneProto(proto string) bool {
	return validCloneProtos[proto]
}

// IsValidSyncStrategy checks if a sync strategy is valid.
func IsValidSyncStrategy(strategy string) bool {
	return validSyncStrategies[strategy]
}

// SanitizeToken removes credentials from URLs for safe logging.
func SanitizeToken(s string) string {
	// Replace common token patterns in URLs
	s = regexp.MustCompile(`://[^:]+:[^@]+@`).ReplaceAllString(s, "://***:***@")
	s = regexp.MustCompile(`\?.*token=[^&]+`).ReplaceAllString(s, "?token=***")
	s = regexp.MustCompile(`&token=[^&]+`).ReplaceAllString(s, "&token=***")

	return s
}

// NormalizeProvider converts provider name to lowercase.
func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// ================================================================================
// Recursive Configuration Validation (NEW!)
// ================================================================================

// ValidateConfig validates a recursive hierarchical config.
func (v *Validator) ValidateConfig(c *Config) error {
	if c == nil {
		return nil // nil config is valid (optional)
	}

	// Validate provider if specified
	if c.Provider != "" && !IsValidProvider(c.Provider) {
		return fmt.Errorf("invalid provider '%s': must be github, gitlab, gitea, or bitbucket", c.Provider)
	}

	// Validate clone protocol if specified
	if c.CloneProto != "" && !IsValidCloneProto(c.CloneProto) {
		return fmt.Errorf("invalid clone protocol '%s': must be ssh or https", c.CloneProto)
	}

	// Validate SSH port if specified
	if c.SSHPort != 0 && (c.SSHPort < 1 || c.SSHPort > 65535) {
		return fmt.Errorf("invalid SSH port %d: must be between 1 and 65535", c.SSHPort)
	}

	// Validate parallel count if specified
	if c.Parallel < 0 {
		return fmt.Errorf("invalid parallel count %d: must be non-negative", c.Parallel)
	}

	// Validate subgroup mode if specified
	if c.SubgroupMode != "" && c.SubgroupMode != "flat" && c.SubgroupMode != "nested" {
		return fmt.Errorf("invalid subgroup mode '%s': must be flat or nested", c.SubgroupMode)
	}

	// Validate command-specific configs
	if c.Sync != nil {
		if err := v.ValidateSyncConfig(c.Sync); err != nil {
			return fmt.Errorf("sync config validation failed: %w", err)
		}
	}

	// Validate children
	for i, child := range c.Children {
		if err := v.ValidateChildEntry(&child); err != nil {
			return fmt.Errorf("child[%d] validation failed: %w", i, err)
		}
	}

	// Validate discovery config
	if c.Discovery != nil {
		if err := v.ValidateDiscoveryConfig(c.Discovery); err != nil {
			return fmt.Errorf("discovery config validation failed: %w", err)
		}
	}

	return nil
}

// ValidateChildEntry validates a child entry.
func (v *Validator) ValidateChildEntry(c *ChildEntry) error {
	if c == nil {
		return fmt.Errorf("child entry is nil")
	}

	// Validate path
	if c.Path == "" {
		return fmt.Errorf("child path is empty")
	}

	// Validate type
	if !c.Type.IsValid() {
		return fmt.Errorf("invalid child type '%s': must be 'config' or 'git'", c.Type)
	}

	// Validate that configFile is only used with type=config
	if c.Type == ChildTypeGit && c.ConfigFile != "" {
		return fmt.Errorf("configFile cannot be specified for type=git")
	}

	// Validate command-specific overrides
	if c.Sync != nil {
		if err := v.ValidateSyncConfig(c.Sync); err != nil {
			return fmt.Errorf("child sync config validation failed: %w", err)
		}
	}

	return nil
}

// ValidateDiscoveryConfig validates discovery configuration.
func (v *Validator) ValidateDiscoveryConfig(d *DiscoveryConfig) error {
	if d == nil {
		return nil
	}

	// Validate mode
	if d.Mode != "" && !d.Mode.IsValid() {
		return fmt.Errorf("invalid discovery mode '%s': must be 'explicit', 'auto', or 'hybrid'", d.Mode)
	}

	return nil
}
