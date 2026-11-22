// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"gopkg.in/yaml.v3"
)

// loadCommitMessages builds the per-repository message map from at most one
// of --file, --json and --yaml, or from piped stdin when none is given, then
// overlays every -m entry. It returns nil when no map was supplied.
func loadCommitMessages() (map[string]string, error) {
	sources := 0
	for _, set := range []bool{commitFile != "", commitJSON != "", commitYAML != ""} {
		if set {
			sources++
		}
	}
	if sources > 1 {
		return nil, fmt.Errorf("--file, --json and --yaml are alternative message maps; give only one")
	}

	var messages map[string]string
	var err error
	switch {
	case commitFile != "":
		messages, err = readCommitMessageFile(commitFile)
		if err != nil {
			return nil, err
		}
	case commitJSON != "":
		messages, err = parseJSONMessages(commitJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to parse --json: %w", err)
		}
	case commitYAML != "":
		messages, err = parseYAMLMessages(commitYAML)
		if err != nil {
			return nil, fmt.Errorf("failed to parse --yaml: %w", err)
		}
	default:
		// Piped stdin is read only when no explicit map was named, so a
		// caller that happens to run gz-git with a pipe on stdin cannot have
		// it silently override --json or --file.
		stat, statErr := os.Stdin.Stat()
		if statErr == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			pipedData, readErr := io.ReadAll(os.Stdin)
			if readErr != nil {
				return nil, fmt.Errorf("failed to read piped data: %w", readErr)
			}
			if len(pipedData) > 0 {
				messages, err = parseJSONOrYAMLMessages(string(pipedData))
				if err != nil {
					return nil, fmt.Errorf("failed to parse piped data: %w", err)
				}
			}
		}
	}

	if len(commitMessages) > 0 {
		if messages == nil {
			messages = make(map[string]string)
		}
		for _, raw := range commitMessages {
			repo, message, parseErr := parseRepoMessage(raw)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid --message format: %w", parseErr)
			}
			messages[repo] = message
		}
	}
	return messages, nil
}

// readCommitMessageFile reads a JSON or YAML message map from file, or from
// stdin when file is "-". An empty or null map is an error: --file names a
// map, and treating a blank one as "no map" would quietly commit every
// repository with a generated message.
func readCommitMessageFile(file string) (map[string]string, error) {
	var (
		data []byte
		err  error
	)
	if file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(file) // #nosec G304 -- the operator names the message file.
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read --file %s: %w", file, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("--file %s is empty; nothing was committed", file)
	}
	messages, err := parseJSONOrYAMLMessages(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse --file %s: %w", file, err)
	}
	if messages == nil {
		return nil, fmt.Errorf("--file %s holds no message map; nothing was committed", file)
	}
	return messages, nil
}

// commitMessageKeys returns the keys that address repoPath, in lookup order:
// the slash-separated path relative to directory, its basename, and the
// absolute path. The scan directory itself is "." and, for compatibility,
// also its basename.
func commitMessageKeys(directory, repoPath string) []string {
	relPath, err := filepath.Rel(directory, repoPath)
	if err != nil {
		relPath = filepath.Base(repoPath)
	}
	relPath = filepath.ToSlash(relPath)
	if relPath == "." {
		return []string{".", filepath.Base(directory), repoPath}
	}
	return []string{relPath, path.Base(relPath), repoPath}
}

// lookupCommitMessageKey returns the first key of messages that addresses
// repoPath.
func lookupCommitMessageKey(directory, repoPath string, messages map[string]string) (string, bool) {
	for _, key := range commitMessageKeys(directory, repoPath) {
		if _, ok := messages[key]; ok {
			return key, true
		}
	}
	return "", false
}

func customCommitMessageGenerator(directory string, messages map[string]string) func(context.Context, string, []string) (string, error) {
	return func(_ context.Context, repoPath string, _ []string) (string, error) {
		if key, ok := lookupCommitMessageKey(directory, repoPath, messages); ok {
			return messages[key], nil
		}
		return "", nil
	}
}

