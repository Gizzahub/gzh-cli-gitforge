// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/wizard"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configLocal bool // For init --local
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration profiles and settings",
	Long: `Manage gz-git configuration including profiles, global settings, and project-specific overrides.

Configuration follows a 5-layer precedence (highest to lowest):
  1. Command flags (e.g., --provider gitlab)
  2. Project config (.gz-git.yaml in current dir or parent)
  3. Active profile (~/.config/gz-git/profiles/{active}.yaml)
  4. Global config (~/.config/gz-git/config.yaml)
  5. Built-in defaults

Examples:
  # Initialize config directory
  gz-git config init

  # Create a profile
  gz-git config profile create work

  # List profiles
  gz-git config profile list

  # Set active profile
  gz-git config profile use work

  # Show effective config
  gz-git config show`,
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

This displays the final configuration values and indicates where each value came from
(flag, project, profile, global, or default).

Examples:
  # Show effective config
  gz-git config show

  # Show project config only
  gz-git config show --local`,
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
	Long:  `Manage configuration profiles for different contexts (work, personal, etc.).`,
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
	Long: `Create a new configuration profile.

Interactive mode: Prompts for common settings
Flag mode: Provide settings via flags

Examples:
  # Interactive creation
  gz-git config profile create work

  # Create with flags
  gz-git config profile create work \
    --provider gitlab \
    --base-url https://gitlab.company.com \
    --clone-proto ssh \
    --ssh-port 2224`,
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

// Hierarchical config subcommands
var configAddChildCmd = &cobra.Command{
	Use:   "add-child <path>",
	Short: "Add a child path to the config",
	Long: `Add a child path to the workspace or workstation configuration.

The child can be either a git repository or another config directory.

Examples:
  # Add a git repository
  gz-git config add-child ~/projects/myrepo --type git

  # Add a workspace with custom config file
  gz-git config add-child ~/mywork --type config --config-file .work-config.yaml

  # Add with inline overrides
  gz-git config add-child ~/opensource --type config --profile opensource --parallel 10

  # Add to workstation config
  gz-git config add-child ~/mydevbox --workstation --type config`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigAddChild,
}

var configListChildrenCmd = &cobra.Command{
	Use:   "list-children",
	Short: "List all child paths in the config",
	Long: `List all child paths defined in the workspace or workstation configuration.

Examples:
  # List children in workspace config
  gz-git config list-children

  # List children in workstation config
  gz-git config list-children --workstation`,
	RunE: runConfigListChildren,
}

var configRemoveChildCmd = &cobra.Command{
	Use:   "remove-child <path>",
	Short: "Remove a child path from the config",
	Long: `Remove a child path from the workspace or workstation configuration.

Examples:
  # Remove from workspace config
  gz-git config remove-child ~/projects/old-repo

  # Remove from workstation config
  gz-git config remove-child ~/old-workspace --workstation`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigRemoveChild,
}

var configHierarchyCmd = &cobra.Command{
	Use:   "hierarchy",
	Short: "Show config hierarchy tree",
	Long: `Show the hierarchical structure of all configuration files.

Starting from workstation config (~/.gz-git-config.yaml), recursively
displays all child configs and git repositories.

