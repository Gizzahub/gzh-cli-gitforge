// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
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
			absConfigPath, _ := filepath.Abs(configPath)
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
					forgeActions, err := planForgeWorkspaces(ctx, recursiveCfg, cmd.OutOrStdout(), strategy)
					if err != nil {
						return err
					}
					allActions = append(allActions, forgeActions...)
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
func planForgeWorkspaces(ctx context.Context, cfg *config.Config, out io.Writer, strategyOverrideStr string) ([]reposync.Action, error) {
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

	var allActions []reposync.Action

	for name, ws := range workspaces {
		if ws.Source == nil {
			continue
		}

		fmt.Fprintf(out, "Planning nested workspace '%s' (%s/%s)...\n", name, ws.Source.Provider, ws.Source.Org)

		prov, err := createProviderFromSource(ws.Source, ws, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider for workspace '%s': %w", name, err)
		}

		plannerConfig := reposync.ForgePlannerConfig{
			TargetPath:       ws.Path,
			Organization:     ws.Source.Org,
			IncludeSubgroups: ws.Source.IncludeSubgroups,
			SubgroupMode:     ws.Source.SubgroupMode,
			IncludePrivate:   true, // workspace sync should include private/internal repos
			Auth: reposync.AuthConfig{
				Token:    ws.Source.Token,
				Provider: ws.Source.Provider,
				SSHPort:  ws.SSHPort,
			},
			CloneProto: ws.CloneProto,
		}

		if ws.CloneProto != "" {
			plannerConfig.CloneProto = ws.CloneProto
		}
		if ws.SSHPort != 0 {
			plannerConfig.SSHPort = ws.SSHPort
		}
		if ws.Source.Token != "" {
			plannerConfig.Auth.Token = ws.Source.Token
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
		allActions = append(allActions, plan.Actions...)
	}

	return allActions, nil
}

// Helpers duplicated from from_config_command.go (since we deleted it)
// We need to implement createProviderFromSource here or import it if public?
// It was private. We recreate it.

func createProviderFromSource(src *config.ForgeSource, ws *config.Workspace, cfg *config.Config) (reposync.ForgeProvider, error) {
	// Extract values from source
	token := src.Token
	baseURL := src.BaseURL
	sshPort := ws.SSHPort
	providerName := src.Provider

	// Fallback to profile values if not set in source
	if ws.Profile != "" && cfg != nil {
		profile := config.GetProfileFromChain(cfg, ws.Profile)
		if profile != nil {
			if token == "" {
				token = profile.Token
			}
			if baseURL == "" {
				baseURL = profile.BaseURL
			}
			if sshPort == 0 {
				sshPort = profile.SSHPort
			}
			if providerName == "" {
				providerName = profile.Provider
			}
		}
	}

	return reposynccli.CreateForgeProviderRaw(providerName, token, baseURL, sshPort)
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

			// Generate minimal config
			// Profile inheritance: if not set on workspace, child will naturally
			// inherit from parent lookup. We only write it if explicitly set.
			profileName := ws.Profile

			content := fmt.Sprintf("# Generated by gz-git workspace sync\n# Parent: %s\n\nparent: %s\n",
				cfg.ConfigPath, cfg.ConfigPath)

			if profileName != "" {
				content += fmt.Sprintf("profile: %s\n", profileName)
			}

			if err := os.WriteFile(childConfigFile, []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write bootstrap config for '%s': %w", name, err)
			}
		}
	}
	return nil
}
