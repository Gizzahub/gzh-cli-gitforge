// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/safefs"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// loadGitWorkspace resolves a `type: git` workspace the way workspace sync
// plans it: the path is the checkout of one repository, named after the
// workspace and cloned from its declared url.
//
// Two shapes keep the depth-1 scan of the path's subdirectories. The config
// directory itself, which sync refuses to plan as a repository, so a devbox
// whose `path: .` workspace lists its member checkouts still does. And an
// existing directory that is not a repository, the parent-directory form
// this loader has always accepted.
//
// A path that does not exist is still one repository when the workspace
// declares a url, as sync would clone it there; status then reports it as
// unreachable instead of dropping it. Without a url, and for any path that
// cannot be read, the error is returned for the caller to report.
func loadGitWorkspace(name string, ws *workspaceEntry, wsPath, configDir string) ([]reposync.RepoSpec, error) {
	if filepath.Clean(wsPath) == filepath.Clean(configDir) {
		return scanGitRepos(wsPath)
	}
	_, err := os.Stat(filepath.Join(wsPath, ".git"))
	switch {
	case err == nil:
		// .git is a directory in a clone and a file in a linked worktree;
		// either way the path is a checkout.
		url := ws.URL
		if url == "" {
			url = gitRemoteURL(wsPath)
		}
		return []reposync.RepoSpec{{Name: name, TargetPath: wsPath, CloneURL: url}}, nil
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("stat %s: %w", filepath.Join(wsPath, ".git"), err)
	}
	if _, err := os.Stat(wsPath); errors.Is(err, fs.ErrNotExist) {
		if ws.URL == "" {
			return nil, fmt.Errorf("path %s does not exist and the workspace declares no url", wsPath)
		}
		return []reposync.RepoSpec{{Name: name, TargetPath: wsPath, CloneURL: ws.URL}}, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat %s: %w", wsPath, err)
	}
	return scanGitRepos(wsPath)
}

// gitRemoteURL reads the origin URL of the checkout at path, or returns ""
// when it has none or its config cannot be read through the path's root.
func gitRemoteURL(path string) string {
	root, err := safefs.OpenRoot(path)
	if err != nil {
		return ""
	}
	defer func() { _ = root.Close() }()
	return getGitRemoteURLAt(root, ".")
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
