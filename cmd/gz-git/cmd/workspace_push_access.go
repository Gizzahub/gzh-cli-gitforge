// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func configuredWorkspacePushAccess(repoPath string) (allowed bool, reason string, err error) {
	access, configPath, err := config.WorkspacePushAccess(repoPath)
	if err != nil {
		return false, "", err
	}
	if !access.IsReadOnly() {
		return true, "", nil
	}
	return false, fmt.Sprintf("read-only workspace declared in %s; push disabled", configPath), nil
}

// configuredWorkspaceRemoteDeleteGuard adapts the workspace access declaration
// at the command boundary. pkg/branch and pkg/repository receive only this
// decision, never workspace configuration.
func configuredWorkspaceRemoteDeleteGuard(ctx context.Context, repo *repository.Repository) error {
	allowed, reason, err := configuredWorkspacePushAccess(repo.Path)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return fmt.Errorf("%s; remote branch deletion disabled", reason)
}

func configuredWorkspaceBulkRemoteDeleteGuard(ctx context.Context, repoPath string) error {
	return configuredWorkspaceRemoteDeleteGuard(ctx, &repository.Repository{Path: repoPath})
}

var _ branch.RemoteDeleteGuard = configuredWorkspaceRemoteDeleteGuard
