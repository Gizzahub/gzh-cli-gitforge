// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/hooks"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/templates"
)

func (f CommandFactory) newSyncCmd() *cobra.Command {
	var (
		configPath  string
		strategy    string
		parallel    int
		maxRetries  int
		resume      bool
		dryRun      bool
		yes         bool
		interactive bool
		verbose     bool
		stateFile   string
		fullOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Clone/update repositories from config",
		Long: cliutil.QuickStartHelp(`  # Sync from default config (.gz-git.yaml) - auto-proceeds without prompt
  gz-git workspace sync

  # Sync from specific config
  gz-git workspace sync -c myworkspace.yaml

  # Preview without making changes
  gz-git workspace sync --dry-run

  # Ask for confirmation before executing
  gz-git workspace sync --interactive

  # Override strategy for all repos
  gz-git workspace sync --strategy pull

  # Resume interrupted sync
  gz-git workspace sync --resume --state-file state.json

Confirmation Behavior:
  By default, sync auto-proceeds after showing the preview.
  Use --interactive (-i) to be asked for confirmation before executing.
  Use --dry-run to show preview without executing.

Preview Behavior:
  Before executing, sync shows a detailed preview including:
    - Repository-level summary (clone/update/skip counts)
    - File-level changes per repo (added/modified/deleted)
    - Conflict warnings: uncommitted local changes, diverged branches

  Example preview output:
    ═══ Sync Preview (Detailed) ═══
    Total: 3 repositories
      + 1 will be cloned (new)
      ↓ 2 will be updated

    ⚠️  Warnings:
      • api-server: Local uncommitted changes may be overwritten

    Repository Details:
      + my-project (clone)
        → https://github.com/owner/my-project.git
      ↓ api-server (update)
        ⚠️  Uncommitted local changes detected
        Files: ~3 modified, -1 deleted

Config File Structure (Reference):
  strategy: reset # reset|pull|fetch
  parallel: 4
  repositories:
    - name: my-project
      url: https://github.com/owner/my-project.git
      path: ./repos/my-project  # Optional: defaults to name if omitted`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Auto-detect config file if not specified
			if configPath == "" {
				detected, err := detectConfigFile(".")
				if err != nil {
					return fmt.Errorf("no config file specified and auto-detection failed: %w", err)
				}
				configPath = detected
				fmt.Fprintf(cmd.OutOrStdout(), "Using config: %s\n", configPath)
			}

			// Get config directory for path resolution
			absConfigPath, err := filepath.Abs(configPath)
			if err != nil {
				return fmt.Errorf("failed to resolve config path %s: %w", configPath, err)
			}
			configDir := filepath.Dir(absConfigPath)
			configFile := filepath.Base(absConfigPath)

			// Load config to check for "repositories" list (flat config format)
			loader := FileSpecLoader{}
			cfgData, err := loader.Load(ctx, configPath)
			if err != nil {
				return err
			}

			// Plan from repositories list (flat config format)
			var allActions []reposync.Action
			if len(cfgData.Plan.Input.Repos) > 0 {
				// CRITICAL: Filter out the devbox directory itself to prevent self-reset
				flatActions := createActionsFromFlatConfig(cfgData.Plan, configDir)
				allActions = append(allActions, flatActions...)
			}

			// Load Recursive Config (File 4 style)
			// We check if there are nested workspaces or forge sources defined

			recursiveCfg, recursiveErr := config.LoadConfigRecursive(configDir, configFile)
			if recursiveErr != nil {
				if !os.IsNotExist(recursiveErr) {
					fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Config warning: %v\n", recursiveErr)
				}
				recursiveCfg = nil
			}

			if recursiveCfg != nil {
				// Auto-create child configs if missing
				// This handles bootstrapping where child directories don't exist yet
				if err := ensureChildConfigs(cmd.OutOrStdout(), recursiveCfg); err != nil {
					return fmt.Errorf("failed to ensure child configs: %w", err)
				}

				// Discover workspaces (Hybrid mode)
				if err := config.LoadWorkspaces(configDir, recursiveCfg, config.HybridMode); err == nil {
					// Get forge actions
					forgeActions, err := planForgeWorkspaces(ctx, recursiveCfg, cmd.OutOrStdout(), strategy, fullOutput)
					if err != nil {
						return err
					}
					allActions = append(allActions, forgeActions...)

					// Get git workspace actions (type=git with URL)
					gitActions, err := planGitWorkspaces(ctx, recursiveCfg, configDir, cmd.OutOrStdout(), strategy)
					if err != nil {
						return err
					}
					allActions = append(allActions, gitActions...)
				}
			}

			// Execute self-sync first (sync config directory itself)
			if recursiveCfg != nil {
				if err := executeSelfSync(ctx, recursiveCfg, configDir, cmd.OutOrStdout(), dryRun); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Self-sync warning: %v\n", err)
					// Continue with workspace sync even if self-sync fails
				}
			}

			if len(allActions) == 0 {
				fmt.Println("No repositories found to sync.")
				return nil
			}

			out := cmd.OutOrStdout()
			// --dry-run always shows full detail so user can review before deciding
			showDetail := verbose || dryRun
			if showDetail {
				detailedChanges, summary := buildDetailedSyncPreview(ctx, allActions)
				displayDetailedSyncPreview(out, detailedChanges, summary)
			} else {
				// Compact: only show count + conflict warnings
				summary := buildSyncSummary(allActions)
				displayCompactSyncPreview(out, summary, allActions)
			}

			// Determine if we need confirmation
			// Prompt only when: --interactive is set and not --dry-run and not --yes
			// Default behavior (no flags): auto-proceed without prompt
			needsConfirmation := interactive && !dryRun && !yes && isTerminal()

			if needsConfirmation {
				proceed, err := confirmSyncPrompt()
				if err != nil {
					return fmt.Errorf("confirmation prompt failed: %w", err)
				}
				if !proceed {
					fmt.Fprintln(out, "Sync cancelled.")
					return nil
				}
			}

			// In dry-run mode, we already showed the summary, no need to execute
			if dryRun {
				fmt.Fprintln(out, "\n[dry-run] No changes made.")
				return nil
			}

			// Merge options
			runOpts := cfgData.Run
			if cmd.Flags().Changed("parallel") {
				runOpts.Parallel = parallel
			}
			if cmd.Flags().Changed("max-retries") {
				runOpts.MaxRetries = maxRetries
			}
			if cmd.Flags().Changed("resume") {
				runOpts.Resume = resume
			}
			if cmd.Flags().Changed("dry-run") {
				runOpts.DryRun = dryRun
			}
			if runOpts.Resume && stateFile == "" {
				return fmt.Errorf("resume requested but no --state-file provided")
			}

			// If recursive config has parallel setting and flag not set
			if recursiveCfg != nil && recursiveCfg.GetParallel() > 0 && !cmd.Flags().Changed("parallel") {
				runOpts.Parallel = recursiveCfg.GetParallel()
			}

			var progress reposync.ProgressSink
			if isTerminal() {
				progress = newInPlaceProgress(cmd.OutOrStdout(), allActions)
			} else {
				progress = &consoleProgress{out: cmd.OutOrStdout()}
			}

			// Use precomputed planner (same as from-config logic)
			staticPlanner := &precomputedPlanner{actions: allActions}

			// We need an orchestrator, but we can't reuse f.orchestrator() simply because
			// f.orchestrator() might be configured with a different planner (FSPlanner).
			// We need to create a new one or cast.
			// Let's create a new one to be safe and explicit.
			exec := reposync.GitExecutor{}
			var state reposync.StateStore
			if stateFile != "" {
				state = reposync.NewFileStateStore(stateFile)
			} else {
				// We need a state store anyway? NewInMemoryStateStore is good default if nil not allowed
				state = reposync.NewInMemoryStateStore()
			}

			orch := reposync.NewOrchestrator(staticPlanner, exec, state)

			_, err = orch.Run(ctx, reposync.RunRequest{
				RunOptions: runOpts,
				Progress:   progress,
				State:      state,
			})
			if err != nil {
				return fmt.Errorf("workspace sync failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file (auto-detects "+DefaultConfigFile+")")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Strategy override (reset|pull|fetch)")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Parallel workers (overrides config)")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Retry attempts per repo (overrides config)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from previous state")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without making changes")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt (auto-approve, deprecated: now default behavior)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Ask for confirmation before executing (default: auto-proceed)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed repository preview (file changes, conflicts)")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to persist run state for resume")
	cmd.Flags().BoolVar(&fullOutput, "full", false, "Output all fields (name, path) even if redundant")

	return cmd
}

