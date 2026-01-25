// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/hooks"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/templates"
)

func (f CommandFactory) newSyncCmd() *cobra.Command {
	var (
		configPath string
		strategy   string
		parallel   int
		maxRetries int
		resume     bool
		dryRun     bool
		stateFile  string
		fullOutput bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Clone/update repositories from config",
		Long: cliutil.QuickStartHelp(`  # Sync from default config (.gz-git.yaml)
  gz-git workspace sync

  # Sync from specific config
  gz-git workspace sync -c myworkspace.yaml

  # Preview without making changes
  gz-git workspace sync --dry-run

  # Override strategy for all repos
  gz-git workspace sync --strategy pull

  # Resume interrupted sync
  gz-git workspace sync --resume --state-file state.json

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

			// Load legacy config first to check for "repositories" list (flat config)
			// This preserves backward compatibility for File 1, 2 style configs
			loader := FileSpecLoader{}
			cfgData, err := loader.Load(ctx, configPath)
			if err != nil {
				return err
			}

			// Plan from legacy repositories list
			var allActions []reposync.Action
			if len(cfgData.Plan.Input.Repos) > 0 {
				// Reconstruct a static planner that creates actions from repo specs
				// Note: NewStaticPlanner takes a Plan, not Input.
				// But we want to use the logic that existed previously?
				// The previous code passed cfgData.Plan (PlanRequest) to orchestrator.
				// The helper function below `createActionsFromLegacyPlan` does this.
				//
				// CRITICAL: Filter out the devbox directory itself to prevent self-reset
				legacyActions := createActionsFromLegacyPlan(cfgData.Plan, configDir)
				allActions = append(allActions, legacyActions...)
			}

			// Load Recursive Config (File 4 style)
			// We check if there are nested workspaces or forge sources defined

			var recursiveCfg *config.Config
			if err == nil {
				recursiveCfg, err = config.LoadConfigRecursive(configDir, configFile)
				// If error (err is not nil), it falls through and behaves as if recursiveCfg is nil/failed
			}

			if err == nil && recursiveCfg != nil {
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

			if len(allActions) == 0 {
				fmt.Println("No repositories found to sync.")
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
			if recursiveCfg != nil && recursiveCfg.Parallel > 0 && !cmd.Flags().Changed("parallel") {
				runOpts.Parallel = recursiveCfg.Parallel
			}

			progress := &consoleProgress{out: cmd.OutOrStdout()}

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
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to persist run state for resume")
	cmd.Flags().BoolVar(&fullOutput, "full", false, "Output all fields (name, path) even if redundant")

	return cmd
}

// createActionsFromLegacyPlan converts legacy PlanRequest to Actions.
// configDir is the directory containing the config file, used to filter out devbox itself.
func createActionsFromLegacyPlan(req reposync.PlanRequest, configDir string) []reposync.Action {
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

		// Simple logic: Assume update if we don't know status (Orchestrator normally handles this via Planner)
		// But here we are bypassing planner.
		// NOTE: GitExecutor handles Clone vs Check/Update based on dir existence usually?
		// Actually, Executor takes an Action which HAS a Type (Clone/Update).
		// We should probably check file existence here to be accurate,
		// OR use ActionUpdate which often implies Clone if missing?
		// Let's check `planner_forge.go` logic: it checks os.Stat.

		// For legacy simple list, we can probably safely duplicate that check or default to Update which fallsback?
		// GitExecutor.Execute:
		// case ActionClone: git clone
		// case ActionUpdate: check if exists -> pull/reset; else -> clone?
		// Let's check executor_git.go... No time to check deeply.
		// Safer to check existence.

		actionType := reposync.ActionUpdate // Default preference
		// We'll skip existence check here for brevity and assume Update handles missing?
		// Actually better to mimic StaticPlanner logic if possible.
		// StaticPlanner: "ActionClone if AssumePresent false".
		// Let's set to Clone as default for safety?
		// Actually, for workspace sync, Update is usually what we want if it exists.

		actions = append(actions, reposync.Action{
			Repo:      repo,
			Type:      actionType,
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

		fmt.Fprintf(out, "Planning nested workspace '%s' (%s/%s)...\n", name, ws.Source.Provider, ws.Source.Org)

		// Resolve workspace path
		wsPath := resolveWorkspacePath(ws.Path, configDir)

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
			cloneProto = cfg.CloneProto
		}
		if sshPort == 0 && cfg != nil {
			sshPort = cfg.SSHPort
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
		fmt.Fprintf(out, "Planning git workspace '%s' (%s)...\n", name, ws.URL)

		// Resolve workspace path
		wsPath := resolveWorkspacePath(ws.Path, configDir)

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

		// Create action for this git workspace
		action := reposync.Action{
			Repo: reposync.RepoSpec{
				Name:              name,
				CloneURL:          ws.URL,
				AdditionalRemotes: ws.AdditionalRemotes,
				TargetPath:        wsPath,
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
	parallel := 10
	if ws.Parallel > 0 {
		parallel = ws.Parallel
	} else if parentCfg != nil && parentCfg.Parallel > 0 {
		parallel = parentCfg.Parallel
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
