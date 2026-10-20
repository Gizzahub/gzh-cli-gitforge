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
	// The base loader joins workspace paths with the config's directory as
	// given, so it gets an absolute path: a relative one would come back
	// relative to the working directory and be joined twice below.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return reposynccli.ConfigData{}, err
	}
	data, err := l.base.Load(ctx, absPath)
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
