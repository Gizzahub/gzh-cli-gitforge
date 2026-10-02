// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// ─── Recursive child workspace sync ───────────────────────────────────────────

// recursiveSyncOpts holds settings for recursive child workspace sync.
type recursiveSyncOpts struct {
	MaxDepth int
	Visited  map[string]bool // absolute paths already processed
	Out      io.Writer
	Depth    int
	Strategy string
	Parallel int
	DryRun   bool
}

// syncChildWorkspaces scans succeeded execution results for child workspaces
// containing .gz-git.yaml and recursively syncs them.
func syncChildWorkspaces(ctx context.Context, result reposync.ExecutionResult, opts recursiveSyncOpts) {
	if opts.Depth >= opts.MaxDepth {
		fmt.Fprintf(opts.Out, "\n[recursive] Max depth %d reached, stopping.\n", opts.MaxDepth)
		return
	}

	var childDirs []string
	for _, r := range result.Succeeded {
		targetPath := r.Action.Repo.TargetPath
		if targetPath == "" {
			continue
		}

		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			continue
		}

		if opts.Visited[absPath] {
			continue
		}

		configPath := filepath.Join(absPath, DefaultConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			childDirs = append(childDirs, absPath)
		}
	}

	if len(childDirs) == 0 {
		return
	}

	fmt.Fprintf(opts.Out, "\n[recursive] Found %d child workspace(s) at depth %d:\n", len(childDirs), opts.Depth+1)
	for _, dir := range childDirs {
		fmt.Fprintf(opts.Out, "  → %s\n", dir)
	}

	for _, dir := range childDirs {
		runChildSync(ctx, dir, opts)
	}
}

// runChildSync loads config, plans, and executes sync for a single child workspace.
// Errors are reported as warnings and do not propagate to the parent.
func runChildSync(ctx context.Context, childDir string, opts recursiveSyncOpts) {
	absPath, err := filepath.Abs(childDir)
	if err != nil {
		absPath = childDir
	}
	opts.Visited[absPath] = true

	configPath := filepath.Join(childDir, DefaultConfigFile)
	fmt.Fprintf(opts.Out, "\n[recursive] Syncing child workspace: %s (depth %d)\n", childDir, opts.Depth+1)

	// Load flat config
	loader := FileSpecLoader{}
	cfgData, err := loader.Load(ctx, configPath)
	if err != nil {
		fmt.Fprintf(opts.Out, "  ⚠️  Failed to load config: %v\n", err)
		return
	}

	configFile := filepath.Base(configPath)

	// Plan from flat config
	var allActions []reposync.Action
	if len(cfgData.Plan.Input.Repos) > 0 {
		flatActions := createActionsFromFlatConfig(cfgData.Plan, childDir)
		allActions = append(allActions, flatActions...)
	}

	// Load recursive/hierarchical config
	recursiveCfg, recursiveErr := config.LoadConfigRecursive(childDir, configFile)
	if recursiveErr != nil && !os.IsNotExist(recursiveErr) {
		fmt.Fprintf(opts.Out, "  ⚠️  Config warning: %v\n", recursiveErr)
	}

	if recursiveCfg != nil {
		if err := config.LoadWorkspaces(childDir, recursiveCfg, config.HybridMode); err == nil {
			// This is the post-execution recursion: child syncs always run for
			// real here, so the planners keep writing child configs.
			forgeActions, fErr := planForgeWorkspaces(ctx, recursiveCfg, opts.Out, opts.Strategy, false, false)
			if fErr != nil {
				fmt.Fprintf(opts.Out, "  ⚠️  Failed to plan forge workspaces: %v\n", fErr)
			} else {
				allActions = append(allActions, forgeActions...)
			}

			gitActions, gErr := planGitWorkspaces(ctx, recursiveCfg, childDir, opts.Out, opts.Strategy, false)
			if gErr != nil {
				fmt.Fprintf(opts.Out, "  ⚠️  Failed to plan git workspaces: %v\n", gErr)
			} else {
				allActions = append(allActions, gitActions...)
			}
		}
	}

	if len(allActions) == 0 {
		fmt.Fprintf(opts.Out, "  No repositories found in child workspace.\n")
		return
	}

	fmt.Fprintf(opts.Out, "  → %d repositories to sync\n", len(allActions))

	// Execute sync
	staticPlanner := &precomputedPlanner{actions: allActions}
	executor := reposync.GitExecutor{}
	state := reposync.NewInMemoryStateStore()
	orch := reposync.NewOrchestrator(staticPlanner, executor, state)

	runOpts := cfgData.Run
	if opts.Parallel > 0 {
		runOpts.Parallel = opts.Parallel
	}

	// Use consoleProgress (no ANSI in-place) for child syncs to avoid display conflicts
	progress := &consoleProgress{out: opts.Out}

	execResult, err := orch.Run(ctx, reposync.RunRequest{
		RunOptions: runOpts,
		Progress:   progress,
		State:      state,
	})
	if err != nil {
		fmt.Fprintf(opts.Out, "  ⚠️  Child workspace sync failed: %v\n", err)
		return
	}

	succeeded := len(execResult.Succeeded)
	failed := len(execResult.Failed)
	fmt.Fprintf(opts.Out, "  [recursive] Done: %d succeeded, %d failed\n", succeeded, failed)

	// Continue recursion into deeper children
	childOpts := opts
	childOpts.Depth = opts.Depth + 1
	syncChildWorkspaces(ctx, execResult, childOpts)
}

// scanForChildConfigs shows a dry-run preview of child workspaces that would be
// recursively synced. For existing repos it checks for .gz-git.yaml; for repos
// that will be cloned, it notes them as pending.
func scanForChildConfigs(out io.Writer, actions []reposync.Action, opts recursiveSyncOpts) {
	if opts.Depth+1 >= opts.MaxDepth {
		return
	}

	var existing []string
	var pending []string

	for _, action := range actions {
		targetPath := action.Repo.TargetPath
		if targetPath == "" {
			continue
		}

		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			continue
		}

		if opts.Visited[absPath] {
			continue
		}

		// Check if target directory exists
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			pending = append(pending, targetPath)
			continue
		}

		// Check for child config
		configPath := filepath.Join(absPath, DefaultConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			existing = append(existing, absPath)
		}
	}

	if len(existing) == 0 && len(pending) == 0 {
		return
	}

	fmt.Fprintf(out, "\n[recursive] Child workspace scan (depth %d):\n", opts.Depth+1)
	for _, dir := range existing {
		fmt.Fprintf(out, "  → %s (has %s)\n", dir, DefaultConfigFile)
	}
	for _, dir := range pending {
		fmt.Fprintf(out, "  ? %s (will check after clone)\n", dir)
	}
}
