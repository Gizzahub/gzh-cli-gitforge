// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"os/signal"
	"syscall"

	"github.com/gizzahub/gzh-cli-gitforge/internal/tasksync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/spf13/cobra"
)

func newTaskSyncPrepareCommand() *cobra.Command {
	return &cobra.Command{Use: "task-sync-prepare", Hidden: true, Args: cobra.NoArgs, SilenceUsage: true, RunE: func(c *cobra.Command, _ []string) error {
		if !supportsCapability(taskSyncPrepareV1Capability) {
			return cliutil.NewExitError(2, errors.New("task sync prepare wire is not accepted"))
		}
		signal.Ignore(syscall.SIGPIPE)
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		return taskSyncExit(tasksync.RunPrepareCoprocess(ctx, c.InOrStdin(), c.OutOrStdout()))
	}}
}
func init() { rootCmd.AddCommand(newTaskSyncPrepareCommand()) }
