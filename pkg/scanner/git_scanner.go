// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package scanner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// GitRepoScanner scans directories for git repositories.
type GitRepoScanner struct {
	RootPath       string
	MaxDepth       int
	RespectGitIgnore bool
	ExcludePatterns []string
	IncludePatterns []string
}

// ScannedRepo represents a discovered git repository.
type ScannedRepo struct {
	Name       string
	Path       string
	RemoteURLs []string // Can be empty, single, or multiple
	Depth      int      // Depth from root
}

// Scan discovers all git repositories under RootPath.
func (s *GitRepoScanner) Scan(ctx context.Context) ([]*ScannedRepo, error) {
	absRoot, err := filepath.Abs(s.RootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	var repos []*ScannedRepo

	// Load .gitignore patterns if needed
	var gitignorePatterns []string
	if s.RespectGitIgnore {
		gitignorePatterns = s.loadGitIgnorePatterns(absRoot)
	}

	err = s.scanDir(ctx, absRoot, absRoot, 0, gitignorePatterns, &repos)
	if err != nil {
		return nil, err
	}

	return repos, nil
}

func (s *GitRepoScanner) scanDir(ctx context.Context, root, current string, depth int, gitignorePatterns []string, repos *[]*ScannedRepo) error {
	// Check depth limit
	if depth > s.MaxDepth {
		return nil
	}

	// Check if current directory is a git repo
	gitDir := filepath.Join(current, ".git")
	if isDir(gitDir) {
		// Found a git repository
		repo, err := s.analyzeRepo(root, current, depth)
		if err != nil {
			// Log error but continue scanning
			fmt.Fprintf(os.Stderr, "Warning: failed to analyze repo at %s: %v\n", current, err)
		} else {
			*repos = append(*repos, repo)
		}
	}

	// Scan subdirectories
	entries, err := os.ReadDir(current)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", current, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		subPath := filepath.Join(current, name)

		// Skip .git directories
		if name == ".git" {
			continue
		}

		// Check exclusion patterns
		if s.shouldExclude(subPath, root, gitignorePatterns) {
			continue
		}

		// Check inclusion patterns (override exclusion)
		if s.shouldInclude(subPath, root) {
			// Force include
		} else if s.shouldExclude(subPath, root, gitignorePatterns) {
			continue
		}

		// Recursively scan subdirectory
		err = s.scanDir(ctx, root, subPath, depth+1, gitignorePatterns, repos)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *GitRepoScanner) analyzeRepo(root, repoPath string, depth int) (*ScannedRepo, error) {
	name := filepath.Base(repoPath)

	// Get remote URLs
	remoteURLs, err := s.getRemoteURLs(repoPath)
	if err != nil {
		// Don't fail, just use empty URLs
		remoteURLs = []string{}
	}

	return &ScannedRepo{
		Name:       name,
		Path:       repoPath,
		RemoteURLs: remoteURLs,
		Depth:      depth,
	}, nil
}

func (s *GitRepoScanner) getRemoteURLs(repoPath string) ([]string, error) {
	gitConfig := filepath.Join(repoPath, ".git", "config")

	file, err := os.Open(gitConfig)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	var inRemoteSection bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for remote section
		if strings.HasPrefix(line, "[remote ") {
			inRemoteSection = true
			continue
		}

		// End of section
		if strings.HasPrefix(line, "[") {
			inRemoteSection = false
			continue
		}

		// Parse URL in remote section
		if inRemoteSection && strings.HasPrefix(line, "url = ") {
			url := strings.TrimPrefix(line, "url = ")
			urls = append(urls, url)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}

func (s *GitRepoScanner) loadGitIgnorePatterns(root string) []string {
	gitignorePath := filepath.Join(root, ".gitignore")

	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		patterns = append(patterns, line)
	}

	return patterns
}

func (s *GitRepoScanner) shouldExclude(path, root string, gitignorePatterns []string) bool {
	// Get relative path from root
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		relPath = path
	}

	// Check user-defined exclude patterns
	for _, pattern := range s.ExcludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Simple pattern matching
		if matchesPattern(relPath, pattern) {
			return true
		}
	}

	// Check .gitignore patterns if enabled
	if s.RespectGitIgnore {
		for _, pattern := range gitignorePatterns {
			if matchesPattern(relPath, pattern) {
				return true
			}
		}
	}

	return false
}

func (s *GitRepoScanner) shouldInclude(path, root string) bool {
	if len(s.IncludePatterns) == 0 {
		return false
	}

	relPath, err := filepath.Rel(root, path)
	if err != nil {
		relPath = path
	}

	for _, pattern := range s.IncludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		if matchesPattern(relPath, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern performs simple glob-like pattern matching.
func matchesPattern(path, pattern string) bool {
	// Remove leading/trailing slashes
	path = strings.Trim(path, "/")
	pattern = strings.Trim(pattern, "/")

	// Exact match
	if path == pattern {
		return true
	}

	// Directory prefix match (pattern/)
	if strings.HasSuffix(pattern, "/") {
		dirPattern := strings.TrimSuffix(pattern, "/")
		if strings.HasPrefix(path, dirPattern+"/") || path == dirPattern {
			return true
		}
	}

	// Wildcard match (pattern/*)
	if strings.HasSuffix(pattern, "/*") {
		dirPattern := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(path, dirPattern+"/") {
			return true
		}
	}

	// Simple glob match
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	return matched
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ToRepositories converts ScannedRepos to provider.Repository format.
func ToRepositories(scanned []*ScannedRepo) []*provider.Repository {
	repos := make([]*provider.Repository, 0, len(scanned))

	for _, s := range scanned {
		repo := &provider.Repository{
			Name:     s.Name,
			FullName: s.Name,
			CloneURL: "",
		}

		// Handle remote URLs
		if len(s.RemoteURLs) > 0 {
			repo.CloneURL = s.RemoteURLs[0] // Use first URL as primary
		}

		repos = append(repos, repo)
	}

	return repos
}
