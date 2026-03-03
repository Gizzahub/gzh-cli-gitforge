// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

// NewQuickSyncCmd returns the top-level `gz-git sync` (alias: `gz-git s`) command.
//
// Design principles:
//   - 무적(invincible): works from zero – runs init if config is missing
//   - Full parity with `gz-git workspace sync` flags
//   - --dry-run applies to auto-init too (no files written)
//   - Clear, actionable error messages when repos are not found
//   - Optional --check runs `workspace status` after sync
func (f CommandFactory) NewQuickSyncCmd() *cobra.Command {
	var (
		// ── Sync flags (mirrors workspace sync exactly) ─────────────────────
		configPath     string
		strategy       string
		parallel       int
		maxRetries     int
		resume         bool
		dryRun         bool
		yes            bool
		interactive    bool
		verbose        bool
		stateFile      string
		fullOutput     bool
		recursive      bool
		recursiveDepth int
		format         string

		// ── Auto-init flags (active only when .gz-git.yaml is absent) ────────
		initDepth int
		initKind  string
		initForce bool
		noReview  bool // skip review prompt after auto-init

		// ── Extra features ───────────────────────────────────────────────────
		check         bool // run `workspace status` after sync completes
		pushAfterSync bool // push after successful sync
	)

	cmd := &cobra.Command{
		Use:     "sync [path]",
		Aliases: []string{"s"},
		Short:   "Smart sync: auto-init if needed, then sync workspace",
		Long: cliutil.QuickStartHelp(`  # Sync current directory (auto-inits .gz-git.yaml if absent)
  gz-git sync

  # Short alias
  gz-git s

  # Sync a specific directory
  gz-git sync ~/mydevbox

  # Preview without any changes (auto-init is also skipped)
  gz-git sync --dry-run

  # Use a specific config (skips auto-init entirely)
  gz-git sync -c myworkspace.yaml

  # Ask for confirmation before executing
  gz-git sync --interactive

  # Sync and immediately run health-check afterwards
  gz-git sync --check

  # Override strategy for all repos
  gz-git sync --strategy pull

  # Verbose preview with file-level changes and conflict warnings
  gz-git sync --verbose

Auto-Init Behavior:
  When .gz-git.yaml is absent, gz-git sync runs 'workspace init <path>'
  to scan existing git repos and write the config, then syncs immediately.

  Flags that control auto-init (only active when config is absent):
    --init-depth <n>   Scan depth (default: 2)
    --init-kind        Config kind: workspace|repositories (default: workspace)
    --init-force       Overwrite existing config during auto-init
    --no-review        Skip confirmation after auto-init, sync immediately

  To suppress auto-init altogether (e.g. in CI), always pass -c:
    gz-git sync -c /explicit/config.yaml`),

		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			// ── 1. Determine working directory ────────────────────────────────
			workDir := "."
			if len(args) > 0 {
				workDir = args[0]
			}
			absWorkDir, err := filepath.Abs(workDir)
			if err != nil {
				return fmt.Errorf("invalid path %q: %w", workDir, err)
			}
			if stat, err := os.Stat(absWorkDir); err != nil || !stat.IsDir() {
				return fmt.Errorf("path does not exist or is not a directory: %s", absWorkDir)
			}

			// ── 2. Determine effective config path ────────────────────────────
			effectiveConfig := configPath
			if effectiveConfig == "" {
				detected, detectErr := detectConfigFile(workDir)
				if detectErr != nil {
					// Config not found → auto-init branch
					if dryRun {
						// --dry-run: never write files
						fmt.Fprintf(out, "[dry-run] No .gz-git.yaml found in %s.\n", workDir)
						fmt.Fprintln(out, "[dry-run] Would run: workspace init "+workDir)
						fmt.Fprintf(out, "[dry-run] Would create: %s\n", filepath.Join(workDir, DefaultConfigFile))
						fmt.Fprintln(out, "[dry-run] No changes made.")
						return nil
					}

					fmt.Fprintf(out, "ℹ️  No .gz-git.yaml found in %s\n", workDir)
					fmt.Fprintln(out, "   Running 'workspace init' first...")
					fmt.Fprintln(out, "")

					initOpts := &InitOptions{
						Path:     workDir,
						Output:   DefaultConfigFile,
						Depth:    initDepth,
						Kind:     initKind,
						Strategy: "pull",
						Force:    initForce,
					}
					if err := f.runInit(cmd, initOpts); err != nil {
						return fmt.Errorf("auto-init failed: %w", err)
					}

					// Re-detect after init. If still not found, repos were 0.
					detected2, err2 := detectConfigFile(workDir)
					if err2 != nil {
						configHint := filepath.Join(workDir, DefaultConfigFile)
						return fmt.Errorf(
							"auto-init completed but no config was created.\n"+
								"  Likely cause: no git repositories found under %s\n\n"+
								"  Next steps:\n"+
								"    • Add git repos to %s, then run: gz-git sync\n"+
								"    • Or create a config manually:  gz-git workspace init %s --template\n"+
								"    • Or point to an existing config: gz-git sync -c <config.yaml>\n"+
								"  (Expected config at: %s)",
							workDir, workDir, workDir, configHint,
						)
					}
					effectiveConfig = detected2
					fmt.Fprintf(out, "\n✓ Auto-init complete → %s\n", effectiveConfig)

					// After auto-init, offer the user a chance to review before syncing,
					// unless --no-review or --yes was passed, or not a terminal.
					if !noReview && !yes && isTerminal() {
						fmt.Fprintln(out, "")
						fmt.Fprintln(out, "  Review the generated config before syncing?")
						fmt.Fprintf(out, "    View:   cat %s\n", effectiveConfig)
						fmt.Fprintln(out, "    Sync:   press Enter to continue  (Ctrl-C to cancel)")
						fmt.Fprintln(out, "    Tip:    use --no-review to skip this prompt next time")
						fmt.Fprintln(out, "")
						// Simple "press Enter" prompt (no dependency on huh/survey)
						if _, scanErr := fmt.Scanln(); scanErr != nil && scanErr.Error() != "unexpected newline" {
							return fmt.Errorf("prompt cancelled: %w", scanErr)
						}
					}
				} else {
					effectiveConfig = detected
				}
			}

			// ── 3. Build args to pass to inner workspace sync ─────────────────
			// Use cmd.Flags().Changed() so we never guess at default values
			// and don't accidentally override config-file defaults.
			absConfig, _ := filepath.Abs(effectiveConfig)
			syncArgs := []string{"--config", absConfig}

			flags := cmd.Flags()
			addIfChanged := func(name, argName string) {
				if flags.Changed(name) {
					f := flags.Lookup(name)
					if f == nil {
						return
					}
					if f.Value.Type() == "bool" {
						if f.Value.String() == "true" {
							syncArgs = append(syncArgs, "--"+argName)
						}
					} else {
						syncArgs = append(syncArgs, "--"+argName, f.Value.String())
					}
				}
			}

			addIfChanged("strategy", "strategy")
			addIfChanged("parallel", "parallel")
			addIfChanged("max-retries", "max-retries")
			addIfChanged("resume", "resume")
			addIfChanged("dry-run", "dry-run")
			addIfChanged("yes", "yes")
			addIfChanged("interactive", "interactive")
			addIfChanged("verbose", "verbose")
			addIfChanged("state-file", "state-file")
			addIfChanged("full", "full")
			addIfChanged("recursive", "recursive")
			addIfChanged("recursive-depth", "recursive-depth")
			addIfChanged("format", "format")
			addIfChanged("push", "push")

			// ── 4. Delegate to workspace sync ─────────────────────────────────
			syncCmd := f.newSyncCmd()
			syncCmd.SilenceErrors = true
			syncCmd.SilenceUsage = true
			syncCmd.SetArgs(syncArgs)
			syncCmd.SetOut(out)
			syncCmd.SetErr(cmd.ErrOrStderr())
			syncCmd.SetContext(ctx)

			if err := syncCmd.Execute(); err != nil {
				return err
			}

			// ── 5. Optional post-sync health check ────────────────────────────
			if check && !dryRun {
				fmt.Fprintln(out, "")
				fmt.Fprintln(out, "─── Post-sync health check ────────────────────────────────────")
				statusOpts := &reposynccli.StatusOptions{
					ConfigFile: absConfig,
					Timeout:    30 * time.Second,
					Parallel:   4,
					ScanDepth:  1,
					SkipFetch:  true, // we just synced, skip remote fetch for speed
				}
				loader := reposynccli.FileSpecLoader{}
				if err := reposynccli.RunStatus(cmd, statusOpts, loader); err != nil {
					// Non-fatal: status failure shouldn't undo the sync
					fmt.Fprintf(out, "⚠️  Status check failed: %v\n", err)
				}
			}

			return nil
		},
	}

	// ── Sync flags (exact mirror of workspace sync) ───────────────────────────
	cmd.Flags().StringVarP(&configPath, "config", "c", "",
		"Path to config file (auto-detects "+DefaultConfigFile+"; skips auto-init when provided)")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Strategy override (reset|pull|rebase|fetch)")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Parallel workers (overrides config)")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Retry attempts per repo (overrides config)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from previous state")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without making changes (skips auto-init too)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Ask for confirmation before executing")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed repository preview")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to persist run state for resume")
	cmd.Flags().BoolVar(&fullOutput, "full", false, "Output all fields (name, path) even if redundant")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Recursively sync child workspaces")
	cmd.Flags().IntVar(&recursiveDepth, "recursive-depth", 3, "Maximum recursion depth for --recursive")
	cmd.Flags().StringVar(&format, "format", "default", "Output format (default, compact, json, llm)")

	// ── Auto-init flags ───────────────────────────────────────────────────────
	cmd.Flags().IntVar(&initDepth, "init-depth", 2,
		"Scan depth for auto-init (when .gz-git.yaml is absent)")
	cmd.Flags().StringVar(&initKind, "init-kind", "workspace",
		"Config kind for auto-init: workspace|repositories")
	cmd.Flags().BoolVar(&initForce, "init-force", false,
		"Overwrite existing config during auto-init")
	cmd.Flags().BoolVar(&noReview, "no-review", false,
		"Skip review prompt after auto-init, proceed to sync immediately")

	// ── Extra features ────────────────────────────────────────────────────────
	cmd.Flags().BoolVar(&check, "check", false,
		"Run 'workspace status' after sync to verify repository health")
	cmd.Flags().BoolVar(&pushAfterSync, "push", false,
		"Push after successful sync (only repos with local commits ahead)")

	return cmd
}
