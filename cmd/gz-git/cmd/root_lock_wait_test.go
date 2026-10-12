// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestExecuteCommandResetsLockWait(t *testing.T) {
	var wait time.Duration
	var got []struct {
		wait    time.Duration
		changed bool
	}
	check := &cobra.Command{Use: "check", Run: func(cmd *cobra.Command, _ []string) {
		got = append(got, struct {
			wait    time.Duration
			changed bool
		}{wait, cmd.Flags().Changed("lock-wait")})
	}}
	check.Flags().DurationVar(&wait, "lock-wait", 0, "test lock wait")
	root := &cobra.Command{Use: "root"}
	root.AddCommand(check)

	root.SetArgs([]string{"check", "--lock-wait", "25ms"})
	if err := executeCommand(root); err != nil {
		t.Fatalf("explicit invocation: %v", err)
	}
	root.SetArgs([]string{"check"})
	if err := executeCommand(root); err != nil {
		t.Fatalf("omitted invocation: %v", err)
	}
	if len(got) != 2 || got[0].wait != 25*time.Millisecond || !got[0].changed || got[1].wait != 0 || got[1].changed {
		t.Fatalf("lock-wait leaked across reused command tree: %#v", got)
	}
}
