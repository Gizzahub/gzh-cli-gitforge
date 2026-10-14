// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The real repository tracks this boundary marker alongside ignored checkouts.
// Preserve it as source input, never as a resource owned by preparation.
func validateInfraOpsCodesLayout(ctx context.Context, dir string, prepared bool) error {
	entries, err := os.ReadDir(filepath.Join(dir, "codes"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".envrc" {
			if err := verifyInfraOpsTrackedEnvrc(ctx, dir); err != nil {
				return err
			}
			continue
		}
		if prepared && isInfraOpsDependencyName(entry.Name()) {
			continue
		}
		return fmt.Errorf("refusing pre-existing codes resources: %s", entry.Name())
	}
	return nil
}

func verifyInfraOpsTrackedEnvrc(ctx context.Context, dir string) error {
	if err := requireRegularPreparationFile(filepath.Join(dir, "codes", ".envrc")); err != nil {
		return err
	}
	tracked, err := runInfraOpsCommandOutput(ctx, dir, nil, "git", "ls-files", "--error-unmatch", "--", "codes/.envrc")
	if err != nil {
		return fmt.Errorf("verify tracked codes/.envrc: %w", err)
	}
	if strings.TrimSpace(tracked) != "codes/.envrc" {
		return fmt.Errorf("codes/.envrc must be a tracked regular source file")
	}
	return nil
}

func isInfraOpsDependencyName(name string) bool {
	for _, dependency := range infraOpsDependencies {
		if name == dependency.name {
			return true
		}
	}
	return false
}
