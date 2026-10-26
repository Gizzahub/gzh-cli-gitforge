// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/safefs"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// loadGitWorkspace resolves a `type: git` workspace the way workspace sync
// plans it: a workspace that declares a url is the checkout of one
// repository at its path, named after the workspace.
//
// Every other shape keeps the depth-1 scan of the path's subdirectories, as
// sync plans none of them: a workspace without a url, the config directory
// itself (so a devbox whose `path: .` workspace lists its member checkouts
// still does), and an existing directory that is not a repository.
//
// A declared path that does not exist is still one repository, as sync
// would clone it there; status then reports it as unreachable instead of
// dropping it. A path that cannot be read is returned as an error for the
// caller to report.
func loadGitWorkspace(name string, ws *workspaceEntry, wsPath, configDir string) ([]reposync.RepoSpec, error) {
	if ws.URL == "" || config.SameDir(wsPath, configDir) {
		return scanGitRepos(wsPath)
	}
	one := []reposync.RepoSpec{{Name: name, TargetPath: wsPath, CloneURL: ws.URL}}
	// .git is a directory in a clone and a file in a linked worktree;
	// either way the path is a checkout.
	if _, err := os.Stat(filepath.Join(wsPath, ".git")); err == nil {
		return one, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if _, err := os.Stat(wsPath); errors.Is(err, fs.ErrNotExist) {
		return one, nil
	} else if err != nil {
		return nil, err
	}
	return scanGitRepos(wsPath)
}

// scanGitRepos scans a directory for git repositories (depth 1).
func scanGitRepos(dir string) ([]reposync.RepoSpec, error) {
	root, err := safefs.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	entries, err := root.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var repos []reposync.RepoSpec
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		repoRel := entry.Name()
		repoPath := filepath.Join(dir, repoRel)

		// Check if it's a git repository
		repoRoot, err := root.OpenRoot(repoRel)
		if err != nil {
			continue
		}
		if info, err := repoRoot.Stat(".git"); err == nil && info.IsDir() {
			// Get remote URL if available
			remoteURL := getGitRemoteURLAt(repoRoot, ".")

			repos = append(repos, reposync.RepoSpec{
				Name:       entry.Name(),
				TargetPath: repoPath,
				CloneURL:   remoteURL,
			})
		}
		_ = repoRoot.Close()
	}

	return repos, nil
}

func getGitRemoteURLAt(root *safefs.Root, repoRel string) string {
	configPath := filepath.Join(repoRel, ".git", "config")
	data, err := root.ReadFile(configPath)
	if err != nil {
		return ""
	}

	// Simple parsing to find [remote "origin"] url
	lines := strings.Split(string(data), "\n")
	inOrigin := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == `[remote "origin"]` {
			inOrigin = true
			continue
		}
		if inOrigin {
			if strings.HasPrefix(line, "[") {
				break
			}
			if after, ok := strings.CutPrefix(line, "url = "); ok {
				return after
			}
		}
	}
	return ""
}