// validateCommitMessageCoverage rejects a message map that leaves out a
// repository about to be committed. A caller that names messages has said what
// each commit records; filling a gap with a generated message would commit a
// repository the caller may not know was dirty. Conflicted and failed
// repositories are not committed, so they need no message.
func validateCommitMessageCoverage(directory string, messages map[string]string, preview *repository.BulkCommitResult) error {
	var missing []string
	for _, repo := range preview.Repositories {
		if repo.Status != "dirty" && repo.Status != "would-commit" {
			continue
		}
		if _, ok := lookupCommitMessageKey(directory, repo.Path, messages); !ok {
			missing = append(missing, repo.RelativePath)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("no commit message for %s: add them to the message map, or pass --auto-message to generate them; nothing was committed",
		strings.Join(missing, ", "))
}

// validateCommitMessageKeys rejects message keys that match none of the
// repositories with changes in a dry-run result, naming every unmatched key and
// the valid ones. Without it an unmatched key silently commits the auto
// message. Keys match exactly as in lookupCommitMessageKey. A key for a
// conflicted repository, or one whose analysis failed, counts as matched so
// the run reports that refusal or error itself.
func validateCommitMessageKeys(directory string, messages map[string]string, preview *repository.BulkCommitResult) error {
	matched := make(map[string]bool, len(messages))
	valid := make([]string, 0, len(preview.Repositories))
	for _, repo := range preview.Repositories {
		if repo.Status == "clean" {
			continue
		}
		valid = append(valid, repo.RelativePath)
		for _, key := range commitMessageKeys(directory, repo.Path) {
			if _, ok := messages[key]; ok {
				matched[key] = true
			}
		}
	}
	var unmatched []string
	for key := range messages {
		if !matched[key] {
			unmatched = append(unmatched, key)
		}
	}
	if len(unmatched) == 0 {
		return nil
	}
	sort.Strings(unmatched)
	sort.Strings(valid)
	return fmt.Errorf("commit message key(s) match no repository with changes: %s (valid keys: %s); nothing was committed",
		strings.Join(unmatched, ", "), strings.Join(valid, ", "))
}

// parseRepoMessage parses "repo:message" format.
func parseRepoMessage(input string) (repo, message string, err error) {
	before, after, ok := strings.Cut(input, ":")
	if !ok {
		return "", "", fmt.Errorf("expected format 'repo:message', got %q", input)
	}
	repo = strings.TrimSpace(before)
	message = strings.TrimSpace(after)
	if repo == "" || message == "" {
		return "", "", fmt.Errorf("repo and message cannot be empty in %q", input)
	}
	return repo, message, nil
}

// parseJSONMessages parses inline JSON string to message map.
func parseJSONMessages(jsonStr string) (map[string]string, error) {
	var messages map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &messages); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return messages, nil
}

// parseYAMLMessages parses inline YAML string to message map.
func parseYAMLMessages(yamlStr string) (map[string]string, error) {
	var messages map[string]string
	if err := yaml.Unmarshal([]byte(yamlStr), &messages); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return messages, nil
}

// parseJSONOrYAMLMessages tries parsing JSON first, then YAML.
func parseJSONOrYAMLMessages(data string) (map[string]string, error) {
	var messages map[string]string
	if err := json.Unmarshal([]byte(data), &messages); err == nil {
		return messages, nil
	}
	if err := yaml.Unmarshal([]byte(data), &messages); err == nil {
		return messages, nil
	}
	return nil, fmt.Errorf("data is not valid payload (expected JSON or YAML)")
}

// applyCustomMessages applies custom messages to repository results.
func applyCustomMessages(result *repository.BulkCommitResult, messages map[string]string) {
	for i := range result.Repositories {
		repo := &result.Repositories[i]
		// Try to match by relative path or full path
		if msg, ok := messages[repo.RelativePath]; ok {
			repo.Message = msg
		} else if msg, ok := messages[path.Base(repo.RelativePath)]; ok {
			repo.Message = msg
		} else if msg, ok := messages[filepath.Base(repo.Path)]; ok && repo.RelativePath == "." {
			repo.Message = msg
		} else if msg, ok := messages[repo.Path]; ok {
			repo.Message = msg
		}
	}
}
