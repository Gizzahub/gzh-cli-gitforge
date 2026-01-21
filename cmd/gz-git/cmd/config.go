// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/wizard"
)

var configLocal bool // For init --local

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration profiles and settings",
	Long: `Manage gz-git configuration using a 5-layer precedence system.

Configuration Layers (highest to lowest priority):
  1. Command flags           (--provider gitlab)
  2. Project config          (.gz-git.yaml in current/parent dir)
  3. Active profile          (~/.config/gz-git/profiles/{active}.yaml)
  4. Global config           (~/.config/gz-git/config.yaml)
  5. Built-in defaults

Quick Start Workflow:
  1. Initialize:      gz-git config init
  2. Create profile:  gz-git config profile create work
  3. Activate:        gz-git config profile use work
  4. Verify:          gz-git config show

Security Best Practices:
  - Use environment variables for sensitive values: token: ${GITLAB_TOKEN}
  - Profiles are created with 0600 permissions (user read/write only)
  - Config directory has 0700 permissions (user access only)
  - Never commit tokens in plain text

Profile Lifecycle:
  - Create profiles for different contexts (work, personal, client projects)
  - Switch contexts instantly with 'profile use'
  - Project configs can reference profiles and override settings
  - All values are shown with their source in 'config show'

Examples:
  # Initialize config directory
  gz-git config init

  # Create a profile
  gz-git config profile create work

  # List profiles
  gz-git config profile list

  # Set active profile
  gz-git config profile use work

  # Show effective config with precedence sources
  gz-git config show`,
	Args: cobra.NoArgs,
}

// configInitCmd initializes configuration
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration directory or project config",
	Long: `Initialize gz-git configuration directory or project-specific config.

Without --local: Creates ~/.config/gz-git/ with default profile
With --local: Creates .gz-git.yaml in current directory

Examples:
  # Initialize global config
  gz-git config init

  # Initialize project config
  gz-git config init --local`,
	RunE: runConfigInit,
}

// configShowCmd shows effective configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective configuration with sources",
	Long: `Show the effective configuration after applying all precedence layers.

Understanding the Output:
  Each configuration value is displayed with its source attribution:
    [flag]    - Value from command-line flag (highest priority)
    [project] - Value from .gz-git.yaml in current/parent directory
    [profile] - Value from active profile (~/.config/gz-git/profiles/{active}.yaml)
    [global]  - Value from global config (~/.config/gz-git/config.yaml)
    [default] - Built-in default value (lowest priority)

Use Cases:
  - Debugging: "Why is this value being used?"
  - Verification: "Is my profile active?"
  - Documentation: "What are the current settings?"
  - Troubleshooting: "Which config layer is overriding my value?"

Examples:
  # Show effective config with source attribution
  gz-git config show

  # Show project config only
  gz-git config show --local

  # Typical output:
  # provider: gitlab [profile:work]
  # baseURL: https://gitlab.company.com [profile:work]
  # cloneProto: ssh [default]
  # parallel: 10 [project]`,
	RunE: runConfigShow,
}

// configGetCmd gets a specific config value
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a specific configuration value",
	Long: `Get a specific configuration value from the effective config.

Examples:
  # Get provider
  gz-git config get provider

  # Get parallel count
  gz-git config get parallel`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

// configSetCmd sets a global default
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a global default value",
	Long: `Set a global default value in ~/.config/gz-git/config.yaml.

Examples:
  # Set default parallel count
  gz-git config set defaults.parallel 10

  # Set default clone protocol
  gz-git config set defaults.cloneProto ssh`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

// Profile subcommands
var configProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configuration profiles",
	Long: `Manage configuration profiles for different contexts (work, personal, etc.).

Profile Concept:
  Profiles are named configuration sets that let you switch contexts instantly.
  Each profile can have different providers, credentials, clone protocols, etc.

Common Use Cases:
  - Work vs Personal GitHub/GitLab accounts
  - Different client projects with separate credentials
  - Self-hosted vs cloud Git forges
  - SSH vs HTTPS clone preferences

Profile Lifecycle:
  create → use → show → (modify) → delete

Examples:
  # Create a profile
  gz-git config profile create work

  # List all profiles
  gz-git config profile list

  # Switch to a profile
  gz-git config profile use work

  # Show profile details
  gz-git config profile show work`,
	Args: cobra.NoArgs,
}

var configProfileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	Long:  `List all available configuration profiles.`,
	RunE:  runConfigProfileList,
}

var configProfileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show profile details",
	Long:  `Show the contents of a specific profile.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigProfileShow,
}

var configProfileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new profile",
	Long: `Create a new configuration profile for different contexts.

What are Profiles?
  Profiles let you switch between different Git forge configurations instantly.
  Common use cases:
    - Work vs Personal accounts
    - Different clients or organizations
    - Self-hosted vs cloud providers
    - SSH vs HTTPS clone preferences

Creation Modes:
  Interactive mode: Prompts for common settings (provider, base URL, token, etc.)
  Flag mode:        Provide all settings via command-line flags

Security Best Practices:
  - Store tokens in environment variables: --token ${GITLAB_TOKEN}
  - Profile files are created with 0600 permissions (user-only read/write)
  - Never commit profiles containing plain-text tokens
  - Use different tokens for different profiles when possible

Examples:
  # Interactive creation (recommended for first-time)
  gz-git config profile create work

  # Create work profile with all settings
  gz-git config profile create work \
    --provider gitlab \
    --base-url https://gitlab.company.com \
    --token ${WORK_GITLAB_TOKEN} \
    --clone-proto ssh \
    --ssh-port 2224

  # Create personal profile (GitHub)
  gz-git config profile create personal \
    --provider github \
    --token ${GITHUB_TOKEN} \
    --clone-proto https

  # After creation, activate the profile
  gz-git config profile use work`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigProfileCreate,
}

var configProfileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set active profile",
	Long:  `Set the active configuration profile.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigProfileUse,
}

var configProfileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Long:  `Delete a configuration profile. The default profile cannot be deleted.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigProfileDelete,
}

var configHierarchyCmd = &cobra.Command{
	Use:   "hierarchy",
	Short: "Show config hierarchy tree",
	Long: `Show the hierarchical structure of all configuration files.

What is Hierarchical Configuration?
  gz-git supports recursive configuration with unlimited nesting:
    ~/.gz-git.yaml (home)
      ├── ~/mydevbox/.gz-git.yaml
      │     ├── gzh-cli/.gz-git.yaml
      │     └── gzh-cli-gitforge/.gz-git.yaml
      └── ~/work/.work-config.yaml  # Custom filename!
            └── client-app/.gz-git.yaml

Precedence Rule: Child Overrides Parent
  - Child configs inherit from parent
  - Child settings override parent settings
  - Discovery modes: explicit (defined), auto (scan), hybrid (both)

Discovery Modes:
  explicit - Only use children defined in config
  auto     - Scan directories, ignore explicit children
  hybrid   - Use defined children, otherwise scan (DEFAULT)

Examples:
  # Show full hierarchy from current directory
  gz-git config hierarchy

  # Show hierarchy with validation (check for errors)
  gz-git config hierarchy --validate

  # Show compact format (less verbose)
  gz-git config hierarchy --compact

  # Typical output:
  # ~/.gz-git.yaml
  # ├── ~/mydevbox/.gz-git.yaml
  # │   ├── gzh-cli/ (type=git)
  # │   └── gzh-cli-gitforge/.gz-git.yaml (type=config)
  # └── ~/work/.work-config.yaml
  #     └── client-app/.gz-git.yaml`,
	RunE: runConfigHierarchy,
}

// Config command flags
var (
	validateFlag bool
	compactFlag  bool
)

// Profile creation flags
var (
	profileProvider     string
	profileBaseURL      string
	profileToken        string
	profileCloneProto   string
	profileSSHPort      int
	profileParallel     int
	profileSubgroups    bool
	profileSubgroupMode string
)

