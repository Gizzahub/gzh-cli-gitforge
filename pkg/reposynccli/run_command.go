package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func (f CommandFactory) newRunCmd() *cobra.Command {
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
		Use:   "run",
		Short: "Plan and execute repository synchronization",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			loader := f.SpecLoader
			if loader == nil {
				loader = FileSpecLoader{}
			}

			cfg, err := loader.Load(ctx, configPath)
			if err != nil {
				return err
			}

			planReq := cfg.Plan
			runOpts := cfg.Run

			if cmd.Flags().Changed("strategy") {
				parsed, err := reposync.ParseStrategy(strategy)
				if err != nil {
					return err
				}
				planReq.Options.DefaultStrategy = parsed
			}

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

			progress := ConsoleProgressSink{Out: cmd.OutOrStdout()}

			orch, err := f.orchestrator()
			if err != nil {
				return err
			}

			var state reposync.StateStore
			if stateFile != "" {
				state = reposync.NewFileStateStore(stateFile)
			}

			_, err = orch.Run(ctx, reposync.RunRequest{
				PlanRequest: planReq,
				RunOptions:  runOpts,
				Progress:    progress,
				State:       state,
			})
			if err != nil {
				return fmt.Errorf("run: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to git-sync config file")
	_ = cmd.MarkFlagRequired("config")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Default strategy override (reset|pull|fetch)")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Parallel workers (overrides config)")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Retry attempts per repo (overrides config)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from previous state (overrides config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry-run without touching git (overrides config)")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to persist and load run state for resume")

	return cmd
}
