// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// A release reads the repository-root declaration from the release source
// commit, not from the working tree. The release fast-forwards the target to
// exactly that commit, so afterwards the target carries the declaration
// committed there; the working-tree copy on the target checkout is the
// pre-release state the release replaces. Reading the working tree instead
// let a key the source had already deleted block the only path that delivers
// the deletion (ISSUE-109). The committed file still passes the removed-key
// guard and every other declaration check, so a release cannot carry a
// removed key forward. Every release-path loader (Check's first read and its
// post-fetch revalidation, Run's heuristic-target precheck, reclaim, and the
// evidence provenance) goes through loadPlanDeclaration so they compare like
// with like. Integration into the integration branch keeps reading the
// working tree.

// loadPlanDeclaration reads the declaration that governs an integration:
// the one committed at sourceSHA for a release, the working tree otherwise.
func loadPlanDeclaration(ctx context.Context, g gitRepo, root string, release bool, sourceSHA string) (config.TaskPatternDecl, error) {
	if !release {
		return config.LoadRepoRootTaskPattern(root)
	}
	files, err := committedDeclarationFiles(ctx, g, sourceSHA)
	if err != nil {
		return config.TaskPatternDecl{}, fmt.Errorf("release source declaration: %w", err)
	}
	decl, err := config.LoadCommittedTaskPattern(root, sourceSHA, files)
	if err != nil {
		return decl, fmt.Errorf("release source declaration: %w", err)
	}
	return decl, nil
}

// initialDeclaration is the read before the fetch. A release reads the
// source as it resolves locally then: the approved --expect-source commit
// when it is present, otherwise the source ref. Check's post-fetch
// revalidation re-reads at the fetched source and refuses a declaration
// that changed in between.
func initialDeclaration(ctx context.Context, g gitRepo, root string, opts CheckOptions) (config.TaskPatternDecl, error) {
	if !opts.Release {
		return config.LoadRepoRootTaskPattern(root)
	}
	source, err := localReleaseSource(ctx, g, opts)
	if err != nil {
		return config.TaskPatternDecl{}, err
	}
	return loadPlanDeclaration(ctx, g, root, true, source)
}

func localReleaseSource(ctx context.Context, g gitRepo, opts CheckOptions) (string, error) {
	if opts.ExpectSource != "" {
		sha, ok, err := g.revParse(ctx, opts.ExpectSource)
		if err != nil {
			return "", err
		}
		if ok {
			return sha, nil
		}
	}
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		var err error
		branch, err = g.currentBranch(ctx)
		if err != nil {
			return "", err
		}
	}
	if err := gitcmd.SanitizeBranchName(branch); err != nil {
		return "", fmt.Errorf("invalid branch: %w", err)
	}
	sha, ok, err := g.revParse(ctx, branch)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("release source %s is not available locally; fetch it before releasing", branch)
	}
	return sha, nil
}

// committedDeclarationFiles returns the .gz-git.{yaml,yml,json} blobs of
// commit. Blobs are read by object ID, so no committed path reaches a git
// argument. A repo-root declaration committed as a symbolic link is refused:
// its bytes are a link target, not the declaration the release installs.
func committedDeclarationFiles(ctx context.Context, g gitRepo, commit string) ([]config.CommittedConfigFile, error) {
	if _, err := NormalizeExpectSource(commit); err != nil {
		return nil, fmt.Errorf("release source %q is not a full commit ID", commit)
	}
	res, err := g.run(ctx, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git ls-tree %s: %s", commit, strings.TrimSpace(res.Stderr))
	}
	var files []config.CommittedConfigFile
	for _, entry := range strings.Split(res.Stdout, "\x00") {
		meta, rel, ok := strings.Cut(entry, "\t")
		if !ok || !config.IsProjectConfigName(path.Base(rel)) {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		mode, oid := fields[0], fields[2]
		if mode == "120000" {
			if !strings.Contains(rel, "/") {
				return nil, fmt.Errorf("%s at commit %s is a symbolic link; commit the declaration as a regular file", rel, config.ShortCommit(commit))
			}
			continue
		}
		blob, err := g.run(ctx, "cat-file", "blob", oid)
		if err != nil {
			return nil, err
		}
		if blob.ExitCode != 0 {
			return nil, fmt.Errorf("git cat-file blob %s (%s): %s", oid, rel, strings.TrimSpace(blob.Stderr))
		}
		files = append(files, config.CommittedConfigFile{Path: rel, Data: []byte(blob.Stdout)})
	}
	return files, nil
}
