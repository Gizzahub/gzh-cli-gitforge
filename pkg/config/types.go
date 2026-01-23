// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package config provides configuration management for gz-git, including
// profile-based settings, global defaults, and project-specific overrides.
//
// Configuration follows a 5-layer precedence system (highest to lowest):
//  1. Command flags (e.g., --provider gitlab)
//  2. Project config (.gz-git.yaml in current dir or parent)
//  3. Active profile (~/.config/gz-git/profiles/{active}.yaml)
//  4. Global config (~/.config/gz-git/config.yaml)
//  5. Built-in defaults
package config

// ================================================================================
// Config File Meta Information
// ================================================================================

// ConfigKind represents the type of configuration file.
type ConfigKind string

const (
	// KindRepositories is for simple flat repository lists (repositories array)
	KindRepositories ConfigKind = "repositories"

	// KindWorkspace is for hierarchical workspace configurations (workspaces map)
	KindWorkspace ConfigKind = "workspace"
)

// IsValid returns true if this is a valid config kind.
func (k ConfigKind) IsValid() bool {
	return k == KindRepositories || k == KindWorkspace
}

// ConfigMeta holds common metadata for all config file types.
// This should be at the top of every config file.
//
// Example:
//
//	version: 1
//	kind: repositories
//	metadata:
//	  name: "my-devbox"
//	  team: "platform"
type ConfigMeta struct {
	// Version is the schema version (currently 1)
	Version int `yaml:"version,omitempty"`

	// Kind specifies the config type: "repositories" or "workspace"
	// If omitted, inferred from content:
	//   - Has "workspaces" or "profiles" key → workspace
	//   - Otherwise → repositories (default)
	Kind ConfigKind `yaml:"kind,omitempty"`

	// Metadata holds optional descriptive information
	Metadata *Metadata `yaml:"metadata,omitempty"`
}

// InferKindFromFilename is deprecated and always returns empty.
// Config kind should be determined by reading the "kind" field inside the file
// or by content detection (presence of "workspaces" or "profiles" keys).
//
// Deprecated: Use content-based detection instead.
func InferKindFromFilename(_ string) ConfigKind {
	return ""
}

// ================================================================================
// Profiles
// ================================================================================

// Profile represents a named configuration profile.
// A profile contains default values for command flags, eliminating
// the need to repeatedly specify the same options.
//
// Example profile file (~/.config/gz-git/profiles/work.yaml):
//
//	name: work
//	provider: gitlab
//	baseURL: https://gitlab.company.com
//	token: ${WORK_GITLAB_TOKEN}
//	cloneProto: ssh
//	sshPort: 2224
//	parallel: 10
//	sync:
//	  strategy: reset
//	  maxRetries: 3
type Profile struct {
	// Name is the profile identifier (e.g., "work", "personal")
	Name string `yaml:"name"`

	// Forge provider settings
	Provider string `yaml:"provider,omitempty"` // github, gitlab, gitea
	BaseURL  string `yaml:"baseURL,omitempty"`  // API endpoint
	Token    string `yaml:"token,omitempty"`    // API token (use ${ENV_VAR})

	// Clone settings
	CloneProto    string `yaml:"cloneProto,omitempty"`    // ssh, https
	SSHPort       int    `yaml:"sshPort,omitempty"`       // Custom SSH port
	SSHKeyPath    string `yaml:"sshKeyPath,omitempty"`    // SSH private key file path (priority)
	SSHKeyContent string `yaml:"sshKeyContent,omitempty"` // SSH private key content (use ${ENV_VAR})

	// Bulk operation settings
	Parallel         int    `yaml:"parallel,omitempty"`         // Parallel job count
	IncludeSubgroups bool   `yaml:"includeSubgroups,omitempty"` // GitLab subgroups
	SubgroupMode     string `yaml:"subgroupMode,omitempty"`     // flat, nested

	// Command-specific overrides
	Sync   *SyncConfig   `yaml:"sync,omitempty"`
	Branch *BranchConfig `yaml:"branch,omitempty"`
	Fetch  *FetchConfig  `yaml:"fetch,omitempty"`
	Pull   *PullConfig   `yaml:"pull,omitempty"`
	Push   *PushConfig   `yaml:"push,omitempty"`
}

