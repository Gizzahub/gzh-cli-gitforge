// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// LoadEffectiveConfig loads configuration with precedence and merges with command flags.
// It handles profile override from --profile global flag.
//
// Usage:
//
//	effective, err := LoadEffectiveConfig(cmd, map[string]interface{}{
//	    "provider": provider,  // From command flags
//	    "org": org,
//	})
func LoadEffectiveConfig(cmd *cobra.Command, flags map[string]any) (*config.EffectiveConfig, error) {
	// Create loader. A failure here means the config directory itself could not
	// be resolved (e.g. $HOME unset), and the old defaults fallback hid it
	// behind a zero-value loader whose nil manager panics in Load.
	loader, err := config.NewLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create config loader: %w", err)
	}

	// Load configs (global, profile, project). A missing file is not an error —
	// the manager returns defaults and the project config may be nil — so any
	// error here is real: an unreadable file, a syntax error, or a removed key.
	// Failing fast beats running on defaults while silently dropping the
	// operator's configuration.
	if err := loader.Load(); err != nil {
		return nil, fmt.Errorf("failed to load config layers: %w", err)
	}

	// Apply profile override if provided via --profile flag
	if profileOverride != "" {
		mgr, err := config.NewManager()
		if err == nil && mgr.ProfileExists(profileOverride) {
			overrideProfile, err := mgr.LoadProfile(profileOverride)
			if err == nil {
				loader.SetActiveProfileInternal(overrideProfile)
			}
		}
	}

	// Resolve effective config with flags
	effective, err := loader.ResolveConfig(flags)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config: %w", err)
	}

	return effective, nil
}

// ApplyConfigToFlags applies config values to command flags if not already set.
// This is useful for commands that want to use config as defaults.
//
// Usage:
//
//	effective, err := LoadEffectiveConfig(cmd, nil)
//	if err != nil {
//	    return err
//	}
//	if provider == "" && effective.Provider != "" {
//	    provider = effective.Provider
//	}
func ApplyConfigToFlags(effective *config.EffectiveConfig, provider, baseURL, token *string, parallel *int) {
	if provider != nil && *provider == "" && effective.Provider != "" {
		*provider = effective.Provider
	}
	if baseURL != nil && *baseURL == "" && effective.BaseURL != "" {
		*baseURL = effective.BaseURL
	}
	if token != nil && *token == "" && effective.Token != "" {
		*token = effective.Token
	}
	if parallel != nil && *parallel == 0 && effective.Parallel > 0 {
		*parallel = effective.Parallel
	}
}

// PrintConfigSources prints the source of each config value (for debugging).
func PrintConfigSources(cmd *cobra.Command, effective *config.EffectiveConfig) {
	if !verbose {
		return
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "\n🔍 Configuration Sources:")
	if effective.Provider != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "  Provider: %s (from %s)\n", effective.Provider, effective.GetSource("provider"))
	}
	if effective.BaseURL != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "  BaseURL: %s (from %s)\n", effective.BaseURL, effective.GetSource("baseURL"))
	}
	if effective.Token != "" {
		sanitized := config.SanitizeToken(effective.Token)
		fmt.Fprintf(cmd.ErrOrStderr(), "  Token: %s (from %s)\n", sanitized, effective.GetSource("token"))
	}
	if effective.Parallel > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "  Parallel: %d (from %s)\n", effective.Parallel, effective.GetSource("parallel"))
	}
	fmt.Fprintln(cmd.ErrOrStderr())
}
