// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// setupBulkParent creates a parent dir with one initialized git child.
func setupBulkParent(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "r1")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, parent, "init", "r1")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")
	return parent
}

func resetBulkOpFlags(t *testing.T) {
	t.Helper()
	prevQuiet, prevVerbose := quiet, verbose
	prevFetch, prevPull, prevPush, prevUpdate := fetchFlags, pullFlags, pushFlags, updateFlags
	t.Cleanup(func() {
		quiet, verbose = prevQuiet, prevVerbose
		fetchFlags, pullFlags, pushFlags, updateFlags = prevFetch, prevPull, prevPush, prevUpdate
	})
	quiet = false
	verbose = false
}

func TestRunFetchPullPushUpdateDryRun(t *testing.T) {
	resetBulkOpFlags(t)
	parent := setupBulkParent(t)

	fetchFlags = BulkCommandFlags{Depth: 1, Parallel: 2, DryRun: true, Format: "default"}
	captureStdout(t, func() {
		if err := runFetch(fetchCmd, []string{parent}); err != nil {
			t.Logf("fetch: %v", err)
		}
	})

	pullFlags = BulkCommandFlags{Depth: 1, Parallel: 2, DryRun: true, Format: "json"}
	captureStdout(t, func() {
		if err := runPull(pullCmd, []string{parent}); err != nil {
			t.Logf("pull: %v", err)
		}
	})

	pushFlags = BulkCommandFlags{Depth: 1, Parallel: 2, DryRun: true, Format: "default"}
	captureStdout(t, func() {
		if err := runPush(pushCmd, []string{parent}); err != nil {
			t.Logf("push: %v", err)
		}
	})

	updateFlags = BulkCommandFlags{Depth: 1, Parallel: 2, DryRun: true, Format: "compact"}
	captureStdout(t, func() {
		if err := runUpdate(updateCmd, []string{parent}); err != nil {
			t.Logf("update: %v", err)
		}
	})
}

func TestRunInfoTagStashDiff(t *testing.T) {
	resetBulkOpFlags(t)
	parent := setupBulkParent(t)

	captureStdout(t, func() {
		if err := runInfo(infoCmd, []string{parent}); err != nil {
			t.Logf("info: %v", err)
		}
	})

	tagList := findCommand(t, rootCmd, "tag", "list")
	captureStdout(t, func() {
		if err := runTagList(tagList, []string{parent}); err != nil {
			t.Logf("tag list: %v", err)
		}
	})

	stashList := findCommand(t, rootCmd, "stash", "list")
	captureStdout(t, func() {
		if stashList.RunE != nil {
			if err := stashList.RunE(stashList, []string{parent}); err != nil {
				t.Logf("stash list: %v", err)
			}
		}
	})

	captureStdout(t, func() {
		if err := runDiff(diffCmd, []string{parent}); err != nil {
			t.Logf("diff: %v", err)
		}
	})
}

func TestDisplayFetchPullPushResults(t *testing.T) {
	fetchRes := &repository.BulkFetchResult{
		TotalScanned: 1,
		Duration:     time.Millisecond,
		Summary:      map[string]int{"success": 1},
		Repositories: []repository.RepositoryFetchResult{
			{RelativePath: "r1", Status: "success", Message: "ok"},
		},
	}
	captureStdout(t, func() { displayFetchResults(fetchRes) })

	pullRes := &repository.BulkPullResult{
		TotalScanned: 1,
		Duration:     time.Millisecond,
		Summary:      map[string]int{"up-to-date": 1},
		Repositories: []repository.RepositoryPullResult{
			{RelativePath: "r1", Status: "up-to-date"},
		},
	}
	captureStdout(t, func() { displayPullResults(pullRes) })

	pushRes := &repository.BulkPushResult{
		TotalScanned: 1,
		Duration:     time.Millisecond,
		Summary:      map[string]int{"nothing-to-push": 1},
		Repositories: []repository.RepositoryPushResult{
			{RelativePath: "r1", Status: "nothing-to-push"},
		},
	}
	captureStdout(t, func() { displayPushResults(pushRes) })
}

func TestDisplayCleanResults(t *testing.T) {
	res := &repository.BulkCleanResult{
		TotalScanned: 1,
		Duration:     time.Millisecond,
		Summary:      map[string]int{"clean": 1},
		Repositories: []repository.RepositoryCleanResult{
			{RelativePath: "r1", Status: "clean"},
			{RelativePath: "r2", Status: "would-clean", FilesRemoved: []string{"a.tmp"}, FilesCount: 1},
			{RelativePath: "r3", Status: "error", Error: errors.New("boom")},
		},
	}
	for _, format := range []string{"default", "json", "llm"} {
		captureStdout(t, func() {
			displayCleanResults(res, true)
			displayCleanResultsDefault(res, true)
			displayCleanResultsVerbose(res, false)
			displayCleanResultsStructured(res, format)
		})
	}
}

func TestDisplayStatusResultsStructured(t *testing.T) {
	res := &repository.BulkStatusResult{
		TotalScanned: 1,
		Duration:     time.Millisecond,
		Summary:      map[string]int{"clean": 1},
		Repositories: []repository.RepositoryStatusResult{
			{RelativePath: "r1", Status: "clean", Branch: "main"},
		},
	}
	captureStdout(t, func() {
		displayStatusResultsStructured(res, "json")
		displayStatusResultsStructured(res, "llm")
	})
}

func TestPrintBulkTagResult(t *testing.T) {
	res := &repository.BulkTagResult{
		TotalScanned: 1,
		Duration:     time.Millisecond,
		Summary:      map[string]int{"success": 1},
		Repositories: []repository.RepositoryTagResult{
			{RelativePath: "r1", Status: "success", LatestTag: "v1.0.0", TagCount: 1},
		},
	}
	captureStdout(t, func() {
		printBulkTagResult(res, "list", false, "default")
		printBulkTagResult(res, "list", true, "json")
		displayTagResultsStructured(res, "list", "json")
	})
}