Examples:
  # Show full hierarchy
  gz-git config hierarchy

  # Show hierarchy with validation
  gz-git config hierarchy --validate

  # Show compact format
  gz-git config hierarchy --compact`,
	RunE: runConfigHierarchy,
}

// Hierarchical config flags
var (
	childType       string
	childConfigFile string
	childProfile    string
	childParallel   int
	workstationFlag bool
	validateFlag    bool
	compactFlag     bool
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
	configCmd.AddCommand(configAddChildCmd)
	configCmd.AddCommand(configListChildrenCmd)
	configCmd.AddCommand(configRemoveChildCmd)
	configCmd.AddCommand(configHierarchyCmd)

	// Profile subcommands
	configProfileCmd.AddCommand(configProfileListCmd)
	configProfileCmd.AddCommand(configProfileShowCmd)
	configProfileCmd.AddCommand(configProfileCreateCmd)
	configProfileCmd.AddCommand(configProfileUseCmd)
	configProfileCmd.AddCommand(configProfileDeleteCmd)

	// Flags
	configInitCmd.Flags().BoolVar(&configLocal, "local", false, "Initialize project config (.gz-git.yaml)")
	configInitCmd.Flags().BoolVar(&workstationFlag, "workstation", false, "Initialize workstation config (~/.gz-git-config.yaml)")
	configShowCmd.Flags().BoolVar(&configLocal, "local", false, "Show project config only")

	// Hierarchical config command flags
	configAddChildCmd.Flags().StringVar(&childType, "type", "git", "Child type (config, git)")
	configAddChildCmd.Flags().StringVar(&childConfigFile, "config-file", "", "Custom config filename (for type=config)")
	configAddChildCmd.Flags().StringVar(&childProfile, "profile", "", "Override profile for this child")
	configAddChildCmd.Flags().IntVar(&childParallel, "parallel", 0, "Override parallel count for this child")
	configAddChildCmd.Flags().BoolVar(&workstationFlag, "workstation", false, "Add to workstation config instead of workspace")

	configListChildrenCmd.Flags().BoolVar(&workstationFlag, "workstation", false, "List children from workstation config")

	configRemoveChildCmd.Flags().BoolVar(&workstationFlag, "workstation", false, "Remove from workstation config instead of workspace")

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

	if workstationFlag {
		// Initialize workstation config
		cfg := &config.Config{
			Profile: config.DefaultProfileName,
		}

		home, _ := os.UserHomeDir()
		if err := mgr.SaveConfig(home, ".gz-git-config.yaml", cfg); err != nil {
			return fmt.Errorf("failed to create workstation config: %w", err)
		}

		fmt.Printf("Created ~/.gz-git-config.yaml\n")
		return nil
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

// runConfigAddChild adds a child path to config
func runConfigAddChild(cmd *cobra.Command, args []string) error {
	childPath := args[0]

	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Validate child type
	var ct config.ChildType
	switch childType {
	case "config":
		ct = config.ChildTypeConfig
	case "git":
		ct = config.ChildTypeGit
	default:
		return fmt.Errorf("invalid child type '%s': must be 'config' or 'git'", childType)
	}

	// Load the appropriate config
	var cfg *config.Config
	var configPath, configFile string

	if workstationFlag {
		// Load workstation config
		cfg, err = mgr.LoadWorkstationConfig()
		if err != nil {
			return fmt.Errorf("failed to load workstation config: %w", err)
		}
		if cfg == nil {
			// Create new workstation config
			cfg = &config.Config{}
		}
		home, _ := os.UserHomeDir()
		configPath = home
		configFile = ".gz-git-config.yaml"
	} else {
		// Load workspace config
		cfg, err = mgr.LoadWorkspaceConfig()
		if err != nil {
			return fmt.Errorf("failed to load workspace config: %w", err)
		}
		if cfg == nil {
			// Create new workspace config in current directory
			cwd, _ := os.Getwd()
			cfg = &config.Config{}
			configPath = cwd
			configFile = ".gz-git.yaml"
		} else {
			// Find where config is located
			cwd, _ := os.Getwd()
			configPath, err = config.FindConfigRecursive(cwd, ".gz-git.yaml")
			if err != nil {
				configPath, _ = os.Getwd()
			}
			configFile = ".gz-git.yaml"
		}
	}

	// Create child entry
	child := config.ChildEntry{
		Path:       childPath,
		Type:       ct,
		ConfigFile: childConfigFile,
	}

	// Add inline overrides if provided
	if childProfile != "" {
		child.Profile = childProfile
	}
	if childParallel > 0 {
		child.Parallel = childParallel
	}

	// Check if child already exists
	for _, existing := range cfg.Children {
		if existing.Path == childPath {
			return fmt.Errorf("child path '%s' already exists", childPath)
		}
	}

	// Add child
	cfg.Children = append(cfg.Children, child)

	// Save config
	if err := mgr.SaveConfig(configPath, configFile, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Added child '%s' (type=%s) to %s/%s\n", childPath, ct, configPath, configFile)
	return nil
}

// runConfigListChildren lists all child paths
func runConfigListChildren(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	var cfg *config.Config
	var configName string

	if workstationFlag {
		cfg, err = mgr.LoadWorkstationConfig()
		configName = "workstation config (~/.gz-git-config.yaml)"
	} else {
		cfg, err = mgr.LoadWorkspaceConfig()
		configName = "workspace config (.gz-git.yaml)"
	}

	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		fmt.Printf("No %s found\n", configName)
		return nil
	}

	if len(cfg.Children) == 0 {
		fmt.Printf("No children defined in %s\n", configName)
		return nil
	}

	fmt.Printf("Children in %s:\n\n", configName)
	for i, child := range cfg.Children {
		fmt.Printf("%d. %s (type=%s)\n", i+1, child.Path, child.Type)
		if child.ConfigFile != "" {
			fmt.Printf("   config-file: %s\n", child.ConfigFile)
		}
		if child.Profile != "" {
			fmt.Printf("   profile: %s\n", child.Profile)
		}
		if child.Parallel > 0 {
			fmt.Printf("   parallel: %d\n", child.Parallel)
		}
	}

	return nil
}

// runConfigRemoveChild removes a child path from config
func runConfigRemoveChild(cmd *cobra.Command, args []string) error {
	childPath := args[0]

	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Load the appropriate config
	var cfg *config.Config
	var configPath, configFile string

	if workstationFlag {
		cfg, err = mgr.LoadWorkstationConfig()
		if err != nil {
			return fmt.Errorf("failed to load workstation config: %w", err)
		}
		home, _ := os.UserHomeDir()
		configPath = home
		configFile = ".gz-git-config.yaml"
	} else {
		cfg, err = mgr.LoadWorkspaceConfig()
		if err != nil {
			return fmt.Errorf("failed to load workspace config: %w", err)
		}
		cwd, _ := os.Getwd()
		configPath, err = config.FindConfigRecursive(cwd, ".gz-git.yaml")
		if err != nil {
			configPath, _ = os.Getwd()
		}
		configFile = ".gz-git.yaml"
	}

	if cfg == nil {
		return fmt.Errorf("no config found")
	}

	// Find and remove child
	found := false
	newChildren := make([]config.ChildEntry, 0, len(cfg.Children))
	for _, child := range cfg.Children {
		if child.Path == childPath {
			found = true
			continue
		}
		newChildren = append(newChildren, child)
	}

	if !found {
		return fmt.Errorf("child path '%s' not found", childPath)
	}

	cfg.Children = newChildren

	// Save config
	if err := mgr.SaveConfig(configPath, configFile, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Removed child '%s' from %s/%s\n", childPath, configPath, configFile)
	return nil
}

// runConfigHierarchy shows config hierarchy tree
func runConfigHierarchy(cmd *cobra.Command, args []string) error {
	mgr, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Load workstation config
	cfg, err := mgr.LoadWorkstationConfig()
	if err != nil {
		return fmt.Errorf("failed to load workstation config: %w", err)
	}

	if cfg == nil {
		fmt.Println("No workstation config found (~/.gz-git-config.yaml)")
		fmt.Println("Create one with: gz-git config init --workstation")
		return nil
	}

	fmt.Println("Configuration Hierarchy:")
	fmt.Println()

	home, _ := os.UserHomeDir()
	printConfigTree(cfg, home, ".gz-git-config.yaml", 0, validateFlag, compactFlag)

	return nil
}

// printConfigTree recursively prints config hierarchy
func printConfigTree(cfg *config.Config, path string, configFile string, depth int, validate bool, compact bool) {
	indent := strings.Repeat("  ", depth)

	// Print this level
	if depth == 0 {
		fmt.Printf("%s~/.gz-git-config.yaml (workstation)\n", indent)
	} else {
		fmt.Printf("%s%s/%s\n", indent, path, configFile)
	}

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

	// Print children
	for i, child := range cfg.Children {
		childIndent := strings.Repeat("  ", depth+1)

		if child.Type == config.ChildTypeConfig {
			// Recursive: load child config
			childPath, _ := resolveChildPath(path, child.Path)
			childConfigFile := child.ConfigFile
			if childConfigFile == "" {
				childConfigFile = ".gz-git.yaml"
			}

			childCfg, err := config.LoadConfigRecursive(childPath, childConfigFile)
			if err != nil {
				fmt.Printf("%s[%d] %s (type=config) ⚠ failed to load: %v\n", childIndent, i+1, child.Path, err)
			} else {
				fmt.Printf("%s[%d] ", childIndent, i+1)
				printConfigTree(childCfg, childPath, childConfigFile, depth+2, validate, compact)
			}
		} else {
			// Leaf: git repo
			fmt.Printf("%s[%d] %s (type=git)\n", childIndent, i+1, child.Path)
			if !compact && (child.Profile != "" || child.Parallel > 0) {
				if child.Profile != "" {
					fmt.Printf("%s    profile: %s\n", childIndent, child.Profile)
				}
				if child.Parallel > 0 {
					fmt.Printf("%s    parallel: %d\n", childIndent, child.Parallel)
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