// createActionsFromFlatConfig converts flat config PlanRequest to Actions.
// configDir is the directory containing the config file, used to filter out devbox itself.
func createActionsFromFlatConfig(req reposync.PlanRequest, configDir string) []reposync.Action {
	defaultStrategy := req.Options.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = reposync.StrategyReset
	}

	// Get absolute path of config directory to compare with target paths
	absConfigDir, _ := filepath.Abs(configDir)

	actions := make([]reposync.Action, 0, len(req.Input.Repos))
	for _, repo := range req.Input.Repos {
		// CRITICAL: Skip if target path resolves to the devbox directory itself
		// This prevents workspace sync from resetting the devbox repository
		absTargetPath, _ := filepath.Abs(repo.TargetPath)
		if absTargetPath == absConfigDir {
			// Skip devbox directory itself
			continue
		}

		strategy := repo.Strategy
		if strategy == "" {
			strategy = defaultStrategy
		}

		// Use ActionUpdate - executor handles clone if directory doesn't exist
		actions = append(actions, reposync.Action{
			Repo:      repo,
			Type:      reposync.ActionUpdate,
			Strategy:  strategy,
			Reason:    "workspace config",
			PlannedBy: "config",
		})
	}
	return actions
}

// planForgeWorkspaces generates actions from recursive config workspaces.
func planForgeWorkspaces(ctx context.Context, cfg *config.Config, out io.Writer, strategyOverrideStr string, fullOutput bool) ([]reposync.Action, error) {
	workspaces := config.GetForgeWorkspaces(cfg)
	if len(workspaces) == 0 {
		return nil, nil
	}

	fmt.Fprintf(out, "Found %d recursive workspaces\n", len(workspaces))

	var strategyOverride reposync.Strategy
	if strategyOverrideStr != "" {
		s, err := reposync.ParseStrategy(strategyOverrideStr)
		if err != nil {
			return nil, err
		}
		strategyOverride = s
	}

	// Get config directory for resolving paths
	configDir := filepath.Dir(cfg.ConfigPath)

	var allActions []reposync.Action

	for name, ws := range workspaces {
		if ws.Source == nil {
			continue
		}

		// Default path to workspace name when omitted (compact config convention)
		effectivePath := ws.Path
		if effectivePath == "" {
			effectivePath = name
		}

		// Resolve workspace path
		wsPath := resolveWorkspacePath(effectivePath, configDir)

		// Skip workspaces that point to the config directory itself
		if wsPath == configDir || effectivePath == "." {
			fmt.Fprintf(out, "⚠️  Skipping workspace '%s': path '%s' points to config directory (self-sync not allowed)\n", name, effectivePath)
			continue
		}

		fmt.Fprintf(out, "Planning nested workspace '%s' (%s/%s)...\n", name, ws.Source.Provider, ws.Source.Org)

		// Merge root and workspace hooks
		mergedHooks := hooks.Merge(cfg.Hooks, ws.Hooks)

		// Execute before hooks
		if mergedHooks != nil && len(mergedHooks.Before) > 0 {
			// Ensure parent directory exists for before hooks
			if err := os.MkdirAll(filepath.Dir(wsPath), 0o755); err != nil {
				return nil, fmt.Errorf("failed to create directory for workspace '%s' before hooks: %w", name, err)
			}

			fmt.Fprintf(out, "  → Running before hooks for workspace '%s'...\n", name)
			if err := hooks.Execute(ctx, mergedHooks, "before", filepath.Dir(wsPath), nil); err != nil {
				return nil, fmt.Errorf("before hooks failed for workspace '%s': %w", name, err)
			}
		}

		prov, err := createProviderFromSource(ws.Source, ws, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider for workspace '%s': %w", name, err)
		}

		// Resolve values with profile fallback
		cloneProto := ws.CloneProto
		sshPort := ws.SSHPort
		token := ws.Source.Token

		// Fallback to profile values if not set
		if ws.Profile != "" && cfg != nil {
			profile := config.GetProfileFromChain(cfg, ws.Profile)
			if profile != nil {
				if cloneProto == "" {
					cloneProto = profile.CloneProto
				}
				if sshPort == 0 {
					sshPort = profile.SSHPort
				}
				if token == "" {
					token = profile.Token
				}
			}
		}

		// Fallback to root config values
		if cloneProto == "" && cfg != nil {
			cloneProto = cfg.GetCloneProto()
		}
		if sshPort == 0 && cfg != nil {
			sshPort = cfg.GetSSHPort()
		}

		plannerConfig := reposync.ForgePlannerConfig{
			TargetPath:       ws.Path,
			Organization:     ws.Source.Org,
			IncludeSubgroups: ws.Source.IncludeSubgroups,
			SubgroupMode:     ws.Source.SubgroupMode,
			IncludePrivate:   true, // workspace sync should include private/internal repos
			Auth: reposync.AuthConfig{
				Token:    token,
				Provider: ws.Source.Provider,
				SSHPort:  sshPort,
			},
			CloneProto: cloneProto,
			SSHPort:    sshPort,
		}

		planner := reposync.NewForgePlanner(prov, plannerConfig)
		planReq := reposync.PlanRequest{
			Options: reposync.PlanOptions{
				DefaultStrategy: strategyOverride,
			},
		}

		if planReq.Options.DefaultStrategy == "" {
			if ws.Sync != nil && ws.Sync.Strategy != "" {
				s, err := reposync.ParseStrategy(ws.Sync.Strategy)
				if err == nil {
					planReq.Options.DefaultStrategy = s
				}
			}
		}
		if planReq.Options.DefaultStrategy == "" {
			planReq.Options.DefaultStrategy = reposync.StrategyReset
		}

		plan, err := planner.Plan(ctx, planReq)
		if err != nil {
			return nil, fmt.Errorf("failed to plan workspace '%s': %w", name, err)
		}

		fmt.Fprintf(out, "  → Found %d repositories\n", len(plan.Actions))

		// Handle config: either create symlink (configLink) or write child config
		if ws.ConfigLink != "" {
			// Create symlink instead of writing config
			fmt.Fprintf(out, "  → Creating config symlink: %s → %s\n", filepath.Join(wsPath, ".gz-git.yaml"), ws.ConfigLink)
			if err := config.CreateConfigSymlink(ws.ConfigLink, wsPath, configDir); err != nil {
				return nil, fmt.Errorf("failed to create config symlink for '%s': %w", name, err)
			}
		} else {
			// Write child config with repository list
			if err := writeChildForgeConfig(out, cfg, name, ws, plan.Actions, cloneProto, sshPort, fullOutput); err != nil {
				return nil, fmt.Errorf("failed to write child config for '%s': %w", name, err)
			}
		}

		// Execute after hooks (run after config is set up but before actual sync)
		// Note: Ideally these should run after the sync completes, but the current
		// architecture separates planning from execution. For now, we run them
		// after the workspace is prepared (directory created, config set up).
		if mergedHooks != nil && len(mergedHooks.After) > 0 {
			fmt.Fprintf(out, "  → Running after hooks for workspace '%s'...\n", name)
			if err := hooks.Execute(ctx, mergedHooks, "after", wsPath, nil); err != nil {
				return nil, fmt.Errorf("after hooks failed for workspace '%s': %w", name, err)
			}
		}

		allActions = append(allActions, plan.Actions...)
	}

	return allActions, nil
}