func init() {
	rootCmd.AddCommand(configCmd)

	// Subcommands
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configProfileCmd)
	configCmd.AddCommand(configHierarchyCmd)

	// Profile subcommands
	configProfileCmd.AddCommand(configProfileListCmd)
	configProfileCmd.AddCommand(configProfileShowCmd)
	configProfileCmd.AddCommand(configProfileCreateCmd)
	configProfileCmd.AddCommand(configProfileUseCmd)
	configProfileCmd.AddCommand(configProfileDeleteCmd)

	// Flags
	configInitCmd.Flags().BoolVar(&configLocal, "local", false, "Initialize project config (.gz-git.yaml)")

	configHierarchyCmd.Flags().BoolVar(&validateFlag, "validate", false, "Validate all config files in hierarchy")
	configHierarchyCmd.Flags().BoolVar(&compactFlag, "compact", false, "Show compact output")

	// Profile creation flags
	configProfileCreateCmd.Flags().StringVar(&profileProvider, "provider", "", "Forge provider (github, gitlab, gitea)")
	configProfileCreateCmd.Flags().StringVar(&profileBaseURL, "base-url", "", "API base URL")
	configProfileCreateCmd.Flags().StringVar(&profileToken, "token", "", "API token (use ${ENV_VAR} for env vars)")
	configProfileCreateCmd.Flags().StringVar(&profileCloneProto, "clone-proto", "", "Clone protocol (ssh, https)")
	configProfileCreateCmd.Flags().IntVar(&profileSSHPort, "ssh-port", 0, "SSH port")
	configProfileCreateCmd.Flags().IntVar(&profileParallel, "parallel", 0, "Parallel job count")
	configProfileCreateCmd.Flags().BoolVar(&profileSubgroups, "include-subgroups", false, "Include subgroups (GitLab)")
	configProfileCreateCmd.Flags().StringVar(&profileSubgroupMode, "subgroup-mode", "", "Subgroup mode (flat, nested)")
}

// runConfigInit initializes configuration
func runConfigInit(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	if configLocal {
		// Initialize project config
		projectConfig := &config.ProjectConfig{
			Profile: config.DefaultProfileName,
		}

		if err := mgr.SaveProjectConfig(projectConfig); err != nil {
			return fmt.Errorf("failed to create project config: %w", err)
		}

		cwd, _ := os.Getwd()
		fmt.Printf("Created .gz-git.yaml in %s\n", cwd)
		return nil
	}

	// Initialize global config
	if err := mgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	paths, _ := config.NewPaths()
	fmt.Printf("Initialized configuration in %s\n", paths.ConfigDir)
	fmt.Println("Created default profile")

	return nil
}

// runConfigShow shows effective configuration
func runConfigShow(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	if configLocal {
		// Show project config only
		projectConfig, err := mgr.LoadProjectConfig()
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}

		if projectConfig == nil {
			fmt.Println("No project config found")
			return nil
		}

		data, err := yaml.Marshal(projectConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		fmt.Printf("Project Config (.gz-git.yaml):\n%s", string(data))
		return nil
	}

	// Load and show effective config
	loader, err := config.NewLoader()
	if err != nil {
		return fmt.Errorf("failed to create loader: %w", err)
	}

	if err := loader.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	effective, err := loader.ResolveConfig(nil)
	if err != nil {
		return fmt.Errorf("failed to resolve config: %w", err)
	}

	// Print effective config with sources
	fmt.Println("Effective Configuration:")
	fmt.Println()

	printConfigValue("Provider", effective.Provider, effective.GetSource("provider"))
	printConfigValue("Base URL", effective.BaseURL, effective.GetSource("baseURL"))
	printConfigValue("Token", sanitizeToken(effective.Token), effective.GetSource("token"))
	printConfigValue("Clone Protocol", effective.CloneProto, effective.GetSource("cloneProto"))
	if effective.SSHPort > 0 {
		printConfigValue("SSH Port", fmt.Sprintf("%d", effective.SSHPort), effective.GetSource("sshPort"))
	}
	printConfigValue("Parallel", fmt.Sprintf("%d", effective.Parallel), effective.GetSource("parallel"))
	printConfigValue("Include Subgroups", fmt.Sprintf("%t", effective.IncludeSubgroups), effective.GetSource("includeSubgroups"))
	if effective.SubgroupMode != "" {
		printConfigValue("Subgroup Mode", effective.SubgroupMode, effective.GetSource("subgroupMode"))
	}

	return nil
}

