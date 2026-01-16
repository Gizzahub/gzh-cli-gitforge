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
	CloneProto string `yaml:"cloneProto,omitempty"` // ssh, https
	SSHPort    int    `yaml:"sshPort,omitempty"`    // Custom SSH port

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
// Recursive Hierarchical Configuration (NEW!)
// ================================================================================

// Config represents a hierarchical configuration that can be nested recursively.
// This is the unified config type used at ALL levels: workstation, workspace,
// project, submodule, etc.
//
// Example usage (all levels use the same structure):
//
//	# ~/.gz-git-config.yaml (workstation level)
//	parallel: 5
//	cloneProto: ssh
//	children:
//	  - path: ~/mydevbox
//	    type: config
//	    profile: opensource
//	  - path: ~/mywork
//	    type: config
//	    configFile: .work-config.yaml
//	    profile: work
//
//	# ~/mydevbox/.gz-git.yaml (workspace level - same structure!)
//	profile: opensource
//	sync:
//	  strategy: reset
//	  parallel: 10
//	children:
//	  - path: gzh-cli
//	    type: git
//	  - path: gzh-cli-gitforge
//	    type: config
//	    sync:
//	      strategy: pull
//
//	# ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml (project level - same structure!)
//	sync:
//	  strategy: pull
//	children:
//	  - path: vendor/lib
//	    type: git
//	    sync:
//	      strategy: skip
type Config struct {
	// === This level's settings ===

	// Profile specifies which profile to use at this level
	Profile string `yaml:"profile,omitempty"`

	// Forge provider settings (workstation/workspace level)
	Provider string `yaml:"provider,omitempty"`
	BaseURL  string `yaml:"baseURL,omitempty"`
	Token    string `yaml:"token,omitempty"`

	// Clone settings
	CloneProto string `yaml:"cloneProto,omitempty"`
	SSHPort    int    `yaml:"sshPort,omitempty"`

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

	// === Children (recursive!) ===

	// Children is the explicit list of child paths (workspaces, projects, git repos)
	// Each child can have its own config file (type: config) or be a plain git repo (type: git)
	Children []ChildEntry `yaml:"children,omitempty"`

	// === Metadata ===

	// Metadata is optional information about this level
	Metadata *Metadata `yaml:"metadata,omitempty"`

	// === Discovery settings ===

	// Discovery controls how children are discovered
	Discovery *DiscoveryConfig `yaml:"discovery,omitempty"`
}

// ChildEntry represents a child path in the hierarchy.
// Each child can be either:
//   - A directory with a config file (type: config) - enables recursive nesting
//   - A plain git repository (type: git) - leaf node
type ChildEntry struct {
	// Path is the relative or absolute path to the child
	// Supports: absolute (/foo/bar), relative (./foo), home-relative (~/foo)
	Path string `yaml:"path"`

	// Type specifies what kind of child this is
	// Values: "config" (has config file, enables recursion), "git" (plain repo)
	Type ChildType `yaml:"type"`

	// ConfigFile specifies the config file name (optional)
	// Only used when Type == "config"
	// Default: ".gz-git.yaml"
	// Example custom: ".work-config.yaml"
	ConfigFile string `yaml:"configFile,omitempty"`

	// === Inline overrides (optional) ===
	// These override the child's config file settings

	Profile  string        `yaml:"profile,omitempty"`
	Parallel int           `yaml:"parallel,omitempty"`
	Sync     *SyncConfig   `yaml:"sync,omitempty"`
	Branch   *BranchConfig `yaml:"branch,omitempty"`
	Fetch    *FetchConfig  `yaml:"fetch,omitempty"`
	Pull     *PullConfig   `yaml:"pull,omitempty"`
	Push     *PushConfig   `yaml:"push,omitempty"`
}

// ChildType represents the type of child entry.
type ChildType string

const (
	// ChildTypeConfig indicates the child has a config file (enables recursion)
	// The config file will be loaded recursively
	ChildTypeConfig ChildType = "config"

	// ChildTypeGit indicates the child is a plain git repository (no config)
	// This is a leaf node in the hierarchy
	ChildTypeGit ChildType = "git"
)

// DefaultConfigFile returns the default config file name for this child type.
func (t ChildType) DefaultConfigFile() string {
	if t == ChildTypeConfig {
		return ".gz-git.yaml"
	}
	return "" // Git repos don't have config files
}

// IsValid returns true if this is a valid child type.
func (t ChildType) IsValid() bool {
	return t == ChildTypeConfig || t == ChildTypeGit
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
	CloneProto string
	SSHPort    int

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