// planGitWorkspaces generates actions from git workspaces (type=git with URL).
func planGitWorkspaces(ctx context.Context, cfg *config.Config, configDir string, out io.Writer, strategyOverrideStr string) ([]reposync.Action, error) {
	workspaces := config.GetGitWorkspaces(cfg)
	if len(workspaces) == 0 {
		return nil, nil
	}

	fmt.Fprintf(out, "Found %d git workspaces\n", len(workspaces))

	var strategyOverride reposync.Strategy
	if strategyOverrideStr != "" {
		s, err := reposync.ParseStrategy(strategyOverrideStr)
		if err != nil {
			return nil, err
		}
		strategyOverride = s
	}

	var allActions []reposync.Action

	for name, ws := range workspaces {
		// Default path to workspace name when omitted (compact config convention)
		effectivePath := ws.Path
		if effectivePath == "" {
			effectivePath = name
		}

		// Resolve workspace path
		wsPath := resolveWorkspacePath(effectivePath, configDir)

		// Skip workspaces that point to the config directory itself
		// This prevents accidental reset of the devbox/orchestrator directory
		if wsPath == configDir || effectivePath == "." {
			fmt.Fprintf(out, "⚠️  Skipping workspace '%s': path '%s' points to config directory (self-sync not allowed)\n", name, effectivePath)
			continue
		}

		fmt.Fprintf(out, "Planning git workspace '%s' (%s)...\n", name, ws.URL)

		// Merge root and workspace hooks
		mergedHooks := hooks.Merge(cfg.Hooks, ws.Hooks)

		// Execute before hooks
		if mergedHooks != nil && len(mergedHooks.Before) > 0 {
			// Ensure parent directory exists for before hooks
			if err := os.MkdirAll(filepath.Dir(wsPath), 0o755); err != nil {
				return nil, fmt.Errorf("failed to create directory for workspace '%s' before hooks: %w", name, err)
			}

			fmt.Fprintf(out, "  → Running before hooks for workspace '%s'...\n", name)
			if err := hooks.Execute(ctx, mergedHooks, "before", filepath.Dir(wsPath), nil); err != nil {
				return nil, fmt.Errorf("before hooks failed for workspace '%s': %w", name, err)
			}
		}

		// Create configLink symlink if specified
		if ws.ConfigLink != "" {
			fmt.Fprintf(out, "  → Creating config symlink: %s → %s\n", filepath.Join(wsPath, ".gz-git.yaml"), ws.ConfigLink)
			if err := config.CreateConfigSymlink(ws.ConfigLink, wsPath, configDir); err != nil {
				return nil, fmt.Errorf("failed to create config symlink for '%s': %w", name, err)
			}
		}

		// Determine strategy
		strategy := strategyOverride
		if strategy == "" {
			if ws.Sync != nil && ws.Sync.Strategy != "" {
				s, err := reposync.ParseStrategy(ws.Sync.Strategy)
				if err == nil {
					strategy = s
				}
			}
		}
		if strategy == "" {
			strategy = reposync.StrategyReset
		}

		// Extract branch from workspace config
		branch := ""
		if ws.Branch != nil && len(ws.Branch.DefaultBranch) > 0 {
			branch = strings.Join(ws.Branch.DefaultBranch, ",")
		}

		// Create action for this git workspace
		action := reposync.Action{
			Repo: reposync.RepoSpec{
				Name:              name,
				CloneURL:          ws.URL,
				AdditionalRemotes: ws.AdditionalRemotes,
				TargetPath:        wsPath,
				Branch:            branch,
			},
			Type:      reposync.ActionUpdate,
			Strategy:  strategy,
			Reason:    "git workspace",
			PlannedBy: "config",
		}

		allActions = append(allActions, action)

		// Execute after hooks
		if mergedHooks != nil && len(mergedHooks.After) > 0 {
			// Ensure workspace directory exists for after hooks
			if err := os.MkdirAll(wsPath, 0o755); err != nil {
				return nil, fmt.Errorf("failed to create workspace directory '%s' for after hooks: %w", name, err)
			}

			fmt.Fprintf(out, "  → Running after hooks for workspace '%s'...\n", name)
			if err := hooks.Execute(ctx, mergedHooks, "after", wsPath, nil); err != nil {
				return nil, fmt.Errorf("after hooks failed for workspace '%s': %w", name, err)
			}
		}
	}

	return allActions, nil
}

