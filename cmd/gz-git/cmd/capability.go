// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	integrateQueueControllerV1Capability         = "integrate-queue-controller-v1"
	integrateQueueBaseMissingV1Capability        = "integrate-queue-base-missing-v1"
	contextReferenceObserveV1Capability          = "context-reference-observe-v1"
	integrateRunContextV1Capability              = "integrate-run-context-v1"
	integrateReconcilePreMutationV1Capability    = "integrate-reconcile-pre-mutation-v1"
	integrateLegacyCleanupStandaloneV1Capability = "integrate-legacy-cleanup-standalone-v1"
	integrateLegacyCleanupCEV1Capability         = "integrate-legacy-cleanup-ce-v1"
)

func newCapabilityCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "capability NAME",
		Short: "Probe a machine-readable CLI capability",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !supportsCapability(args[0]) {
				return fmt.Errorf("unsupported capability %q", args[0])
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), args[0]); err != nil {
				return fmt.Errorf("write capability result: %w", err)
			}
			return nil
		},
	}
}

func supportsCapability(name string) bool {
	switch name {
	case integrateQueueControllerV1Capability, integrateQueueBaseMissingV1Capability, contextReferenceObserveV1Capability, integrateRunContextV1Capability, integrateReconcilePreMutationV1Capability, integrateLegacyCleanupStandaloneV1Capability, integrateLegacyCleanupCEV1Capability:
		return true
	default:
		return false
	}
}

func init() {
	rootCmd.AddCommand(newCapabilityCommand())
}