// SyncConfig holds sync command defaults.
type SyncConfig struct {
	Strategy   string `yaml:"strategy,omitempty"`   // pull, reset, skip
	MaxRetries int    `yaml:"maxRetries,omitempty"` // Retry count
	Timeout    string `yaml:"timeout,omitempty"`    // Operation timeout
}

// BranchConfig holds branch command defaults.
type BranchConfig struct {
	DefaultBranch     string   `yaml:"defaultBranch,omitempty"`     // main, develop, master
	ProtectedBranches []string `yaml:"protectedBranches,omitempty"` // Branches to protect
}

// FetchConfig holds fetch command defaults.
type FetchConfig struct {
	AllRemotes bool `yaml:"allRemotes,omitempty"` // Fetch all remotes
	Prune      bool `yaml:"prune,omitempty"`      // Prune deleted branches
}

// PullConfig holds pull command defaults.
type PullConfig struct {
	Rebase bool `yaml:"rebase,omitempty"` // Use rebase instead of merge
	FFOnly bool `yaml:"ffOnly,omitempty"` // Fast-forward only
}

// PushConfig holds push command defaults.
type PushConfig struct {
	SetUpstream bool `yaml:"setUpstream,omitempty"` // Auto set upstream
}

// GlobalConfig represents ~/.config/gz-git/config.yaml
//
// Example global config file:
//
//	activeProfile: work
//	defaults:
//	  parallel: 5
//	  cloneProto: ssh
//	  format: default
//	environments:
//	  work:
//	    gitlabToken: ${WORK_GITLAB_TOKEN}
//	  personal:
//	    githubToken: ${PERSONAL_GITHUB_TOKEN}
type GlobalConfig struct {
	// ActiveProfile is the default profile to use
	ActiveProfile string `yaml:"activeProfile,omitempty"`

	// Defaults apply to all profiles unless overridden
	Defaults map[string]interface{} `yaml:"defaults,omitempty"`

	// Environments define named token sets
	Environments map[string]Environment `yaml:"environments,omitempty"`
}

// Environment represents a named set of API tokens.
type Environment struct {
	GitHubToken string `yaml:"githubToken,omitempty"`
	GitLabToken string `yaml:"gitlabToken,omitempty"`
	GiteaToken  string `yaml:"giteaToken,omitempty"`
}

// ProjectConfig represents .gz-git.yaml in a project directory.
// This file is auto-detected by walking up the directory tree.
//
// Example project config file:
//
//	profile: work
//	sync:
//	  strategy: pull
//	  parallel: 3
//	branch:
//	  defaultBranch: main
//	  protectedBranches: [main, develop, release/*]
//	metadata:
//	  team: backend
//	  repository: https://gitlab.company.com/backend/myproject
type ProjectConfig struct {
	// Profile specifies which profile to use for this project
	Profile string `yaml:"profile,omitempty"`

	// Command-specific overrides
	Sync   *SyncConfig   `yaml:"sync,omitempty"`
	Branch *BranchConfig `yaml:"branch,omitempty"`
	Fetch  *FetchConfig  `yaml:"fetch,omitempty"`
	Pull   *PullConfig   `yaml:"pull,omitempty"`
	Push   *PushConfig   `yaml:"push,omitempty"`

	// Metadata is optional project information
	Metadata *ProjectMetadata `yaml:"metadata,omitempty"`
}

// ProjectMetadata holds optional project information.
type ProjectMetadata struct {
	Team       string `yaml:"team,omitempty"`
	Repository string `yaml:"repository,omitempty"`
	Owner      string `yaml:"owner,omitempty"`
}

// ================================================================================
// Recursive Hierarchical Configuration with Workspaces
// ================================================================================