// writeChildForgeConfig writes a child config file with the list of repositories.
// It respects the workspace's ChildConfigMode and protects user-maintained files.
func writeChildForgeConfig(out io.Writer, parentCfg *config.Config, wsName string, ws *config.Workspace, actions []reposync.Action, cloneProto string, sshPort int, fullOutput bool) error {
	if len(actions) == 0 {
		return nil
	}

	// Resolve workspace path
	wsPath := ws.Path
	if len(wsPath) > 1 && wsPath[0] == '~' && wsPath[1] == '/' {
		home, err := os.UserHomeDir()
		if err == nil {
			wsPath = filepath.Join(home, wsPath[2:])
		}
	}

	// Get child config mode (workspace > config > default)
	mode := ws.ChildConfigMode
	if mode == "" && parentCfg != nil && parentCfg.ChildConfigMode != "" {
		mode = parentCfg.ChildConfigMode
	}
	mode = mode.Default()

	// If mode is "none": create directory only, no config file
	if mode == config.ChildConfigModeNone {
		if err := os.MkdirAll(wsPath, 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
		fmt.Fprintf(out, "  → Created directory %s (no config, childConfigMode: none)\n", wsPath)
		return nil
	}

	childConfigPath := filepath.Join(wsPath, ".gz-git.yaml")

	// Detect existing config format to protect user-maintained files
	format, err := config.DetectChildConfigFormat(childConfigPath)
	if err != nil {
		return fmt.Errorf("detect config format: %w", err)
	}

	// If user-maintained: warn and skip (don't overwrite)
	if format == config.ChildConfigFormatUserMaintained {
		fmt.Fprintf(out, "  ⚠ Skipping %s (user-maintained, use --force to overwrite)\n", childConfigPath)
		return nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Generate config based on mode
	switch mode {
	case config.ChildConfigModeWorkspaces:
		return writeWorkspacesFormatConfig(out, parentCfg, ws, actions, childConfigPath, fullOutput)
	default: // ChildConfigModeRepositories
		return writeRepositoriesFormatConfig(out, parentCfg, ws, actions, childConfigPath, cloneProto, sshPort, fullOutput)
	}
}

// writeRepositoriesFormatConfig writes a child config in repositories array format.
func writeRepositoriesFormatConfig(out io.Writer, parentCfg *config.Config, ws *config.Workspace, actions []reposync.Action, childConfigPath string, cloneProto string, sshPort int, fullOutput bool) error {
	// Build repository list from actions
	repos := make([]templates.ChildForgeRepoData, 0, len(actions))
	for _, action := range actions {
		// Calculate relative path from workspace root
		relPath := action.Repo.Name
		if action.Repo.TargetPath != "" {
			// Extract just the repo directory name from full path
			relPath = filepath.Base(action.Repo.TargetPath)
		}

		// Omit path if it equals name (compact output)
		// Path defaults to Name when loading config, so redundant paths can be omitted
		pathOutput := relPath
		if !fullOutput && relPath == action.Repo.Name {
			pathOutput = ""
		}

		repos = append(repos, templates.ChildForgeRepoData{
			Name:   action.Repo.Name,
			URL:    action.Repo.CloneURL,
			Path:   pathOutput,
			Branch: action.Repo.Branch,
		})
	}

	// Determine strategy string
	strategyStr := "pull"
	if ws.Sync != nil && ws.Sync.Strategy != "" {
		strategyStr = ws.Sync.Strategy
	}

	// Determine parallel
	parallel := repository.DefaultLocalParallel
	if ws.Parallel > 0 {
		parallel = ws.Parallel
	} else if parentCfg != nil && parentCfg.GetParallel() > 0 {
		parallel = parentCfg.GetParallel()
	}

	data := templates.ChildForgeData{
		Parent:       parentCfg.ConfigPath,
		Profile:      ws.Profile,
		GeneratedAt:  time.Now().Format(time.RFC3339),
		Provider:     ws.Source.Provider,
		Organization: ws.Source.Org,
		Strategy:     strategyStr,
		Parallel:     parallel,
		CloneProto:   cloneProto,
		SSHPort:      sshPort,
		Repositories: repos,
	}

	content, err := templates.Render(templates.RepositoriesChildForge, data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	if err := os.WriteFile(childConfigPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Fprintf(out, "  → Updated %s (%d repositories)\n", childConfigPath, len(repos))
	return nil
}

// writeWorkspacesFormatConfig writes a child config in workspaces map format.
func writeWorkspacesFormatConfig(out io.Writer, parentCfg *config.Config, ws *config.Workspace, actions []reposync.Action, childConfigPath string, fullOutput bool) error {
	// Build workspace entries from actions
	workspaces := make([]templates.ChildWorkspaceEntry, 0, len(actions))
	for _, action := range actions {
		// Calculate relative path from workspace root
		relPath := action.Repo.Name
		if action.Repo.TargetPath != "" {
			relPath = filepath.Base(action.Repo.TargetPath)
		}

		workspaces = append(workspaces, templates.ChildWorkspaceEntry{
			Name: action.Repo.Name,
			Path: relPath,
		})
	}

	data := templates.ChildWorkspacesData{
		Parent:       parentCfg.ConfigPath,
		Profile:      ws.Profile,
		GeneratedAt:  time.Now().Format(time.RFC3339),
		Provider:     ws.Source.Provider,
		Organization: ws.Source.Org,
		Workspaces:   workspaces,
	}

	content, err := templates.Render(templates.WorkspacesChildForge, data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	if err := os.WriteFile(childConfigPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Fprintf(out, "  → Updated %s (%d workspaces, map format)\n", childConfigPath, len(workspaces))
	return nil
}

// createProviderFromSource delegates to reposynccli.CreateProviderFromSource.
// This function is kept for backwards compatibility within this package.
func createProviderFromSource(src *config.ForgeSource, ws *config.Workspace, cfg *config.Config) (reposync.ForgeProvider, error) {
	return reposynccli.CreateProviderFromSource(src, ws, cfg)
}

// Helper types
type precomputedPlanner struct {
	actions []reposync.Action
}

func (p *precomputedPlanner) Plan(_ context.Context, _ reposync.PlanRequest) (reposync.Plan, error) {
	return reposync.Plan{Actions: p.actions}, nil
}

func (p *precomputedPlanner) Describe(_ reposync.PlanRequest) string {
	return fmt.Sprintf("precomputed plan with %d actions", len(p.actions))
}

// consoleProgress implements reposync.ProgressSink for console output.
type consoleProgress struct {
	out io.Writer
}

// OnStart implements reposync.ProgressSink.
func (p *consoleProgress) OnStart(action reposync.Action) {
	fmt.Fprintf(p.out, "→ [%s] %s (%s)\n", action.Type, action.Repo.Name, action.Repo.TargetPath)
}

// OnProgress implements reposync.ProgressSink.
func (p *consoleProgress) OnProgress(action reposync.Action, message string, progress float64) {
	// Suppress redundant retry messages for non-git-repo errors to reduce noise
	// These get summarized in OnComplete as a final error
	if strings.Contains(message, "retrying") {
		return
	}
	fmt.Fprintf(p.out, "   [%s] %s: %s (%.0f%%)\n", action.Type, action.Repo.Name, message, progress*100)
}

// OnComplete implements reposync.ProgressSink.
func (p *consoleProgress) OnComplete(result reposync.ActionResult) {
	if result.Error != nil {
		fmt.Fprintf(p.out, "✗ [%s] %s: %v\n", result.Action.Type, result.Action.Repo.Name, result.Error)
		return
	}
	fmt.Fprintf(p.out, "✓ [%s] %s: %s\n", result.Action.Type, result.Action.Repo.Name, result.Message)
}

// ─── In-place progress display ────────────────────────────────────────────────

// repoStatus tracks the current display state of a single repo row.
type repoStatus struct {
	name   string
	action reposync.ActionType
	state  string // waiting, running, done, error
	msg    string // last status message
}

// inPlaceProgress implements reposync.ProgressSink with ANSI in-place updates.
// All repositories are displayed as a fixed list upfront; each row is updated
// in-place as the sync progresses. This avoids scrolling log output.
//
// Thread-safe: multiple goroutines (parallel sync workers) may call OnStart/
// OnProgress/OnComplete concurrently, so all mutations go through mu.
type inPlaceProgress struct {
	mu         sync.Mutex
	out        io.Writer
	repos      []repoStatus   // ordered list matching initial allActions order
	index      map[string]int // repo name → index in repos
	total      int
	done       int
	errored    int
	errDetails []string // full error messages buffered for post-run display
	// linesRendered is how many rows we printed during the initial render.
	// We use this offset to know how far up to jump when redrawing.
	linesRendered int
}

// newInPlaceProgress creates the progress tracker and immediately prints the
// initial "waiting" list so the user sees all repos from the start.
func newInPlaceProgress(out io.Writer, actions []reposync.Action) *inPlaceProgress {
	p := &inPlaceProgress{
		out:   out,
		repos: make([]repoStatus, 0, len(actions)),
		index: make(map[string]int, len(actions)),
		total: len(actions),
	}
	for _, a := range actions {
		idx := len(p.repos)
		p.repos = append(p.repos, repoStatus{
			name:   a.Repo.Name,
			action: a.Type,
			state:  "waiting",
		})
		p.index[a.Repo.Name] = idx
	}
	// Print header + initial rows
	fmt.Fprintf(out, "\nSyncing %d repositories:\n", len(actions))
	for _, r := range p.repos {
		fmt.Fprintf(out, "  ○ %-30s  waiting\n", r.name)
	}
	p.linesRendered = len(p.repos)
	return p
}

// redraw moves the cursor up linesRendered rows and rewrites every line.
// Must be called with p.mu held.
func (p *inPlaceProgress) redraw() {
	// Move cursor up N lines
	fmt.Fprintf(p.out, "\033[%dA", p.linesRendered)
	for _, r := range p.repos {
		var icon, color, reset string
		reset = "\033[0m"
		switch r.state {
		case "waiting":
			icon = "○"
			color = "\033[2m" // dim
		case "running":
			icon = "●"
			color = "\033[33m" // yellow
		case "done":
			icon = "✓"
			color = "\033[32m" // green
		case "error":
			icon = "✗"
			color = "\033[31m" // red
		}
		// Truncate message to keep lines short
		msg := r.msg
		if len(msg) > 45 {
			msg = msg[:42] + "..."
		}
		// Overwrite line: clear to end of line (\033[K) prevents leftover chars
		fmt.Fprintf(p.out, "  %s%s %-30s  %s%s\033[K\n", color, icon, r.name, msg, reset)
	}
}

// OnStart implements reposync.ProgressSink.
func (p *inPlaceProgress) OnStart(action reposync.Action) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx, ok := p.index[action.Repo.Name]; ok {
		p.repos[idx].state = "running"
		p.repos[idx].msg = string(action.Type) + "..."
	}
	p.redraw()
}

// OnProgress implements reposync.ProgressSink.
func (p *inPlaceProgress) OnProgress(action reposync.Action, message string, progress float64) {
	// Skip retry noise
	if strings.Contains(message, "retrying") {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx, ok := p.index[action.Repo.Name]; ok {
		msg := message
		if progress > 0 {
			msg = fmt.Sprintf("%s (%.0f%%)", message, progress*100)
		}
		p.repos[idx].msg = msg
	}
	p.redraw()
}

// OnComplete implements reposync.ProgressSink.
func (p *inPlaceProgress) OnComplete(result reposync.ActionResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done++
	if idx, ok := p.index[result.Action.Repo.Name]; ok {
		if result.Error != nil {
			p.errored++
			p.repos[idx].state = "error"
			// Short message (first line) for the row display
			errMsg := result.Error.Error()
			firstLine := errMsg
			if nl := strings.Index(errMsg, "\n"); nl > 0 {
				firstLine = errMsg[:nl]
			}
			p.repos[idx].msg = firstLine
			// Save full error for post-run details section
			p.errDetails = append(p.errDetails, fmt.Sprintf("%s: %s", result.Action.Repo.Name, errMsg))
		} else {
			p.repos[idx].state = "done"
			p.repos[idx].msg = result.Message
		}
	}
	p.redraw()

	// After all done, print final summary + error details
	if p.done == p.total {
		succeeded := p.done - p.errored
		if p.errored == 0 {
			fmt.Fprintf(p.out, "\n\033[32m✓ All %d repositories synced successfully.\033[0m\n", p.total)
		} else {
			fmt.Fprintf(p.out, "\n\033[33m⚠  %d succeeded, %d failed out of %d repositories.\033[0m\n",
				succeeded, p.errored, p.total)
			// Print full error details for failed repos
			fmt.Fprintf(p.out, "\n\033[31mErrors:\033[0m\n")
			for _, detail := range p.errDetails {
				// Indent each line of the error message
				for i, line := range strings.Split(detail, "\n") {
					if i == 0 {
						fmt.Fprintf(p.out, "  ✗ %s\n", line)
					} else if strings.TrimSpace(line) != "" {
						fmt.Fprintf(p.out, "    %s\n", line)
					}
				}
			}
		}
	}
}

// ensureChildConfigs checks explicit workspaces and creates .gz-git.yaml if missing.
func ensureChildConfigs(out io.Writer, cfg *config.Config) error {
	if cfg == nil || len(cfg.Workspaces) == 0 {
		return nil
	}

	for name, ws := range cfg.Workspaces {
		// Skip workspaces with forge source - they are handled by planForgeWorkspaces
		// which writes a complete config with repository list
		if ws.Source != nil {
			continue
		}

		// Skip git-type workspaces (leaf repos with URL) - they don't need a child config.
		// These are single git repositories that will be cloned/updated directly.
		// Creating .gz-git.yaml inside them would be incorrect (they are leaf nodes).
		effectiveType := ws.Type.Resolve(ws.Source != nil)
		if effectiveType == config.WorkspaceTypeGit && ws.URL != "" {
			continue
		}

		// Get child config mode (workspace > config > default)
		mode := ws.ChildConfigMode
		if mode == "" && cfg.ChildConfigMode != "" {
			mode = cfg.ChildConfigMode
		}
		// Skip if mode is "none"
		if mode == config.ChildConfigModeNone {
			continue
		}

		// We only care about explicit workspaces defined in this config
		// If it's type: config (explicitly set or inferred)
		// Or if it's meant to be a workspace but directory/config is missing

		// Resolve path relative to config file's directory
		configDir := filepath.Dir(cfg.ConfigPath)
		wsPath := ws.Path
		if !filepath.IsAbs(wsPath) {
			if wsPath == "." {
				wsPath = configDir
			} else if filepath.IsAbs(wsPath) {
				// already absolute
			} else {
				// resolve relative
				// Note: We need robust path resolution here matching config package
				// But simple Join usually works for ./foo
				// Manually resolve ~ since we don't have access to config's internal resolvePath (it's private)
				// Actually filepath.Join doesn't handle ~. We should assume it's already resolved if loaded from Config?
				// Wait, config struct has raw strings. Config package resolves them internally when needed.
				// We need to resolve it here.
				// We need os.UserHomeDir if it starts with ~
				if len(wsPath) > 1 && wsPath[0] == '~' && wsPath[1] == '/' {
					home, err := os.UserHomeDir()
					if err == nil {
						wsPath = filepath.Join(home, wsPath[2:])
					}
				} else {
					wsPath = filepath.Join(configDir, wsPath)
				}
			}
		}

		// Check if .gz-git.yaml exists
		childConfigFile := filepath.Join(wsPath, ".gz-git.yaml")
		if _, err := os.Stat(childConfigFile); os.IsNotExist(err) {
			// Missing! Create it.
			fmt.Fprintf(out, "→ Bootstrapping workspace '%s': creating %s\n", name, childConfigFile)

			if err := os.MkdirAll(wsPath, 0o755); err != nil {
				return fmt.Errorf("failed to create directory for workspace '%s': %w", name, err)
			}

			// Generate bootstrap config using template
			content, err := templates.Render(templates.RepositoriesBootstrap, templates.BootstrapData{
				Parent:  cfg.ConfigPath,
				Profile: ws.Profile,
			})
			if err != nil {
				return fmt.Errorf("failed to render bootstrap template for '%s': %w", name, err)
			}

			if err := os.WriteFile(childConfigFile, []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write bootstrap config for '%s': %w", name, err)
			}
		}
	}
	return nil
}

// resolveWorkspacePath resolves a workspace path that may be:
// - Absolute: /path/to/workspace
// - Home-relative: ~/path/to/workspace
// - Relative: ./path/to/workspace or path/to/workspace
//
// Relative paths are resolved from baseDir.
func resolveWorkspacePath(path, baseDir string) string {
	if path == "" {
		return baseDir
	}

	// Handle home-relative paths
	if len(path) > 1 && path[0] == '~' && path[1] == '/' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}

	// Handle absolute paths
	if filepath.IsAbs(path) {
		return path
	}

	// Handle relative paths
	return filepath.Join(baseDir, path)
}

// executeSelfSync syncs the config directory itself if selfSync is enabled.
// This runs before workspace sync to ensure the devbox is up-to-date.
func executeSelfSync(ctx context.Context, cfg *config.Config, configDir string, out io.Writer, dryRun bool) error {
	if cfg.SelfSync == nil || !cfg.SelfSync.Enabled {
		return nil
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "=== Self-sync (config directory) ===")

	// Check if configDir is a git repository
	gitDir := filepath.Join(configDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Fprintf(out, "⚠️  Skipping self-sync: %s is not a git repository\n", configDir)
		return nil
	}

	// Determine strategy (default to fetch for safety)
	strategy := cfg.SelfSync.Strategy
	if strategy == "" {
		strategy = "fetch"
	}

	// Disallow reset for self-sync
	if strategy == "reset" {
		fmt.Fprintf(out, "⚠️  Strategy 'reset' not allowed for self-sync, falling back to 'fetch'\n")
		strategy = "fetch"
	}

	if dryRun {
		fmt.Fprintf(out, "  [dry-run] Would %s %s\n", strategy, configDir)
		return nil
	}

	// Check if working tree is dirty
	isDirty, err := isWorkingTreeDirty(ctx, configDir)
	if err != nil {
		fmt.Fprintf(out, "⚠️  Failed to check working tree status: %v\n", err)
		// Continue with fetch as safe fallback
		strategy = "fetch"
	}

	if isDirty && strategy == "pull" {
		fmt.Fprintf(out, "  Working tree is dirty, falling back to fetch\n")
		strategy = "fetch"
	}

	// Execute the sync
	switch strategy {
	case "fetch":
		fmt.Fprintf(out, "  Fetching %s...\n", configDir)
		if err := gitFetch(ctx, configDir); err != nil {
			return fmt.Errorf("self-sync fetch failed: %w", err)
		}
		fmt.Fprintf(out, "  ✓ Fetched successfully\n")
	case "pull":
		fmt.Fprintf(out, "  Pulling %s...\n", configDir)
		if err := gitPull(ctx, configDir); err != nil {
			return fmt.Errorf("self-sync pull failed: %w", err)
		}
		fmt.Fprintf(out, "  ✓ Pulled successfully\n")
	case "skip":
		fmt.Fprintf(out, "  Skipping (strategy=skip)\n")
	default:
		fmt.Fprintf(out, "⚠️  Unknown self-sync strategy '%s', skipping\n", strategy)
	}

	fmt.Fprintln(out, "")
	return nil
}

// isWorkingTreeDirty checks if the git working tree has uncommitted changes.
func isWorkingTreeDirty(ctx context.Context, repoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// gitFetch runs git fetch --all in the specified directory.
func gitFetch(ctx context.Context, repoPath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "fetch", "--all", "--prune")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// gitPull runs git pull in the specified directory.
func gitPull(ctx context.Context, repoPath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "pull")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// syncSummary holds counts of different action types for preview.
type syncSummary struct {
	Clone   int
	Update  int
	Skip    int
	Delete  int
	Total   int
	Actions []reposync.Action
}

// RepoChanges holds detailed change information for a single repository.
type RepoChanges struct {
	RepoName  string
	Action    reposync.ActionType
	Path      string
	URL       string
	Files     FileChangeSummary
	Conflicts []ConflictInfo
	Warnings  []string
	Diverged  bool // Tracks if local branch has diverged from remote
}

// FileChangeSummary holds file-level change statistics.
type FileChangeSummary struct {
	Added    []string
	Modified []string
	Deleted  []string
	Total    int
}

// ConflictInfo describes a detected conflict.
type ConflictInfo struct {
	FilePath      string
	ConflictType  string // "local-changes", "diverged-branches", "dirty-worktree"
	LocalChanges  bool   // Has uncommitted local changes
	RemoteChanges bool   // Has incoming remote changes
	Description   string // Human-readable description
}

// ConflictType constants
const (
	ConflictTypeLocalChanges     = "local-changes"
	ConflictTypeDivergedBranches = "diverged-branches"
	ConflictTypeDirtyWorktree    = "dirty-worktree"
)

// buildSyncSummary analyzes actions and counts by type.
func buildSyncSummary(actions []reposync.Action) syncSummary {
	summary := syncSummary{
		Total:   len(actions),
		Actions: actions,
	}
	for _, action := range actions {
		switch action.Type {
		case reposync.ActionClone:
			summary.Clone++
		case reposync.ActionUpdate:
			summary.Update++
		case reposync.ActionSkip:
			summary.Skip++
		case reposync.ActionDelete:
			summary.Delete++
		}
	}
	return summary
}

// analyzeRepoChanges performs detailed analysis of changes for a repository.
// Returns nil if repository doesn't exist yet (new clone) or analysis fails.
func analyzeRepoChanges(ctx context.Context, action reposync.Action) (*RepoChanges, error) {
	changes := &RepoChanges{
		RepoName: action.Repo.Name,
		Action:   action.Type,
		Path:     action.Repo.TargetPath,
		URL:      action.Repo.CloneURL,
		Warnings: []string{},
	}

	// Skip analysis for new clones (directory doesn't exist yet)
	if action.Type == reposync.ActionClone {
		return changes, nil
	}

	// Skip analysis if path doesn't exist
	if _, err := os.Stat(action.Repo.TargetPath); os.IsNotExist(err) {
		return changes, nil
	}

	// For updates, analyze file changes and conflicts
	if action.Type == reposync.ActionUpdate {
		// Check working tree status
		isDirty, err := isWorkingTreeDirty(ctx, action.Repo.TargetPath)
		if err != nil {
			changes.Warnings = append(changes.Warnings, fmt.Sprintf("Failed to check working tree: %v", err))
		} else if isDirty {
			changes.Conflicts = append(changes.Conflicts, ConflictInfo{
				ConflictType: ConflictTypeDirtyWorktree,
				LocalChanges: true,
				Description:  "Uncommitted local changes detected",
			})
			changes.Warnings = append(changes.Warnings, "Local uncommitted changes may be overwritten")
		}

		// Get file diff against remote
		baseBranch := action.Repo.Branch
		if baseBranch == "" {
			baseBranch = "HEAD"
		}

		fileDiff, err := getFileDiff(action.Repo.TargetPath, baseBranch)
		if err != nil {
			changes.Warnings = append(changes.Warnings, fmt.Sprintf("Failed to get diff: %v", err))
		} else {
			changes.Files = fileDiff
		}

		// Check for diverged branches
		diverged, err := checkDivergence(action.Repo.TargetPath, baseBranch)
		if err != nil {
			changes.Warnings = append(changes.Warnings, fmt.Sprintf("Failed to check divergence: %v", err))
		} else if diverged {
			changes.Diverged = true
			changes.Conflicts = append(changes.Conflicts, ConflictInfo{
				ConflictType:  ConflictTypeDivergedBranches,
				LocalChanges:  true,
				RemoteChanges: true,
				Description:   "Local branch has diverged from remote",
			})
			changes.Warnings = append(changes.Warnings, "Branch has diverged from remote (manual merge may be needed)")
		}
	}

	return changes, nil
}

// getFileDiff analyzes file changes between local HEAD and remote.
// Returns empty summary if fetch hasn't been done yet.
func getFileDiff(repoPath, baseBranch string) (FileChangeSummary, error) {
	summary := FileChangeSummary{
		Added:    []string{},
		Modified: []string{},
		Deleted:  []string{},
	}

	// Construct remote tracking branch name
	// Typically: origin/{branch}
	remoteBranch := fmt.Sprintf("origin/%s", baseBranch)
	if baseBranch == "HEAD" {
		remoteBranch = "@{u}" // Upstream tracking branch
	}

	// Run git diff --name-status HEAD..origin/branch
	cmd := exec.Command("git", "diff", "--name-status", fmt.Sprintf("HEAD..%s", remoteBranch))
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		// If remote branch doesn't exist or fetch not done, return empty summary
		return summary, nil
	}

	// Parse output: each line is "STATUS\tFILENAME"
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		status := parts[0]
		filename := parts[1]

		switch status[0] {
		case 'A':
			summary.Added = append(summary.Added, filename)
		case 'M':
			summary.Modified = append(summary.Modified, filename)
		case 'D':
			summary.Deleted = append(summary.Deleted, filename)
		}
	}

	summary.Total = len(summary.Added) + len(summary.Modified) + len(summary.Deleted)
	return summary, nil
}

// checkDivergence checks if local branch has diverged from remote.
// Returns true if local has commits not in remote AND remote has commits not in local.
func checkDivergence(repoPath, baseBranch string) (bool, error) {
	remoteBranch := fmt.Sprintf("origin/%s", baseBranch)
	if baseBranch == "HEAD" {
		remoteBranch = "@{u}"
	}

	// Check commits in local not in remote
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..HEAD", remoteBranch))
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false, nil // Remote branch might not exist
	}
	localAhead := strings.TrimSpace(string(output))

	// Check commits in remote not in local
	cmd = exec.Command("git", "rev-list", "--count", fmt.Sprintf("HEAD..%s", remoteBranch))
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err != nil {
		return false, nil
	}
	remoteBehind := strings.TrimSpace(string(output))

	// Diverged if both have commits
	localCount, _ := strconv.Atoi(localAhead)
	remoteCount, _ := strconv.Atoi(remoteBehind)

	return localCount > 0 && remoteCount > 0, nil
}

// displaySyncSummary prints a preview summary to the output.
func displaySyncSummary(out io.Writer, s syncSummary) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "═══ Sync Preview ═══")
	fmt.Fprintf(out, "Total: %d repositories\n", s.Total)
	fmt.Fprintln(out, "")

	if s.Clone > 0 {
		fmt.Fprintf(out, "  + %d will be cloned (new)\n", s.Clone)
	}
	if s.Update > 0 {
		fmt.Fprintf(out, "  ↓ %d will be updated\n", s.Update)
	}
	if s.Skip > 0 {
		fmt.Fprintf(out, "  ⊘ %d will be skipped\n", s.Skip)
	}
	if s.Delete > 0 {
		fmt.Fprintf(out, "  ✗ %d will be deleted\n", s.Delete)
	}
	fmt.Fprintln(out, "")
}

// displayCompactSyncPreview shows a lightweight one-line summary before sync.
// Used by default (without --verbose). Only highlights repos with warnings.
func displayCompactSyncPreview(out io.Writer, s syncSummary, actions []reposync.Action) {
	// Brief counts line
	var parts []string
	if s.Clone > 0 {
		parts = append(parts, fmt.Sprintf("+%d clone", s.Clone))
	}
	if s.Update > 0 {
		parts = append(parts, fmt.Sprintf("↓%d update", s.Update))
	}
	if s.Skip > 0 {
		parts = append(parts, fmt.Sprintf("⊘%d skip", s.Skip))
	}
	summary := strings.Join(parts, "  ")
	fmt.Fprintf(out, "\nSyncing %d repositories  [%s]\n", s.Total, summary)

	// Show only repos with local changes (dirty worktree) as warnings
	for _, action := range actions {
		path := action.Repo.TargetPath
		if path == "" {
			continue
		}
		// Quick dirty check via git status --porcelain
		cmd := exec.Command("git", "-C", path, "status", "--porcelain")
		if out2, err := cmd.Output(); err == nil && len(strings.TrimSpace(string(out2))) > 0 {
			fmt.Fprintf(out, "  ⚠  %-28s  has local changes (may be overwritten)\n", action.Repo.Name)
		}
	}
}

// buildDetailedSyncPreview analyzes all actions and collects detailed change information.
func buildDetailedSyncPreview(ctx context.Context, actions []reposync.Action) ([]RepoChanges, syncSummary) {
	var detailedChanges []RepoChanges
	summary := buildSyncSummary(actions)

	for _, action := range actions {
		changes, err := analyzeRepoChanges(ctx, action)
		if err != nil {
			// Log error but continue with other repos
			continue
		}
		if changes != nil {
			detailedChanges = append(detailedChanges, *changes)
		}
	}

	return detailedChanges, summary
}

// displayDetailedSyncPreview shows comprehensive preview including file-level changes and conflicts.
func displayDetailedSyncPreview(out io.Writer, changes []RepoChanges, summary syncSummary) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "═══════════════════════════════════════════════")
	fmt.Fprintln(out, "           Sync Preview (Detailed)            ")
	fmt.Fprintln(out, "═══════════════════════════════════════════════")
	fmt.Fprintf(out, "Total: %d repositories\n", summary.Total)
	fmt.Fprintln(out, "")

	// Show high-level summary
	if summary.Clone > 0 {
		fmt.Fprintf(out, "  + %d will be cloned (new)\n", summary.Clone)
	}
	if summary.Update > 0 {
		fmt.Fprintf(out, "  ↓ %d will be updated\n", summary.Update)
	}
	if summary.Skip > 0 {
		fmt.Fprintf(out, "  ⊘ %d will be skipped\n", summary.Skip)
	}
	if summary.Delete > 0 {
		fmt.Fprintf(out, "  ✗ %d will be deleted\n", summary.Delete)
	}
	fmt.Fprintln(out, "")

	// Collect warnings
	var allWarnings []string
	for _, change := range changes {
		if len(change.Warnings) > 0 {
			for _, warning := range change.Warnings {
				allWarnings = append(allWarnings, fmt.Sprintf("%s: %s", change.RepoName, warning))
			}
		}
	}

	// Show warnings first if any
	if len(allWarnings) > 0 {
		fmt.Fprintln(out, "⚠️  Warnings:")
		for _, warning := range allWarnings {
			fmt.Fprintf(out, "  • %s\n", warning)
		}
		fmt.Fprintln(out, "")
	}

	// Show detailed repository changes
	fmt.Fprintln(out, "Repository Details:")
	fmt.Fprintln(out, "")

	for _, change := range changes {
		displayRepoChange(out, change)
	}

	fmt.Fprintln(out, "═══════════════════════════════════════════════")
}

