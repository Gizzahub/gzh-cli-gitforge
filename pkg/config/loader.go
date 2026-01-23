// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"reflect"
)

// ConfigLoader handles configuration loading with 5-layer precedence.
//
// Precedence (highest to lowest):
//  1. Command flags
//  2. Project config (.gz-git.yaml)
//  3. Active profile
//  4. Global config
//  5. Built-in defaults
type ConfigLoader struct {
	manager       *Manager
	globalConfig  *GlobalConfig
	activeProfile *Profile
	projectConfig *ProjectConfig
}

// NewLoader creates a new configuration loader.
func NewLoader() (*ConfigLoader, error) {
	manager, err := NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	return &ConfigLoader{
		manager: manager,
	}, nil
}

// Load loads all configuration layers.
func (l *ConfigLoader) Load() error {
	// Load global config
	globalConfig, err := l.manager.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}
	l.globalConfig = globalConfig

	// Load active profile
	activeProfileName, err := l.manager.GetActiveProfile()
	if err != nil {
		return fmt.Errorf("failed to get active profile: %w", err)
	}

	if activeProfileName != "" && l.manager.ProfileExists(activeProfileName) {
		activeProfile, err := l.manager.LoadProfile(activeProfileName)
		if err != nil {
			return fmt.Errorf("failed to load active profile '%s': %w", activeProfileName, err)
		}
		l.activeProfile = activeProfile
	}

	// Load project config (may be nil if not found)
	projectConfig, err := l.manager.LoadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}
	l.projectConfig = projectConfig

	// If project specifies a different profile, load it
	if projectConfig != nil && projectConfig.Profile != "" && projectConfig.Profile != activeProfileName {
		projectProfile, err := l.manager.LoadProfile(projectConfig.Profile)
		if err != nil {
			return fmt.Errorf("failed to load project profile '%s': %w", projectConfig.Profile, err)
		}
		// Project profile takes precedence over active profile
		l.activeProfile = projectProfile
	}

	return nil
}

// ResolveConfig builds the effective configuration with precedence.
// Flags parameter contains command-line flag values (highest priority).
func (l *ConfigLoader) ResolveConfig(flags map[string]interface{}) (*EffectiveConfig, error) {
	effective := &EffectiveConfig{
		Sources: make(map[string]string),
	}

	// Layer 1: Built-in defaults
	l.applyDefaults(effective)

	// Layer 2: Global config
	if l.globalConfig != nil {
		l.applyGlobalConfig(effective)
	}

	// Layer 3: Active profile
	if l.activeProfile != nil {
		l.applyProfile(effective)
	}

	// Layer 4: Project config
	if l.projectConfig != nil {
		l.applyProjectConfig(effective)
	}

	// Layer 5: Command flags (highest priority)
	l.applyFlags(effective, flags)

	return effective, nil
}

// applyDefaults applies built-in default values.
func (l *ConfigLoader) applyDefaults(cfg *EffectiveConfig) {
	cfg.Parallel = 10
	cfg.CloneProto = "ssh"
	cfg.Sync.Strategy = "pull"
	cfg.Fetch.AllRemotes = true
	cfg.Pull.Rebase = false
	cfg.Pull.FFOnly = false
	cfg.Push.SetUpstream = false

	// Mark all as default source
	cfg.Sources["parallel"] = string(SourceDefault)
	cfg.Sources["cloneProto"] = string(SourceDefault)
	cfg.Sources["sync.strategy"] = string(SourceDefault)
	cfg.Sources["fetch.allRemotes"] = string(SourceDefault)
	cfg.Sources["pull.rebase"] = string(SourceDefault)
	cfg.Sources["pull.ffOnly"] = string(SourceDefault)
	cfg.Sources["push.setUpstream"] = string(SourceDefault)
}

// applyGlobalConfig applies global configuration defaults.
func (l *ConfigLoader) applyGlobalConfig(cfg *EffectiveConfig) {
	if l.globalConfig == nil || l.globalConfig.Defaults == nil {
		return
	}

	defaults := l.globalConfig.Defaults

	// Apply global defaults
	if v, ok := defaults["parallel"].(int); ok {
		cfg.Parallel = v
		cfg.Sources["parallel"] = string(SourceGlobal)
	}
	if v, ok := defaults["cloneProto"].(string); ok {
		cfg.CloneProto = v
		cfg.Sources["cloneProto"] = string(SourceGlobal)
	}
}