// Config represents a hierarchical configuration that can be nested recursively.
// This is the unified config type used at ALL levels: workstation, workspace,
// project, submodule, etc.
//
// Example usage:
//
//	# ~/.gz-git.yaml (workstation level)
//	profile: polypia
//	parallel: 10
//
//	# Inline profiles (no external file needed!)
//	profiles:
//	  polypia:
//	    provider: gitlab
//	    baseURL: https://gitlab.polypia.net
//	    token: ${GITLAB_POLYPIA_TOKEN}
//	    cloneProto: ssh
//	  github-personal:
//	    provider: github
//	    token: ${GITHUB_TOKEN}
//
//	workspaces:
//	  devbox:
//	    path: ~/mydevbox
//	    source:
//	      provider: gitlab
//	      org: devbox
//	      includeSubgroups: true
//	      subgroupMode: flat
//	    sync:
//	      strategy: pull
//
//	  personal:
//	    path: ~/personal
//	    profile: github-personal
//	    source:
//	      provider: github
//	      org: myusername
//
// Parent config reference example:
//
//	# ~/mydevbox/.gz-git.yaml (workspace level)
//	parent: ~/devenv/workstation/.gz-git.yaml  # Explicit parent reference
//	profile: polypia  # Now found in parent's profiles
//	parallel: 10
type Config struct {
	// === Parent config reference ===

	// Parent specifies an explicit path to a parent config file.
	// When set, the parent config is loaded and merged (child overrides parent).
	// Profile lookup order: current config → parent config → global config.
	// Supports: absolute paths, home-relative (~), relative paths (resolved from current config dir).
	// If not set, falls back to global config (~/.config/gz-git/config.yaml).
	Parent string `yaml:"parent,omitempty"`

	// === This level's settings ===

	// Profile specifies which profile to use at this level
	// Profile lookup order: inline (Profiles map) → parent config → external (~/.config/gz-git/profiles/)
	Profile string `yaml:"profile,omitempty"`

	// === Inline Profiles ===

	// Profiles defines named profiles inline (no external file needed)
	// These take precedence over external profile files
	Profiles map[string]*Profile `yaml:"profiles,omitempty"`

	// Forge provider settings (default for workspaces)
	Provider string `yaml:"provider,omitempty"`
	BaseURL  string `yaml:"baseURL,omitempty"`
	Token    string `yaml:"token,omitempty"`

	// Clone settings
	CloneProto    string `yaml:"cloneProto,omitempty"`
	SSHPort       int    `yaml:"sshPort,omitempty"`
	SSHKeyPath    string `yaml:"sshKeyPath,omitempty"`    // SSH private key file path (priority)
	SSHKeyContent string `yaml:"sshKeyContent,omitempty"` // SSH private key content (use ${ENV_VAR})

	// Bulk operation settings
	Parallel         int    `yaml:"parallel,omitempty"`
	IncludeSubgroups bool   `yaml:"includeSubgroups,omitempty"`
	SubgroupMode     string `yaml:"subgroupMode,omitempty"`
	Format           string `yaml:"format,omitempty"`

	// Command-specific overrides
	Sync   *SyncConfig   `yaml:"sync,omitempty"`
	Branch *BranchConfig `yaml:"branch,omitempty"`
	Fetch  *FetchConfig  `yaml:"fetch,omitempty"`
	Pull   *PullConfig   `yaml:"pull,omitempty"`
	Push   *PushConfig   `yaml:"push,omitempty"`

	// === Workspaces (recursive!) ===

	// Workspaces is a map of named workspace configurations
	// Each workspace can have its own forge source, sync settings, and nested workspaces
	Workspaces map[string]*Workspace `yaml:"workspaces,omitempty"`

	// === Metadata ===

	// Metadata is optional information about this level
	Metadata *Metadata `yaml:"metadata,omitempty"`

	// === Discovery settings ===

	// Discovery controls how workspaces are discovered
	Discovery *DiscoveryConfig `yaml:"discovery,omitempty"`

	// === Internal fields (not serialized) ===

	// ParentConfig is the resolved parent config (nil if no parent or not loaded)
	// This is populated by LoadConfigRecursive when Parent field is set
	ParentConfig *Config `yaml:"-"`

	// ConfigPath is the absolute path to this config file
	// Used for circular reference detection and relative path resolution
	ConfigPath string `yaml:"-"`
}

