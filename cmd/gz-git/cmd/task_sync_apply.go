// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/tasksync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// newTaskSyncApplyCommand is the child side of task-sync-apply-v1. A caller
// must speak that protocol on stdin and stdout, so it stays out of help. The
// verdict travels in the result message; the exit code only reports whether
// that message could exist (2: no valid begin, 3: not delivered).
func newTaskSyncApplyCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "task-sync-apply",
		Short:        "Serve one task-sync-apply-v1 exchange on stdio",
		Hidden:       true,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Without this, a write to a dead owner kills the process before the
			// loss can be handled as a preserved, uncertain apply.
			signal.Ignore(syscall.SIGPIPE)
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return taskSyncExit(tasksync.RunCoprocess(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), tasksync.DefaultCoprocessResponseTimeout))
		},
	}
}

func taskSyncExit(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tasksync.ErrCoprocessBegin):
		return cliutil.NewExitError(2, err)
	case errors.Is(err, tasksync.ErrCoprocessResult):
		return cliutil.NewExitError(3, err)
	default:
		return fmt.Errorf("task sync apply: %w", err)
	}
}

func init() {
	rootCmd.AddCommand(newTaskSyncApplyCommand())
}