// applyProfile applies active profile configuration.
func (l *ConfigLoader) applyProfile(cfg *EffectiveConfig) {
	if l.activeProfile == nil {
		return
	}

	prof := l.activeProfile
	source := fmt.Sprintf("%s:%s", SourceProfile, prof.Name)

	// Provider settings
	if prof.Provider != "" {
		cfg.Provider = prof.Provider
		cfg.Sources["provider"] = source
	}
	if prof.BaseURL != "" {
		cfg.BaseURL = prof.BaseURL
		cfg.Sources["baseURL"] = source
	}
	if prof.Token != "" {
		cfg.Token = prof.Token
		cfg.Sources["token"] = source
	}

	// Clone settings
	if prof.CloneProto != "" {
		cfg.CloneProto = prof.CloneProto
		cfg.Sources["cloneProto"] = source
	}
	if prof.SSHPort > 0 {
		cfg.SSHPort = prof.SSHPort
		cfg.Sources["sshPort"] = source
	}

	// Bulk settings
	if prof.Parallel > 0 {
		cfg.Parallel = prof.Parallel
		cfg.Sources["parallel"] = source
	}
	if prof.IncludeSubgroups {
		cfg.IncludeSubgroups = prof.IncludeSubgroups
		cfg.Sources["includeSubgroups"] = source
	}
	if prof.SubgroupMode != "" {
		cfg.SubgroupMode = prof.SubgroupMode
		cfg.Sources["subgroupMode"] = source
	}

	// Command-specific configs
	if prof.Sync != nil {
		l.applySyncConfig(&cfg.Sync, prof.Sync, source)
	}
	if prof.Branch != nil {
		l.applyBranchConfig(&cfg.Branch, prof.Branch, source)
	}
	if prof.Fetch != nil {
		l.applyFetchConfig(&cfg.Fetch, prof.Fetch, source)
	}
	if prof.Pull != nil {
		l.applyPullConfig(&cfg.Pull, prof.Pull, source)
	}
	if prof.Push != nil {
		l.applyPushConfig(&cfg.Push, prof.Push, source)
	}
}

// applyProjectConfig applies project configuration.
func (l *ConfigLoader) applyProjectConfig(cfg *EffectiveConfig) {
	if l.projectConfig == nil {
		return
	}

	proj := l.projectConfig
	source := string(SourceProject)

	// Command-specific configs
	if proj.Sync != nil {
		l.applySyncConfig(&cfg.Sync, proj.Sync, source)
	}
	if proj.Branch != nil {
		l.applyBranchConfig(&cfg.Branch, proj.Branch, source)
	}
	if proj.Fetch != nil {
		l.applyFetchConfig(&cfg.Fetch, proj.Fetch, source)
	}
	if proj.Pull != nil {
		l.applyPullConfig(&cfg.Pull, proj.Pull, source)
	}
	if proj.Push != nil {
		l.applyPushConfig(&cfg.Push, proj.Push, source)
	}
}

// applyFlags applies command-line flags (highest priority).
func (l *ConfigLoader) applyFlags(cfg *EffectiveConfig, flags map[string]interface{}) {
	if flags == nil {
		return
	}

	source := string(SourceFlag)

	// Provider settings
	if v, ok := flags["provider"].(string); ok && v != "" {
		cfg.Provider = v
		cfg.Sources["provider"] = source
	}
	if v, ok := flags["base-url"].(string); ok && v != "" {
		cfg.BaseURL = v
		cfg.Sources["baseURL"] = source
	}
	if v, ok := flags["token"].(string); ok && v != "" {
		cfg.Token = v
		cfg.Sources["token"] = source
	}

	// Clone settings
	if v, ok := flags["clone-proto"].(string); ok && v != "" {
		cfg.CloneProto = v
		cfg.Sources["cloneProto"] = source
	}
	if v, ok := flags["ssh-port"].(int); ok && v > 0 {
		cfg.SSHPort = v
		cfg.Sources["sshPort"] = source
	}

	// Bulk settings
	if v, ok := flags["parallel"].(int); ok && v > 0 {
		cfg.Parallel = v
		cfg.Sources["parallel"] = source
	}
	if v, ok := flags["include-subgroups"].(bool); ok {
		cfg.IncludeSubgroups = v
		cfg.Sources["includeSubgroups"] = source
	}
	if v, ok := flags["subgroup-mode"].(string); ok && v != "" {
		cfg.SubgroupMode = v
		cfg.Sources["subgroupMode"] = source
	}
}