// Workspace represents a named workspace in the hierarchy.
// Each workspace can sync from a forge source or manage existing git repos.
//
// Example:
//
//	devbox:
//	  path: ./devbox
//	  source:
//	    provider: gitlab
//	    org: devbox
//	    includeSubgroups: true
//	    subgroupMode: flat
//	  sync:
//	    strategy: pull
//	  workspaces:
//	    subproject:
//	      path: ./subproject
//	      type: git
type Workspace struct {
	// Path is the target directory for this workspace
	// Supports: absolute (/foo/bar), relative (./foo), home-relative (~/foo)
	Path string `yaml:"path"`

	// Type specifies what kind of workspace this is
	// Values: "forge" (sync from forge), "git" (single repo), "config" (has nested config)
	// Default: "forge" if Source is set, "git" otherwise
	Type WorkspaceType `yaml:"type,omitempty"`

	// Profile overrides the parent profile for this workspace
	Profile string `yaml:"profile,omitempty"`

	// === Git Repository Settings (for type=git) ===

	// URL is the git clone URL (required for type=git sync)
	// Supports: HTTPS, SSH, git:// protocols
	URL string `yaml:"url,omitempty"`

	// AdditionalRemotes defines extra git remotes to configure after clone
	// Map of remote name to URL (e.g., {"upstream": "https://github.com/original/repo.git"})
	AdditionalRemotes map[string]string `yaml:"additionalRemotes,omitempty"`

	// === Forge Source (for type=forge) ===

	// Source defines the forge to sync from
	Source *ForgeSource `yaml:"source,omitempty"`

	// === Sync settings ===

	Sync     *SyncConfig `yaml:"sync,omitempty"`
	Parallel int         `yaml:"parallel,omitempty"`

	// === Other settings ===

	CloneProto    string        `yaml:"cloneProto,omitempty"`
	SSHPort       int           `yaml:"sshPort,omitempty"`
	SSHKeyPath    string        `yaml:"sshKeyPath,omitempty"`    // SSH private key file path
	SSHKeyContent string        `yaml:"sshKeyContent,omitempty"` // SSH private key content
	Branch        *BranchConfig `yaml:"branch,omitempty"`
	Fetch         *FetchConfig  `yaml:"fetch,omitempty"`
	Pull          *PullConfig   `yaml:"pull,omitempty"`
	Push          *PushConfig   `yaml:"push,omitempty"`

	// === Nested workspaces (recursive!) ===

	// Workspaces allows nested workspace definitions
	Workspaces map[string]*Workspace `yaml:"workspaces,omitempty"`

	// === Metadata ===

	Metadata *Metadata `yaml:"metadata,omitempty"`

	// === Child config generation ===

	// ChildConfigMode controls how child config files are generated during sync.
	// Values: "repositories" (default), "workspaces", "none"
	ChildConfigMode ChildConfigMode `yaml:"childConfigMode,omitempty"`
}

// WorkspaceType represents the type of workspace.
type WorkspaceType string

const (
	// WorkspaceTypeForge indicates the workspace syncs from a forge (GitLab/GitHub/Gitea)
	// This is the default when Source is defined
	WorkspaceTypeForge WorkspaceType = "forge"

	// WorkspaceTypeGit indicates the workspace is a single git repository
	// No forge sync, just manages an existing repo
	WorkspaceTypeGit WorkspaceType = "git"

	// WorkspaceTypeConfig indicates the workspace has a nested config file
	// Loads .gz-git.yaml from the workspace path
	WorkspaceTypeConfig WorkspaceType = "config"
)

// ChildConfigMode controls how child config files are generated during workspace sync.
type ChildConfigMode string

const (
	// ChildConfigModeRepositories generates a flat array format (default).
	// Example: repositories: [{name: repo1, url: ...}, ...]
	ChildConfigModeRepositories ChildConfigMode = "repositories"

	// ChildConfigModeWorkspaces generates a map structure format.
	// Example: workspaces: {repo1: {path: repo1, type: git}, ...}
	ChildConfigModeWorkspaces ChildConfigMode = "workspaces"

	// ChildConfigModeNone creates directory only, no config file.
	// Useful when child config is manually maintained or not needed.
	ChildConfigModeNone ChildConfigMode = "none"
)

// IsValid returns true if this is a valid child config mode.
func (m ChildConfigMode) IsValid() bool {
	return m == "" || m == ChildConfigModeRepositories || m == ChildConfigModeWorkspaces || m == ChildConfigModeNone
}

// Default returns the default mode if empty.
func (m ChildConfigMode) Default() ChildConfigMode {
	if m == "" {
		return ChildConfigModeRepositories
	}
	return m
}

