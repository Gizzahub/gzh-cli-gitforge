// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package tag

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// Manager defines tag operations.
type Manager interface {
	// Create creates a new tag.
	Create(ctx context.Context, repo *repository.Repository, opts CreateOptions) error

	// List returns all tags.
	List(ctx context.Context, repo *repository.Repository, opts ListOptions) ([]*Tag, error)

	// Push pushes tags to remote.
	Push(ctx context.Context, repo *repository.Repository, opts PushOptions) error

	// Delete deletes a tag.
	Delete(ctx context.Context, repo *repository.Repository, opts DeleteOptions) error

	// Exists checks if a tag exists.
	Exists(ctx context.Context, repo *repository.Repository, name string) (bool, error)

	// Latest returns the latest tag by version or date.
	Latest(ctx context.Context, repo *repository.Repository) (*Tag, error)

	// NextVersion suggests the next version based on current tags.
	NextVersion(ctx context.Context, repo *repository.Repository, bump string) (string, error)
}

// manager implements Manager.
type manager struct {
	executor *gitcmd.Executor
}

// NewManager creates a new tag manager.
func NewManager() Manager {
	return &manager{
		executor: gitcmd.NewExecutor(),
	}
}

// Create creates a new tag.
func (m *manager) Create(ctx context.Context, repo *repository.Repository, opts CreateOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}
	if opts.Name == "" {
		return fmt.Errorf("tag name is required")
	}

	args := []string{"tag"}

	if opts.Message != "" {
		args = append(args, "-a", "-m", opts.Message)
	}

	if opts.Sign {
		args = append(args, "-s")
	}

	if opts.Force {
		args = append(args, "-f")
	}

	args = append(args, opts.Name)

	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git tag failed: %s", result.Stderr)
	}

	return nil
}

// List returns all tags.
func (m *manager) List(ctx context.Context, repo *repository.Repository, opts ListOptions) ([]*Tag, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// Get tags with format
	args := []string{"tag", "-l", "--format=%(refname:short)|%(objectname:short)|%(creatordate:unix)|%(contents:subject)"}

	if opts.Pattern != "" {
		args = append(args, opts.Pattern)
	}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git tag list failed: %s", result.Stderr)
	}

	var tags []*Tag
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		tag := parseTagLine(line)
		if tag != nil {
			tags = append(tags, tag)
		}
	}

	// Sort tags
	switch opts.Sort {
	case "version":
		sort.Slice(tags, func(i, j int) bool {
			return compareSemVer(tags[i].Name, tags[j].Name) > 0
		})
	case "date":
		sort.Slice(tags, func(i, j int) bool {
			return tags[i].Date.After(tags[j].Date)
		})
	case "name":
		sort.Slice(tags, func(i, j int) bool {
			return tags[i].Name < tags[j].Name
		})
	default:
		// Default: version sort
		sort.Slice(tags, func(i, j int) bool {
			return compareSemVer(tags[i].Name, tags[j].Name) > 0
		})
	}

	// Apply limit
	if opts.Limit > 0 && len(tags) > opts.Limit {
		tags = tags[:opts.Limit]
	}

	return tags, nil
}

// Push pushes tags to remote.
func (m *manager) Push(ctx context.Context, repo *repository.Repository, opts PushOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}

	args := []string{"push", remote}

	if opts.All {
		args = append(args, "--tags")
	} else if opts.Name != "" {
		args = append(args, opts.Name)
	} else {
		return fmt.Errorf("either --all or tag name is required")
	}

	if opts.Force {
		args = append(args, "--force")
	}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return fmt.Errorf("failed to push tag: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git push tag failed: %s", result.Stderr)
	}

	return nil
}

// Delete deletes a tag.
func (m *manager) Delete(ctx context.Context, repo *repository.Repository, opts DeleteOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}
	if opts.Name == "" {
		return fmt.Errorf("tag name is required")
	}

	// Delete local tag
	result, err := m.executor.Run(ctx, repo.Path, "tag", "-d", opts.Name)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git tag delete failed: %s", result.Stderr)
	}

	// Delete remote tag if requested
	if opts.Remote {
		result, err = m.executor.Run(ctx, repo.Path, "push", "origin", ":refs/tags/"+opts.Name)
		if err != nil {
			return fmt.Errorf("failed to delete remote tag: %w", err)
		}
	}

	return nil
}

// Exists checks if a tag exists.
func (m *manager) Exists(ctx context.Context, repo *repository.Repository, name string) (bool, error) {
	if repo == nil {
		return false, fmt.Errorf("repository cannot be nil")
	}

	result, _ := m.executor.Run(ctx, repo.Path, "rev-parse", "--verify", "refs/tags/"+name)
	return result.ExitCode == 0, nil
}

// Latest returns the latest tag by version.
func (m *manager) Latest(ctx context.Context, repo *repository.Repository) (*Tag, error) {
	tags, err := m.List(ctx, repo, ListOptions{Sort: "version", Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}
	return tags[0], nil
}

// NextVersion suggests the next version based on current tags.
func (m *manager) NextVersion(ctx context.Context, repo *repository.Repository, bump string) (string, error) {
	latest, err := m.Latest(ctx, repo)
	if err != nil {
		return "", err
	}

	if latest == nil {
		return "v0.1.0", nil
	}

	// Parse current version
	major, minor, patch := parseSemVer(latest.Name)

	switch bump {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	default:
		patch++
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

// parseTagLine parses a tag line from git output.
func parseTagLine(line string) *Tag {
	parts := strings.SplitN(line, "|", 4)
	if len(parts) < 3 {
		return nil
	}

	name := parts[0]
	sha := parts[1]
	timestampStr := parts[2]
	message := ""
	if len(parts) > 3 {
		message = parts[3]
	}

	timestamp, _ := strconv.ParseInt(timestampStr, 10, 64)
	date := time.Unix(timestamp, 0)

	return &Tag{
		Name:        name,
		Ref:         "refs/tags/" + name,
		SHA:         sha,
		Message:     message,
		Date:        date,
		IsAnnotated: message != "",
	}
}

// parseSemVer parses a semver version string.
func parseSemVer(version string) (major, minor, patch int) {
	// Remove 'v' prefix
	version = strings.TrimPrefix(version, "v")

	// Extract numbers
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(version)

	if len(matches) >= 4 {
		major, _ = strconv.Atoi(matches[1])
		minor, _ = strconv.Atoi(matches[2])
		patch, _ = strconv.Atoi(matches[3])
	}

	return
}

// compareSemVer compares two semver versions.
// Returns positive if a > b, negative if a < b, 0 if equal.
func compareSemVer(a, b string) int {
	aMajor, aMinor, aPatch := parseSemVer(a)
	bMajor, bMinor, bPatch := parseSemVer(b)

	if aMajor != bMajor {
		return aMajor - bMajor
	}
	if aMinor != bMinor {
		return aMinor - bMinor
	}
	return aPatch - bPatch
}
