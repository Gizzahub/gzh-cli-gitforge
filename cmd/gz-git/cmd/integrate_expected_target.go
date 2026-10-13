// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// expectedTargetFromFlag preserves the legacy absent-flag behavior while
// rejecting an explicitly empty approval before command setup can inspect a
// repository or start an integration effect.
func expectedTargetFromFlag(cmd *cobra.Command, value string) (string, error) {
	if !cmd.Flags().Changed("expect-target") {
		return value, nil
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("--expect-target must not be empty when supplied")
	}
	return value, nil
}
