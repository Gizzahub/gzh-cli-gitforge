// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package wizard provides interactive setup wizards for gz-git commands.
//
// The wizard package uses charmbracelet/huh for form-based interactive input,
// guiding users through complex configuration tasks step by step.
//
// Available Wizards:
//   - SyncSetup: Configure repository synchronization from Git forges
//   - BranchCleanup: Interactive branch cleanup across repositories
//   - ProfileCreate: Create configuration profiles
//
// Example usage:
//
//	wizard := wizard.NewSyncSetupWizard()
//	opts, err := wizard.Run(ctx)
//	if err != nil {
//	    return err
//	}
//	// Use opts to run sync
package wizard