// IsValid returns true if this is a valid workspace type.
func (t WorkspaceType) IsValid() bool {
	return t == WorkspaceTypeForge || t == WorkspaceTypeGit || t == WorkspaceTypeConfig || t == ""
}

// Resolve returns the effective type based on context.
// If type is empty, it's inferred from Source presence.
func (t WorkspaceType) Resolve(hasSource bool) WorkspaceType {
	if t != "" {
		return t
	}
	if hasSource {
		return WorkspaceTypeForge
	}
	return WorkspaceTypeGit
}

// ForgeSource defines a forge (GitLab/GitHub/Gitea) to sync repositories from.
//
// Example:
//
//	source:
//	  provider: gitlab
//	  org: devbox
//	  baseURL: https://gitlab.polypia.net
//	  includeSubgroups: true
//	  subgroupMode: flat
type ForgeSource struct {
	// Provider is the forge type: gitlab, github, gitea
	Provider string `yaml:"provider"`

	// Org is the organization/group to sync from
	Org string `yaml:"org"`

	// BaseURL is the API endpoint (optional, uses default for provider)
	BaseURL string `yaml:"baseURL,omitempty"`

	// Token overrides the profile token (use ${ENV_VAR} for security)
	Token string `yaml:"token,omitempty"`

	// IncludeSubgroups includes subgroups (GitLab only)
	IncludeSubgroups bool `yaml:"includeSubgroups,omitempty"`

	// SubgroupMode controls directory structure: "flat" or "nested"
	SubgroupMode string `yaml:"subgroupMode,omitempty"`
}

// Metadata holds optional information about a config level.
type Metadata struct {
	Name       string `yaml:"name,omitempty"`       // workstation, mydevbox, project-name
	Type       string `yaml:"type,omitempty"`       // development, production, personal
	Owner      string `yaml:"owner,omitempty"`      // archmagece, team-name
	Team       string `yaml:"team,omitempty"`       // backend, frontend
	Repository string `yaml:"repository,omitempty"` // https://...
}

// DiscoveryConfig controls how children are discovered.
type DiscoveryConfig struct {
	// Mode controls the discovery behavior
	// Values: "explicit" (use children only), "auto" (scan directories),
	//         "hybrid" (use children if defined, otherwise scan)
	// Default: "hybrid"
	Mode DiscoveryMode `yaml:"mode,omitempty"`
}

// DiscoveryMode represents the children discovery mode.
type DiscoveryMode string

const (
	// ExplicitMode only uses children explicitly defined in the config
	ExplicitMode DiscoveryMode = "explicit"

	// AutoMode scans directories to find children automatically
	// Ignores explicit children definition
	AutoMode DiscoveryMode = "auto"

	// HybridMode uses children if defined, otherwise scans directories
	// This is the default mode
	HybridMode DiscoveryMode = "hybrid"
)

// IsValid returns true if this is a valid discovery mode.
func (m DiscoveryMode) IsValid() bool {
	return m == ExplicitMode || m == AutoMode || m == HybridMode
}

// Default returns the default discovery mode.
func (m DiscoveryMode) Default() DiscoveryMode {
	if m == "" {
		return HybridMode
	}
	return m
}

// EffectiveConfig represents the final resolved configuration
// after applying all precedence layers.
type EffectiveConfig struct {
	// Forge provider settings
	Provider string
	BaseURL  string
	Token    string

	// Clone settings
	CloneProto    string
	SSHPort       int
	SSHKeyPath    string
	SSHKeyContent string

	// Bulk operation settings
	Parallel         int
	IncludeSubgroups bool
	SubgroupMode     string

	// Command-specific settings
	Sync   SyncConfig
	Branch BranchConfig
	Fetch  FetchConfig
	Pull   PullConfig
	Push   PushConfig

	// Metadata for debugging
	Sources map[string]string // key -> source (e.g., "provider" -> "profile:work")
}

// ConfigSource represents where a config value came from.
type ConfigSource string

const (
	SourceFlag    ConfigSource = "flag"
	SourceProject ConfigSource = "project"
	SourceProfile ConfigSource = "profile"
	SourceGlobal  ConfigSource = "global"
	SourceDefault ConfigSource = "default"
)

// String returns a human-readable source description.
func (s ConfigSource) String() string {
	return string(s)
}