// displayRepoChange shows detailed information for a single repository.
func displayRepoChange(out io.Writer, change RepoChanges) {
	// Action symbol
	var actionSymbol string
	var actionDesc string
	switch change.Action {
	case reposync.ActionClone:
		actionSymbol = "+"
		actionDesc = "clone"
	case reposync.ActionUpdate:
		actionSymbol = "↓"
		actionDesc = "update"
	case reposync.ActionSkip:
		actionSymbol = "⊘"
		actionDesc = "skip - up to date"
	case reposync.ActionDelete:
		actionSymbol = "✗"
		actionDesc = "delete"
	default:
		actionSymbol = "?"
		actionDesc = "unknown"
	}

	// Show repo header
	fmt.Fprintf(out, "  %s %s (%s)\n", actionSymbol, change.RepoName, actionDesc)

	// Show URL for clones
	if change.Action == reposync.ActionClone && change.URL != "" {
		fmt.Fprintf(out, "    → %s\n", change.URL)
	}

	// Show conflicts
	if len(change.Conflicts) > 0 {
		for _, conflict := range change.Conflicts {
			fmt.Fprintf(out, "    ⚠️  %s\n", conflict.Description)
		}
	}

	// Show file changes for updates
	if change.Action == reposync.ActionUpdate && change.Files.Total > 0 {
		fmt.Fprintf(out, "    Files: ")
		var parts []string
		if len(change.Files.Added) > 0 {
			parts = append(parts, fmt.Sprintf("+%d added", len(change.Files.Added)))
		}
		if len(change.Files.Modified) > 0 {
			parts = append(parts, fmt.Sprintf("~%d modified", len(change.Files.Modified)))
		}
		if len(change.Files.Deleted) > 0 {
			parts = append(parts, fmt.Sprintf("-%d deleted", len(change.Files.Deleted)))
		}
		fmt.Fprintf(out, "%s\n", strings.Join(parts, ", "))

		// Show file names (limit to first 5 of each type)
		if len(change.Files.Modified) > 0 {
			displayFileList(out, "modified", change.Files.Modified, 5)
		}
		if len(change.Files.Added) > 0 {
			displayFileList(out, "added", change.Files.Added, 5)
		}
		if len(change.Files.Deleted) > 0 {
			displayFileList(out, "deleted", change.Files.Deleted, 5)
		}
	}

	// Show divergence warning
	if change.Diverged {
		fmt.Fprintln(out, "    ⚠️  Branch has diverged from remote")
	}

	fmt.Fprintln(out, "")
}

// displayFileList shows a list of files with optional truncation.
func displayFileList(out io.Writer, label string, files []string, maxShow int) {
	if len(files) == 0 {
		return
	}

	shown := files
	truncated := false
	if len(files) > maxShow {
		shown = files[:maxShow]
		truncated = true
	}

	for _, file := range shown {
		fmt.Fprintf(out, "      → %s\n", file)
	}

	if truncated {
		fmt.Fprintf(out, "      ... and %d more %s\n", len(files)-maxShow, label)
	}
}

// confirmSyncPrompt displays an interactive confirmation prompt.
// Returns true if user wants to proceed, false otherwise.
func confirmSyncPrompt() (bool, error) {
	var proceed bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Proceed with sync?").
				Affirmative("Yes").
				Negative("No").
				Value(&proceed),
		),
	)

	err := form.Run()
	if err != nil {
		return false, err
	}
	return proceed, nil
}

// isTerminal checks if stdout is a terminal (not redirected/piped).
// Returns false in CI/non-interactive environments.
func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}