// runConfigGet gets a specific config value
func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	loader, err := config.NewLoader()
	if err != nil {
		return fmt.Errorf("failed to create loader: %w", err)
	}

	if err := loader.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	effective, err := loader.ResolveConfig(nil)
	if err != nil {
		return fmt.Errorf("failed to resolve config: %w", err)
	}

	// Try to get value
	if val, ok := effective.GetString(key); ok {
		fmt.Println(val)
		return nil
	}
	if val, ok := effective.GetInt(key); ok {
		fmt.Println(val)
		return nil
	}
	if val, ok := effective.GetBool(key); ok {
		fmt.Println(val)
		return nil
	}

	return fmt.Errorf("key '%s' not found or has no value", key)
}

// runConfigSet sets a global default
func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Load global config
	globalConfig, err := mgr.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	// Set value in defaults
	if globalConfig.Defaults == nil {
		globalConfig.Defaults = make(map[string]interface{})
	}

	// Parse key (e.g., defaults.parallel)
	parts := strings.Split(key, ".")
	if len(parts) != 2 || parts[0] != "defaults" {
		return fmt.Errorf("key must be in format 'defaults.key' (e.g., defaults.parallel)")
	}

	// Try to parse value as int, otherwise treat as string
	if intVal, err := fmt.Sscanf(value, "%d", new(int)); err == nil && intVal == 1 {
		var i int
		fmt.Sscanf(value, "%d", &i)
		globalConfig.Defaults[parts[1]] = i
	} else {
		globalConfig.Defaults[parts[1]] = value
	}

	// Save global config
	if err := mgr.SaveGlobalConfig(globalConfig); err != nil {
		return fmt.Errorf("failed to save global config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

// runConfigProfileList lists all profiles
func runConfigProfileList(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	profiles, err := mgr.ListProfiles()
	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found. Run 'gz-git config init' to create default profile.")
		return nil
	}

	// Get active profile
	activeProfile, _ := mgr.GetActiveProfile()

	// Sort profiles
	sort.Strings(profiles)

	fmt.Println("Available profiles:")
	for _, name := range profiles {
		marker := " "
		if name == activeProfile {
			marker = "*"
		}
		fmt.Printf("  %s %s", marker, name)
		if name == activeProfile {
			fmt.Print(" (active)")
		}
		fmt.Println()
	}

	return nil
}

// runConfigProfileShow shows profile details
func runConfigProfileShow(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	profile, err := mgr.LoadProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	fmt.Printf("Profile: %s\n\n%s", profileName, string(data))
	return nil
}

// runConfigProfileCreate creates a new profile
func runConfigProfileCreate(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	ctx := cmd.Context()

	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Check if profile already exists
	if mgr.ProfileExists(profileName) {
		return fmt.Errorf("profile '%s' already exists", profileName)
	}

	var profile *config.Profile

	// If flags provided, use them
	if profileProvider != "" || profileBaseURL != "" || profileToken != "" {
		profile = &config.Profile{
			Name:             profileName,
			Provider:         profileProvider,
			BaseURL:          profileBaseURL,
			Token:            profileToken,
			CloneProto:       profileCloneProto,
			SSHPort:          profileSSHPort,
			Parallel:         profileParallel,
			IncludeSubgroups: profileSubgroups,
			SubgroupMode:     profileSubgroupMode,
		}
	} else {
		// Interactive mode with wizard
		w := wizard.NewProfileCreateWizard(profileName)
		profile, err = w.Run(ctx)
		if err != nil {
			return fmt.Errorf("failed to create profile: %w", err)
		}
	}

	// Create profile
	if err := mgr.CreateProfile(profile); err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	fmt.Printf("Created profile '%s'\n", profileName)
	fmt.Printf("Set as active with: gz-git config profile use %s\n", profileName)

	return nil
}

// runConfigProfileUse sets active profile
func runConfigProfileUse(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	if err := mgr.SetActiveProfile(profileName); err != nil {
		return fmt.Errorf("failed to set active profile: %w", err)
	}

	fmt.Printf("Switched to profile '%s'\n", profileName)
	return nil
}

// runConfigProfileDelete deletes a profile
func runConfigProfileDelete(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	if err := mgr.DeleteProfile(profileName); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	fmt.Printf("Deleted profile '%s'\n", profileName)
	return nil
}

// runConfigHierarchy shows config hierarchy tree
func runConfigHierarchy(cmd *cobra.Command, args []string) error {
	// Load config from home directory or current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Try to find config starting from current directory
	configDir, err := config.FindConfigRecursive(cwd, ".gz-git.yaml")
	if err != nil {
		// If not found, try home directory
		home, _ := os.UserHomeDir()
		configDir = home
	}

	cfg, err := config.LoadConfigRecursive(configDir, ".gz-git.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		fmt.Println("No config found (.gz-git.yaml)")
		fmt.Println("Create one with: gz-git config init --local")
		return nil
	}

	fmt.Println("Configuration Hierarchy:")
	fmt.Println()

	printConfigTree(cfg, configDir, ".gz-git.yaml", 0, validateFlag, compactFlag)

	return nil
}

// printConfigTree recursively prints config hierarchy
func printConfigTree(cfg *config.Config, path string, configFile string, depth int, validate bool, compact bool) {
	indent := strings.Repeat("  ", depth)

	// Print this level
	fmt.Printf("%s%s/%s\n", indent, path, configFile)

	// Print config summary if not compact
	if !compact {
		if cfg.Profile != "" {
			fmt.Printf("%s  profile: %s\n", indent, cfg.Profile)
		}
		if cfg.Parallel > 0 {
			fmt.Printf("%s  parallel: %d\n", indent, cfg.Parallel)
		}
	}

	// Validate if requested
	if validate {
		v := config.NewValidator()
		if err := v.ValidateConfig(cfg); err != nil {
			fmt.Printf("%s  ⚠ validation error: %v\n", indent, err)
		} else if !compact {
			fmt.Printf("%s  ✓ valid\n", indent)
		}
	}

	// Print workspaces
	for name, ws := range cfg.Workspaces {
		wsIndent := strings.Repeat("  ", depth+1)
		wsPath, _ := resolveChildPath(path, ws.Path)

		effectiveType := ws.Type.Resolve(ws.Source != nil)

		switch effectiveType {
		case config.WorkspaceTypeConfig:
			// Recursive: load workspace config
			wsConfigFile := ".gz-git.yaml"

			wsCfg, err := config.LoadConfigRecursive(wsPath, wsConfigFile)
			if err != nil {
				fmt.Printf("%s%s (type=config) ⚠ failed to load: %v\n", wsIndent, name, err)
			} else {
				printConfigTree(wsCfg, wsPath, wsConfigFile, depth+1, validate, compact)
			}

		case config.WorkspaceTypeForge:
			// Forge workspace
			fmt.Printf("%s%s (type=forge)\n", wsIndent, name)
			if !compact {
				fmt.Printf("%s  path: %s\n", wsIndent, ws.Path)
				if ws.Source != nil {
					fmt.Printf("%s  source: %s/%s\n", wsIndent, ws.Source.Provider, ws.Source.Org)
				}
				if ws.Profile != "" {
					fmt.Printf("%s  profile: %s\n", wsIndent, ws.Profile)
				}
			}

		case config.WorkspaceTypeGit:
			// Git workspace
			fmt.Printf("%s%s (type=git)\n", wsIndent, name)
			if !compact {
				fmt.Printf("%s  path: %s\n", wsIndent, ws.Path)
				if ws.Profile != "" {
					fmt.Printf("%s  profile: %s\n", wsIndent, ws.Profile)
				}
			}
		}
	}
}

// resolveChildPath resolves child path relative to parent
func resolveChildPath(parentPath, childPath string) (string, error) {
	if strings.HasPrefix(childPath, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, childPath[2:]), nil
	}
	if filepath.IsAbs(childPath) {
		return childPath, nil
	}
	return filepath.Join(parentPath, childPath), nil
}

// printConfigValue prints a config value with its source
func printConfigValue(key, value, source string) {
	if value == "" {
		return
	}
	fmt.Printf("  %-20s %s", key+":", value)
	if source != "" {
		fmt.Printf(" (from %s)", source)
	}
	fmt.Println()
}

// sanitizeToken removes sensitive parts of tokens for display
func sanitizeToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
