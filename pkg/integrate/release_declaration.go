// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// A release reads the repository-root declaration from commits, never from
// the working tree, and splits it by role:
//
//   - The target commit supplies the integration branch that gates the
//     release (which branch may be promoted). It is read leniently: only
//     branch.integrationBranch is taken and the removed-key guard is skipped,
//     because that declaration is the pre-release state the release replaces.
//     A key the source already deleted must not block the only path that
//     delivers the deletion (ISSUE-109). The source's own integrationBranch
//     never gates: a source branch declaring itself the integration branch
//     would otherwise authorize its own promotion. A target that declares no
//     integration branch falls back to the heuristic, which refuses a release.
//   - The source commit supplies everything else (task patterns, reclaim,
//     provenance) and is strictly validated: the removed-key guard and every
//     other declaration check apply, so a release cannot carry a removed key
//     forward into the target it fast-forwards.
//
// Both reads happen once, after the fetch, at the commits the plan resolved
// (resolveTarget). Nothing reads a release declaration before the fetch: a
// local ref may be stale, and reading it would judge a commit nobody
// approved. A release therefore needs an explicit --target, since deriving
// the default target would need the declaration first. Run's reclaim and
// evidence provenance reuse the source declaration Check read when they ask
// for the same commit. Integration into the integration branch keeps reading
// the working tree.

// ErrReleaseNeedsTarget reports a release without an explicit target. The
// release declarations are read at the fetched commits, so there is no
// declaration to derive a target from before the fetch.
var ErrReleaseNeedsTarget = errors.New("--release requires an explicit --target <default branch>")

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

// loadTargetIntegrationBranch reads the integration branch declared at the
// release target commit, leniently (config.CommittedIntegrationBranch). It
// returns the declaring root file, or "" when the commit has none.
func loadTargetIntegrationBranch(ctx context.Context, g gitRepo, targetSHA string) (config.BranchList, string, error) {
	files, err := committedDeclarationFiles(ctx, g, targetSHA)
	if err != nil {
		return nil, "", fmt.Errorf("release target declaration: %w", err)
	}
	integration, source, err := config.CommittedIntegrationBranch(targetSHA, files)
	if err != nil {
		return nil, source, fmt.Errorf("release target declaration: %w", err)
	}
	return integration, source, nil
}

// checkedPlanDeclaration is loadPlanDeclaration for a step after Check. A
// release asking for the commit Check already read reuses that declaration
// instead of reading the commit again; a commit is immutable, so the result
// is the same.
func checkedPlanDeclaration(ctx context.Context, g gitRepo, root string, check *CheckReport, sourceSHA string) (config.TaskPatternDecl, error) {
	release := check != nil && check.Plan.Release
	if release && sourceSHA != "" && check.Declaration.Commit == sourceSHA {
		return check.Declaration, nil
	}
	return loadPlanDeclaration(ctx, g, root, release, sourceSHA)
}

// emptyTreeIDs are git's well-known empty tree objects by object-ID length
// (SHA-1, SHA-256). Diffing a commit against the empty tree lists its files
// with pathspec glob magic, which ls-tree does not support.
var emptyTreeIDs = map[int]string{
	40: "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
	64: "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321",
}

// committedDeclarationFiles returns the .gz-git.{yaml,yml,json} blobs of
// commit. Pathspecs restrict the listing to those names at any depth, so its
// output does not grow with the rest of the tree. Blobs are read by object
// ID, so no committed path reaches a git argument. A repo-root declaration
// committed as a symbolic link is refused: its bytes are a link target, not
// the declaration the release installs.
func committedDeclarationFiles(ctx context.Context, g gitRepo, commit string) ([]config.CommittedConfigFile, error) {
	if normalized, err := NormalizeExpectSource(commit); err != nil || normalized != commit {
		return nil, fmt.Errorf("%q is not a full commit ID", commit)
	}
	emptyTree, ok := emptyTreeIDs[len(commit)]
	if !ok {
		return nil, fmt.Errorf("%q has no known empty tree for a %d-character object ID", commit, len(commit))
	}
	args := []string{"diff-tree", "-r", "-z", "--no-renames", emptyTree, commit, "--"}
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		args = append(args, ":(glob)**/"+config.ProjectConfigFileName+ext)
	}
	res, err := g.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git diff-tree %s: %s", commit, strings.TrimSpace(res.Stderr))
	}
	// -z raw output alternates ":<old mode> <new mode> <old oid> <new oid> A"
	// and the path.
	fields := strings.Split(strings.TrimSuffix(res.Stdout, "\x00"), "\x00")
	var files []config.CommittedConfigFile
	for i := 0; i+1 < len(fields); i += 2 {
		meta, rel := strings.Fields(strings.TrimPrefix(fields[i], ":")), fields[i+1]
		if len(meta) != 5 {
			return nil, fmt.Errorf("git diff-tree %s: unexpected entry %q", commit, fields[i])
		}
		mode, oid := meta[1], meta[3]
		switch mode {
		case "100644", "100755":
		case "120000":
			if !strings.Contains(rel, "/") {
				return nil, fmt.Errorf("%s at commit %s is a symbolic link; commit the declaration as a regular file", rel, config.ShortCommit(commit))
			}
			continue
		default:
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
