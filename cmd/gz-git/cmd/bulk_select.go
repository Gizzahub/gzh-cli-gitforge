// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// Selection modes for --select.
const (
	selectAll    = "all"
	selectListed = "listed"
)

// RepoSelectionHelpText documents --select/--repo/--repos-from once for every
// bulk command that registers them.
const RepoSelectionHelpText = `
Repository Selection:
  --select all (default) runs on every repository the scan finds.
  --select listed runs only on declared repositories:
    - the workspaces of the nearest .gz-git.yaml (git workspaces are one
      repository each; forge and config workspaces select every repository
      under their path; selfSync.enabled declares the config directory "."),
    - narrowed to the --repo / --repos-from entries when any are given.
  An entry that is not declared, or a declared repository that is not on disk,
  stops the run before anything is touched. Without workspaces in the config,
  --repo / --repos-from alone is the list. Entries are paths relative to the
  scan directory; "." is the scan directory itself.`

// stdinReaderForSelection is the stream --repos-from - reads. Tests replace it.
var stdinReaderForSelection io.Reader = os.Stdin

// stdinSelection caches the --repos-from - list. Watch mode resolves the
// selection on every iteration, and a second read of an already drained stdin
// would return no entries, silently widening the run to every declared
// repository.
var stdinSelection struct {
	once    sync.Once
	entries []string
	err     error
}

// addRepoSelectionFlags registers --select, --repo and --repos-from.
func addRepoSelectionFlags(cmd *cobra.Command, flags *BulkCommandFlags) {
	if cmd.Long != "" && !strings.Contains(cmd.Long, "Repository Selection:") {
		cmd.Long = strings.TrimRight(cmd.Long, "\n") + "\n" + RepoSelectionHelpText
	}
	cmd.Flags().StringVar(&flags.Select, "select", selectAll, "repository selection: all (every scanned repository) or listed (declared workspaces, narrowed by --repo)")
	cmd.Flags().StringArrayVar(&flags.Repos, "repo", nil, "with --select listed: run only on this repository path (repeatable)")
	cmd.Flags().StringVar(&flags.ReposFrom, "repos-from", "", "with --select listed: read repository paths from a file, one per line (- for stdin)")
}

// resolveBulkSelection turns the selection flags into the repository
// selection the bulk operation runs on.
//
// Listed mode narrows; it never widens. The config's workspaces are the
// universe and --repo picks from it, the way docker compose services or a nx
// project list work: an entry outside the universe is an error rather than an
// implicit addition, so a typo cannot become a run on an unexpected
// repository. Declaring a repository permanently is a config change.
func resolveBulkSelection(directory string, flags *BulkCommandFlags) (repository.RepoSelection, error) {
	mode := flags.Select
	if mode == "" {
		mode = selectAll
	}
	switch mode {
	case selectAll:
		if len(flags.Repos) > 0 || flags.ReposFrom != "" {
			return repository.RepoSelection{}, fmt.Errorf("--repo and --repos-from require --select listed")
		}
		return repository.RepoSelection{}, nil
	case selectListed:
	default:
		return repository.RepoSelection{}, fmt.Errorf("invalid --select %q: use all or listed", flags.Select)
	}

	entries, err := selectionEntries(flags)
	if err != nil {
		return repository.RepoSelection{}, err
	}
	// A given list that comes out empty (a filter that printed nothing, a file
	// of comments) must not fall through to "every declared repository".
	if len(entries) == 0 && (len(flags.Repos) > 0 || flags.ReposFrom != "") {
		return repository.RepoSelection{}, fmt.Errorf("--select listed: --repo / --repos-from gave no repository paths; nothing was run")
	}

	abs, err := filepath.Abs(directory)
	if err != nil {
		return repository.RepoSelection{}, fmt.Errorf("failed to resolve %s: %w", directory, err)
	}
	declared, found, err := config.FindDeclaredRepositories(abs)
	if err != nil {
		return repository.RepoSelection{}, fmt.Errorf("--select listed: %w", err)
	}

	if found && declared.DeclaresAny && declared.Empty() {
		// The config declares repositories, just none this scan can reach. An
		// --repo entry here would be undeclared, so refuse rather than treat
		// the config as declaring nothing.
		return repository.RepoSelection{}, fmt.Errorf("--select listed: %s declares no workspace under this scan directory; scan from the config directory or use --select all; nothing was run",
			declared.ConfigPath)
	}
	if !found || !declared.DeclaresAny {
		if len(entries) == 0 {
			return repository.RepoSelection{}, fmt.Errorf("--select listed needs workspaces in .gz-git.yaml or --repo / --repos-from entries")
		}
		return repository.RepoSelection{Listed: true, Paths: entries}, nil
	}

	if len(entries) == 0 {
		return repository.RepoSelection{Listed: true, Paths: declared.Paths, Prefixes: declared.Prefixes}, nil
	}

	selection := repository.RepoSelection{Listed: true}
	var undeclared []string
	for _, entry := range entries {
		switch {
		case !declared.Declares(entry):
			undeclared = append(undeclared, entry)
		case isDeclaredPrefix(declared, entry):
			selection.Prefixes = append(selection.Prefixes, entry)
		default:
			selection.Paths = append(selection.Paths, entry)
		}
	}
	if len(undeclared) > 0 {
		return repository.RepoSelection{}, fmt.Errorf("--select listed: %s not declared in %s workspaces; declare it there or use --select all; nothing was run",
			strings.Join(undeclared, ", "), declared.ConfigPath)
	}
	return selection, nil
}

func isDeclaredPrefix(declared config.DeclaredRepositories, entry string) bool {
	for _, p := range declared.Prefixes {
		if p == entry {
			return true
		}
	}
	return false
}

// selectionEntries collects, normalizes and de-duplicates the --repo and
// --repos-from entries, keeping their first-seen order.
func selectionEntries(flags *BulkCommandFlags) ([]string, error) {
	raw := append([]string{}, flags.Repos...)
	if flags.ReposFrom != "" {
		fromFile, err := readSelectionList(flags.ReposFrom)
		if err != nil {
			return nil, err
		}
		raw = append(raw, fromFile...)
	}

	seen := make(map[string]bool, len(raw))
	entries := make([]string, 0, len(raw))
	for _, entry := range raw {
		normalized, err := repository.NormalizeSelectionPath(entry)
		if err != nil {
			return nil, fmt.Errorf("--repo: %w", err)
		}
		if !seen[normalized] {
			seen[normalized] = true
			entries = append(entries, normalized)
		}
	}
	return entries, nil
}

// readSelectionList reads one repository path per line. Blank lines and lines
// starting with # are skipped.
func readSelectionList(source string) ([]string, error) {
	if source == "-" {
		stdinSelection.once.Do(func() {
			stdinSelection.entries, stdinSelection.err = scanSelectionList(stdinReaderForSelection, source)
		})
		return stdinSelection.entries, stdinSelection.err
	}
	file, err := os.Open(source) // #nosec G304 -- the operator names the list file.
	if err != nil {
		return nil, fmt.Errorf("--repos-from: %w", err)
	}
	defer file.Close()
	return scanSelectionList(file, source)
}

func scanSelectionList(reader io.Reader, source string) ([]string, error) {
	var entries []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("--repos-from %s: %w", source, err)
	}
	return entries, nil
}
