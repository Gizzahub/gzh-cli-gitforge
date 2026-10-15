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
	tracked, err := runInfraOpsCommandOutput(ctx, dir, nil, "git", "ls-files", "--stage", "-z", "--error-unmatch", "--", "codes/.envrc")
	if err != nil {
		return fmt.Errorf("verify tracked codes/.envrc: %w", err)
	}
	// core.symlinks=false materializes a tracked symlink as a regular file.
	// The index mode, exact path and stage must also describe regular source.
	records := strings.Split(tracked, "\x00")
	if len(records) != 2 || records[1] != "" {
		return fmt.Errorf("codes/.envrc must have one tracked stage-0 entry")
	}
	header, path, found := strings.Cut(records[0], "\t")
	fields := strings.Fields(header)
	if !found || path != "codes/.envrc" || len(fields) != 3 || fields[2] != "0" ||
		(fields[0] != "100644" && fields[0] != "100755") {
		return fmt.Errorf("codes/.envrc must be a tracked regular stage-0 source file")
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