// applySyncConfig merges sync configuration.
func (l *ConfigLoader) applySyncConfig(dst *SyncConfig, src *SyncConfig, source string) {
	if src == nil {
		return
	}

	if src.Strategy != "" {
		dst.Strategy = src.Strategy
	}
	if src.MaxRetries > 0 {
		dst.MaxRetries = src.MaxRetries
	}
	if src.Timeout != "" {
		dst.Timeout = src.Timeout
	}
}

// applyBranchConfig merges branch configuration.
func (l *ConfigLoader) applyBranchConfig(dst *BranchConfig, src *BranchConfig, source string) {
	if src == nil {
		return
	}

	if len(src.DefaultBranch) > 0 {
		dst.DefaultBranch = src.DefaultBranch
	}
	if len(src.ProtectedBranches) > 0 {
		dst.ProtectedBranches = src.ProtectedBranches
	}
}

// applyFetchConfig merges fetch configuration.
func (l *ConfigLoader) applyFetchConfig(dst *FetchConfig, src *FetchConfig, source string) {
	if src == nil {
		return
	}

	if src.AllRemotes {
		dst.AllRemotes = src.AllRemotes
	}
	if src.Prune {
		dst.Prune = src.Prune
	}
}

// applyPullConfig merges pull configuration.
func (l *ConfigLoader) applyPullConfig(dst *PullConfig, src *PullConfig, source string) {
	if src == nil {
		return
	}

	if src.Rebase {
		dst.Rebase = src.Rebase
	}
	if src.FFOnly {
		dst.FFOnly = src.FFOnly
	}
}

// applyPushConfig merges push configuration.
func (l *ConfigLoader) applyPushConfig(dst *PushConfig, src *PushConfig, source string) {
	if src == nil {
		return
	}

	if src.SetUpstream {
		dst.SetUpstream = src.SetUpstream
	}
}

// GetString retrieves a string value by key from effective config.
func (cfg *EffectiveConfig) GetString(key string) (string, bool) {
	val := reflect.ValueOf(cfg).Elem()
	field := val.FieldByName(key)

	if !field.IsValid() || field.Kind() != reflect.String {
		return "", false
	}

	str := field.String()
	return str, str != ""
}

// GetInt retrieves an integer value by key from effective config.
func (cfg *EffectiveConfig) GetInt(key string) (int, bool) {
	val := reflect.ValueOf(cfg).Elem()
	field := val.FieldByName(key)

	if !field.IsValid() || field.Kind() != reflect.Int {
		return 0, false
	}

	i := int(field.Int())
	return i, i > 0
}

// GetBool retrieves a boolean value by key from effective config.
func (cfg *EffectiveConfig) GetBool(key string) (bool, bool) {
	val := reflect.ValueOf(cfg).Elem()
	field := val.FieldByName(key)

	if !field.IsValid() || field.Kind() != reflect.Bool {
		return false, false
	}

	return field.Bool(), true
}

// GetSource returns the source for a given config key.
func (cfg *EffectiveConfig) GetSource(key string) string {
	if source, ok := cfg.Sources[key]; ok {
		return source
	}
	return string(SourceDefault)
}

// SetActiveProfileInternal sets the active profile without persisting to disk.
// This is used for temporary profile override (e.g., --profile flag).
func (l *ConfigLoader) SetActiveProfileInternal(profile *Profile) {
	l.activeProfile = profile
}
