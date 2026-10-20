// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"path/filepath"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

// configDirSpecLoader loads a workspace config through reposynccli's loader
// and resolves each relative repository path against the config's
// directory, where `workspace sync` places it (ISSUE-86). Without it,
// `workspace status` and `quicksync --check` would look in the working
// directory instead of where sync cloned.
type configDirSpecLoader struct {
	base reposynccli.SpecLoader
}

func (l configDirSpecLoader) Load(ctx context.Context, path string) (reposynccli.ConfigData, error) {
	data, err := l.base.Load(ctx, path)
	if err != nil {
		return data, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return data, err
	}
	configDir := filepath.Dir(absPath)
	for i := range data.Plan.Input.Repos {
		repo := &data.Plan.Input.Repos[i]
		repo.TargetPath = resolveWorkspacePath(repo.TargetPath, configDir)
	}
	return data, nil
}
