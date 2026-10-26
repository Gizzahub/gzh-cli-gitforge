// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func init() {
	worktreeCmd.AddCommand(newWorktreeSyncCommand())
}

func newWorktreeSyncCommand() *cobra.Command {
	var dryRun bool
	var include, exclude, format string
	var depth int
	cmd := &cobra.Command{
		SilenceUsage: true,
		Use:          "sync [directory]",
		Short:        "Update worktrees from upstreams and configured parent branches",
		Long: `Update every registered worktree in the current repository, or repositories
under a directory. Upstreams use ff-only. Local parent rules come from
branch.worktreeSync in each repository's effective configuration.

Parents run before children. Conflicting rebases remain in their worktree for
resolution. No automatic stash or push is performed. Dry-run does not fetch;
its plan uses existing remote-tracking refs. Patterns use Go path.Match syntax
(a * does not cross /). Non-leaf rebases and rebasing merge commits are blocked.`,
		Example: "  gz-git worktree sync\n  gz-git worktree sync --dry-run --format json\n  gz-git worktree sync ~/projects --scan-depth 2 --include 'feature/*'",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "default" && format != "json" {
				return fmt.Errorf("format must be default or json")
			}
			if depth < 1 {
				return fmt.Errorf("scan-depth must be at least 1")
			}
			directory, err := validateBulkDirectory(args)
			if err != nil {
				return err
			}
			return runWorktreeSync(cmd, directory, depth, format, branch.WorktreeSyncOptions{DryRun: dryRun, Include: include, Exclude: exclude})
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "plan using existing tracking refs; do not fetch or change branches")
	cmd.Flags().StringVar(&include, "include", "", "include branch glob (required parents are included)")
	cmd.Flags().StringVar(&exclude, "exclude", "", "exclude branch glob (dependent children are blocked)")
	cmd.Flags().StringVarP(&format, "format", "f", "default", "output format: default or json")
	cmd.Flags().IntVarP(&depth, "scan-depth", "d", repository.DefaultLocalScanDepth, "repository scan depth")
	return cmd
}

func runWorktreeSync(cmd *cobra.Command, directory string, depth int, format string, opts branch.WorktreeSyncOptions) error {
	ctx := cmd.Context()
	paths, err := worktreeSyncRepositories(ctx, directory, depth)
	if err != nil {
		return err
	}
	plans := []*branch.WorktreeSyncPlan{}
	seen := make(map[string]bool)
	failed := false
	// Discover and validate all repository configurations before fetching or
	// executing any plan. Each repository supplies its own rules.
	type target struct {
		path    string
		options branch.WorktreeSyncOptions
	}
	targets := []target{}
	for _, repoPath := range paths {
		common, root, err := worktreeSyncLocation(ctx, repoPath)
		if err != nil {
			return err
		}
		if seen[common] {
			continue
		}
		seen[common] = true
		cfg, err := loadWorktreeSyncConfig(root)
		if err != nil {
			return fmt.Errorf("%s: %w", repoPath, err)
		}
		options := opts
		options.Config = cfg
		targets = append(targets, target{path: repoPath, options: options})
	}
	for _, target := range targets {
		plan, err := branch.PlanWorktreeSync(ctx, target.path, target.options)
		if err != nil {
			return fmt.Errorf("%s: %w", target.path, err)
		}
		plans = append(plans, plan)
	}
	for _, plan := range plans {
		if err := branch.ExecuteWorktreeSync(ctx, plan); err != nil {
			failed = true
		}
	}
	if err := printWorktreeSync(cmd, plans, format); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("worktree sync incomplete: inspect blocked worktrees")
	}
	return nil
}

func worktreeSyncRepositories(ctx context.Context, directory string, depth int) ([]string, error) {
	executor := gitcmd.NewExecutor()
	result, err := executor.Run(ctx, directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	if result.ExitCode == 0 {
		return []string{strings.TrimSuffix(result.Stdout, "\n")}, nil
	}
	// A directory outside a repository selects bulk discovery.
	scan, err := repository.NewClient().ScanRepositories(ctx, repository.ScanOptions{Directory: directory, MaxDepth: depth})
	if err != nil {
		return nil, err
	}
	if len(scan.Paths) == 0 {
		return nil, fmt.Errorf("no repositories found in %s", directory)
	}
	return scan.Paths, nil
}

func worktreeSyncLocation(ctx context.Context, repoPath string) (commonDir, configRoot string, err error) {
	executor := gitcmd.NewExecutor()
	common, err := executor.Run(ctx, repoPath, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	if common.ExitCode != 0 {
		return "", "", fmt.Errorf("cannot resolve common Git directory: %s", common.Stderr)
	}
	canonical, err := filepath.EvalSymlinks(strings.TrimSuffix(common.Stdout, "\n"))
	if err != nil {
		return "", "", err
	}
	list, err := executor.Run(ctx, repoPath, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", "", err
	}
	if list.ExitCode != 0 || !strings.HasPrefix(list.Stdout, "worktree ") {
		return "", "", fmt.Errorf("cannot discover primary worktree: %s", list.Stderr)
	}
	first, _, _ := strings.Cut(list.Stdout, "\x00")
	return canonical, strings.TrimPrefix(first, "worktree "), nil
}

func loadWorktreeSyncConfig(root string) (*branch.WorktreeSyncConfig, error) {
	loader, err := config.NewLoader()
	if err != nil {
		return nil, err
	}
	if err := loader.LoadAt(root); err != nil {
		return nil, err
	}
	if profileOverride != "" {
		manager, err := config.NewManager()
		if err != nil {
			return nil, err
		}
		profile, err := manager.LoadProfile(profileOverride)
		if err != nil {
			return nil, err
		}
		loader.SetActiveProfileInternal(profile)
	}
	effective, err := loader.ResolveConfig(nil)
	if err != nil {
		return nil, err
	}
	return effective.Branch.WorktreeSync, nil
}

func printWorktreeSync(cmd *cobra.Command, plans []*branch.WorktreeSyncPlan, format string) error {
	if format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(plans)
	}
	for _, plan := range plans {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), plan.Repository); err != nil {
			return err
		}
		if plan.DryRun {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "  Dry-run: not fetched; using existing tracking refs"); err != nil {
				return err
			}
		}
		for _, result := range plan.Results {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", result.Branch, result.Status, result.Path); err != nil {
				return err
			}
			if result.Implicit {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "    included as a required parent"); err != nil {
					return err
				}
			}
			for _, step := range result.Steps {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    <- %s  %s  %s (%s)\n", step.Source, step.Strategy, step.Action, step.Status); err != nil {
					return err
				}
			}
			if result.Message != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", result.Message); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
