// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

const siblingLeaseFile = ".gz-git-sibling-lease.json"

// SiblingLease is the receipt required to release an extracted dependency.
// Manifest covers every extracted regular file so release refuses tampering.
type SiblingLease struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Root        string `json:"root"`
	Destination string `json:"destination"`
	Source      string `json:"source"`
	OID         string `json:"oid"`
	Manifest    string `json:"manifest"`
}

// PersistSiblingLease stores an opaque receipt below the selected repository's
// Git common directory. The committed worktree never carries machine paths.
func PersistSiblingLease(ctx context.Context, repoRoot string, lease SiblingLease) (SiblingLease, error) {
	canonicalRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return lease, err
	}
	repoRoot = canonicalRoot
	if filepath.Clean(repoRoot) != lease.Root {
		return lease, fmt.Errorf("sibling lease belongs to another worktree")
	}
	g := newGitRepo(gitcmd.NewExecutor(), repoRoot)
	common, err := g.output(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return lease, err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repoRoot, common)
	}
	common = filepath.Clean(common)
	if lease.ID == "" {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return lease, err
		}
		lease.ID = hex.EncodeToString(raw)
	}
	if !validSiblingLeaseID(lease.ID) {
		return lease, fmt.Errorf("invalid sibling lease ID")
	}
	dir := filepath.Join(common, "sibling-provision-leases")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return lease, err
	}
	if err := rejectPathSymlinks(dir); err != nil {
		return lease, err
	}
	path := filepath.Join(dir, lease.ID+".json")
	if _, err := os.Lstat(path); err == nil {
		return lease, fmt.Errorf("sibling lease already exists")
	} else if !os.IsNotExist(err) {
		return lease, err
	}
	b, err := json.Marshal(lease)
	if err != nil {
		return lease, err
	}
	// #nosec G304 -- opaque hex lease ID under verified Git common-directory registry
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return lease, err
	}
	_, writeErr := f.Write(b)
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return lease, errors.Join(err, os.Remove(path))
	}
	return lease, nil
}

// ReleasePersistedSiblings releases every receipt bound to taskRoot. It never
// scans the task directory itself: the common-dir registry is the authority.
func ReleasePersistedSiblings(ctx context.Context, repoRoot, taskRoot string) error {
	g := newGitRepo(gitcmd.NewExecutor(), repoRoot)
	common, err := g.output(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repoRoot, common)
	}
	dir := filepath.Join(filepath.Clean(common), "sibling-provision-leases")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("invalid sibling lease receipt")
		}
		// #nosec G304 -- regular receipt under verified repository common directory
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var lease SiblingLease
		if err := json.Unmarshal(data, &lease); err != nil {
			return err
		}
		if lease.Root != taskRoot {
			continue
		}
		if err := ReleaseSibling(lease); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// LoadPersistedSiblingLease resolves one opaque receipt from this repository's
// common Git directory, never from the task worktree.
func LoadPersistedSiblingLease(ctx context.Context, repoRoot, id string) (SiblingLease, string, error) {
	if !validSiblingLeaseID(id) {
		return SiblingLease{}, "", fmt.Errorf("invalid sibling lease ID")
	}
	g := newGitRepo(gitcmd.NewExecutor(), repoRoot)
	common, err := g.output(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return SiblingLease{}, "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repoRoot, common)
	}
	path := filepath.Join(filepath.Clean(common), "sibling-provision-leases", id+".json")
	info, err := os.Lstat(path)
	if err != nil {
		return SiblingLease{}, "", err
	}
	if !info.Mode().IsRegular() {
		return SiblingLease{}, "", fmt.Errorf("invalid sibling lease receipt")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return SiblingLease{}, "", err
	}
	var lease SiblingLease
	if err := json.Unmarshal(b, &lease); err != nil {
		return SiblingLease{}, "", err
	}
	if lease.ID != id {
		return SiblingLease{}, "", fmt.Errorf("sibling lease receipt identity mismatch")
	}
	return lease, path, nil
}

func validSiblingLeaseID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		valid := r >= '0' && r <= '9' || r >= 'a' && r <= 'f'
		if !valid {
			return false
		}
	}
	return true
}

// InspectPersistedSiblingLeases validates provider-owned leases before resuming.
func InspectPersistedSiblingLeases(ctx context.Context, repoRoot string) ([]SiblingLease, error) {
	canonicalRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return nil, err
	}
	repoRoot = canonicalRoot
	g := newGitRepo(gitcmd.NewExecutor(), repoRoot)
	common, err := g.output(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repoRoot, common)
	}
	entries, err := os.ReadDir(filepath.Join(filepath.Clean(common), "sibling-provision-leases"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]SiblingLease, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lease, _, err := LoadPersistedSiblingLease(ctx, repoRoot, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		if filepath.Clean(lease.Root) != filepath.Clean(repoRoot) {
			continue
		}
		if err := inspectSiblingLease(lease); err != nil {
			return nil, err
		}
		out = append(out, lease)
	}
	if len(out) == 0 {
		return out, nil
	}
	if err := validateSiblingLeaseSet(ctx, repoRoot, out); err != nil {
		return nil, err
	}
	return out, nil
}

func inspectSiblingLease(lease SiblingLease) error {
	marker := filepath.Join(lease.Destination, siblingLeaseFile)
	markerInfo, markerErr := os.Lstat(marker)
	if markerErr != nil || !markerInfo.Mode().IsRegular() {
		return errors.Join(fmt.Errorf("invalid sibling lease marker"), markerErr)
	}
	// #nosec G304 -- regular owned lease marker checked immediately above
	markerData, markerErr := os.ReadFile(marker)
	var markerRecord siblingLeaseRecord
	if markerErr != nil || json.Unmarshal(markerData, &markerRecord) != nil || markerRecord.Root != lease.Root || markerRecord.Name != lease.Name || markerRecord.OID != lease.OID || markerRecord.Manifest != lease.Manifest {
		return fmt.Errorf("sibling lease marker identity mismatch")
	}
	got, err := siblingManifest(lease.Destination)
	if err != nil || got != lease.Manifest {
		return errors.Join(fmt.Errorf("sibling lease changed; refusing resume"), err)
	}
	return nil
}

func validateSiblingLeaseSet(ctx context.Context, repoRoot string, out []SiblingLease) error {
	decl, present, err := LoadSiblingProvision(repoRoot)
	if err != nil {
		return err
	}
	if !present || len(out) != len(decl.Siblings) {
		return fmt.Errorf("sibling lease set is incomplete or declaration changed")
	}
	seen := make(map[string]bool, len(out))
	for _, dep := range decl.Siblings {
		var lease *SiblingLease
		for i := range out {
			if out[i].Name == dep.Name {
				if lease != nil {
					return fmt.Errorf("duplicate sibling lease")
				}
				lease = &out[i]
			}
		}
		if lease == nil || seen[dep.Name] {
			return fmt.Errorf("sibling lease set is incomplete")
		}
		seen[dep.Name] = true
		if lease.Destination != filepath.Clean(filepath.Join(repoRoot, dep.Path)) {
			return fmt.Errorf("sibling destination declaration changed")
		}
		if err := rejectPathSymlinks(lease.Destination); err != nil {
			return err
		}
		_, expected, err := resolveSiblingSource(ctx, lease.Source, dep)
		if err != nil {
			return err
		}
		if expected != lease.OID {
			return fmt.Errorf("sibling source revision changed; refusing resume")
		}
	}
	return nil
}

type siblingLeaseRecord struct {
	Root     string `json:"root"`
	Name     string `json:"name"`
	OID      string `json:"oid"`
	Manifest string `json:"manifest"`
}

// LoadSiblingProvision reads the repository-owned declaration without falling
// back to a parent or user config layer.
func LoadSiblingProvision(repoRoot string) (config.SiblingProvision, bool, error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		// #nosec G304 -- fixed declaration filename under selected repository root
		data, err := os.ReadFile(filepath.Join(repoRoot, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return config.SiblingProvision{}, false, err
		}
		return config.ParseSiblingProvisionDocument(data, strings.HasSuffix(name, ".json"))
	}
	return config.SiblingProvision{}, false, nil
}

// ProvisionSibling extracts one archived local source commit at the declared
// relative path. The caller supplies the verified primary source checkout;
// this boundary keeps machine-specific checkout discovery outside committed
// repository configuration.
func ProvisionSibling(ctx context.Context, repoRoot, source string, dep config.SiblingDependency) (SiblingLease, error) {
	if err := config.ValidateSiblingProvision(config.SiblingProvision{Version: 1, Siblings: []config.SiblingDependency{dep}}); err != nil {
		return SiblingLease{}, err
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return SiblingLease{}, err
	}
	if err := rejectPathSymlinks(root); err != nil {
		return SiblingLease{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return SiblingLease{}, err
	}
	dest := filepath.Clean(filepath.Join(root, dep.Path))
	if filepath.Dir(dest) != filepath.Dir(root) {
		return SiblingLease{}, fmt.Errorf("sibling destination escapes worktree parent")
	}
	if _, err := os.Lstat(dest); err == nil {
		return SiblingLease{}, fmt.Errorf("sibling destination already exists: %s", dest)
	} else if !os.IsNotExist(err) {
		return SiblingLease{}, err
	}
	g, oid, err := resolveSiblingSource(ctx, source, dep)
	if err != nil {
		return SiblingLease{}, err
	}
	data, err := gitArchive(ctx, g, oid)
	if err != nil {
		return SiblingLease{}, err
	}
	data, err = materializeSiblingAgentAliases(ctx, data)
	if err != nil {
		return SiblingLease{}, err
	}
	if err := extractGitArchive(ctx, dest, data); err != nil {
		return SiblingLease{}, err
	}
	manifest, err := siblingManifest(dest)
	if err != nil {
		return SiblingLease{}, errors.Join(err, removeOwnedSiblingTree(dest))
	}
	record := siblingLeaseRecord{Root: root, Name: dep.Name, OID: oid, Manifest: manifest}
	b, err := json.Marshal(record)
	if err != nil {
		return SiblingLease{}, errors.Join(err, removeOwnedSiblingTree(dest))
	}
	marker, err := os.OpenFile(filepath.Join(dest, siblingLeaseFile), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, err = marker.Write(b)
		closeErr := marker.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return SiblingLease{}, errors.Join(err, removeOwnedSiblingTree(dest))
	}
	return SiblingLease{Name: dep.Name, Root: root, Destination: dest, Source: g.dir, OID: oid, Manifest: manifest}, nil
}

func resolveSiblingSource(ctx context.Context, source string, dep config.SiblingDependency) (gitRepo, string, error) {
	if err := rejectPathSymlinks(source); err != nil {
		return gitRepo{}, "", fmt.Errorf("sibling source: %w", err)
	}
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return gitRepo{}, "", err
	}
	g := newGitRepo(gitcmd.NewExecutor(), source)
	actual, err := g.toplevel(ctx)
	if err != nil || actual != source {
		return gitRepo{}, "", fmt.Errorf("sibling source must be its primary repository root")
	}
	common, err := g.output(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return gitRepo{}, "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(source, common)
	}
	if filepath.Clean(common) != filepath.Join(source, ".git") {
		return gitRepo{}, "", fmt.Errorf("sibling source must be a primary checkout")
	}
	if err := requireCanonicalRemote(ctx, g, dep.Name, dep.Remote); err != nil {
		return gitRepo{}, "", err
	}
	status, err := isolatedGitRawOutput(ctx, g, "-c", "core.fsmonitor=false", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || !siblingSourceStatusClean(status) {
		if err != nil {
			return gitRepo{}, "", err
		}
		return gitRepo{}, "", fmt.Errorf("%s checkout is dirty", dep.Name)
	}
	oid, ok, err := g.revParse(ctx, "HEAD")
	if err != nil || !ok || !fullObjectID(oid) {
		return gitRepo{}, "", fmt.Errorf("%s HEAD is unavailable", dep.Name)
	}
	if dep.Revision == "source-tip" {
		remoteOID, exists, remoteErr := g.revParse(ctx, "refs/remotes/origin/"+dep.SourceBranch)
		if remoteErr != nil || !exists || remoteOID != oid {
			return gitRepo{}, "", fmt.Errorf("%s local HEAD does not match origin/%s", dep.Name, dep.SourceBranch)
		}
	}
	if dep.Revision != "source-tip" {
		pinned, exists, resolveErr := g.revParse(ctx, dep.Revision+"^{commit}")
		if resolveErr != nil || !exists || pinned != dep.Revision {
			return gitRepo{}, "", fmt.Errorf("%s declared commit is unavailable", dep.Name)
		}
		oid = pinned
	}
	return g, oid, nil
}

// The archive contains only committed objects. Admit the observed local agent
// metadata omitted from that archive, without allowing user excludes to hide
// source changes or widening the independent flow-snapshot policy.
func siblingSourceStatusClean(status string) bool {
	return statusCleanExceptUntracked(status, func(name string) bool {
		if isCELogRuntimeArtifactPath(name) || name == ".ce/task-runtime.yaml" {
			return true
		}
		const prefix = ".agent-mesh/checkpoints/"
		const suffix = ".checkpoint.json"
		if !strings.HasPrefix(name, prefix) {
			return false
		}
		base := strings.TrimPrefix(name, prefix)
		return !strings.Contains(base, "/") && strings.HasSuffix(base, suffix) && strings.TrimSuffix(base, suffix) != ""
	})
}

// DiscoverSiblingPrimary finds the dependency next to the selected repository's
// registered primary checkout. When that adjacent checkout is absent, it may
// use the exact same-name path declared as a git workspace by the primary
// checkout. It intentionally never walks arbitrary parents or discovers an
// undeclared workspace.
func DiscoverSiblingPrimary(ctx context.Context, repoRoot string, dep config.SiblingDependency) (string, error) {
	g := newGitRepo(gitcmd.NewExecutor(), repoRoot)
	trees, err := g.listWorktrees(ctx)
	if err != nil || len(trees) == 0 {
		return "", fmt.Errorf("list selected worktrees: %w", err)
	}
	primary := trees[0].Path
	if err := rejectPathSymlinks(primary); err != nil {
		return "", err
	}
	source := filepath.Join(filepath.Dir(primary), dep.Name)
	if err := rejectPathSymlinks(source); err != nil {
		return "", err
	}
	if _, err := os.Lstat(source); err == nil {
		return validateSiblingPrimary(ctx, source, dep.Name)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	declared, err := config.DeclaredWorkspaceClones(primary)
	if err != nil {
		return "", fmt.Errorf("read declared workspaces: %w", err)
	}
	declaredRemote, ok := declared[dep.Name]
	if !ok {
		return "", fmt.Errorf("declared sibling primary %s is unavailable", dep.Name)
	}
	if normalizeCloneURL(declaredRemote) != normalizeCloneURL(dep.Remote) {
		return "", fmt.Errorf("declared workspace %s does not match sibling remote", dep.Name)
	}
	source = filepath.Join(primary, filepath.FromSlash(dep.Name))
	if err := rejectPathSymlinks(source); err != nil {
		return "", err
	}
	return validateSiblingPrimary(ctx, source, dep.Name)
}

func validateSiblingPrimary(ctx context.Context, source, name string) (string, error) {
	sg := newGitRepo(gitcmd.NewExecutor(), source)
	actual, err := sg.toplevel(ctx)
	if err != nil || actual != source {
		return "", fmt.Errorf("declared sibling primary %s is unavailable", name)
	}
	common, err := sg.output(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(source, common)
	}
	if filepath.Clean(common) != filepath.Join(source, ".git") {
		return "", fmt.Errorf("declared sibling %s is not a primary checkout", name)
	}
	return source, nil
}

// ReleaseSibling removes only an unchanged destination created by ProvisionSibling.
func ReleaseSibling(lease SiblingLease) error {
	if lease.Root == "" || lease.Destination == "" || filepath.Base(lease.Destination) == "." {
		return fmt.Errorf("sibling lease destination is invalid")
	}
	root, err := filepath.Abs(lease.Root)
	if err != nil {
		return err
	}
	if filepath.Dir(lease.Destination) != filepath.Dir(root) || filepath.Clean(lease.Destination) != lease.Destination {
		return fmt.Errorf("sibling lease destination escapes task worktree parent")
	}
	if root == filepath.Dir(root) || lease.Destination == root {
		return fmt.Errorf("sibling destination is not distinct from the task root")
	}
	if err := rejectPathSymlinks(filepath.Dir(root)); err != nil {
		return err
	}
	info, err := os.Lstat(lease.Destination)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("sibling destination is not an owned directory")
	}
	marker := filepath.Join(lease.Destination, siblingLeaseFile)
	markerInfo, err := os.Lstat(marker)
	if err != nil {
		return err
	}
	if !markerInfo.Mode().IsRegular() {
		return fmt.Errorf("sibling lease marker is not a regular file")
	}
	// #nosec G304 -- regular owned lease marker checked immediately above
	b, err := os.ReadFile(marker)
	if err != nil {
		return err
	}
	var record siblingLeaseRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return err
	}
	if record.Root != root || record.Name != lease.Name || record.OID != lease.OID || record.Manifest != lease.Manifest {
		return fmt.Errorf("sibling lease does not match destination")
	}
	got, paths, err := siblingManifestInventory(lease.Destination)
	if err != nil {
		return err
	}
	if got != lease.Manifest {
		return fmt.Errorf("sibling destination changed; refusing release")
	}
	return removeSiblingInventory(paths)
}

// Enumerate before unlinking; rmdir then refuses files added after enumeration.
// A symlink or special file refuses cleanup rather than widening its scope.
func removeOwnedSiblingTree(root string) error {
	var paths []string
	err := filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("owned sibling contains unsupported entry")
		}
		paths = append(paths, name)
		return nil
	})
	if err != nil {
		return err
	}
	return removeSiblingInventory(paths)
}

func removeSiblingInventory(paths []string) error {
	for i := len(paths) - 1; i >= 0; i-- {
		if err := os.Remove(paths[i]); err != nil {
			return err
		}
	}
	return nil
}

func siblingManifest(root string) (string, error) {
	digest, _, err := siblingManifestInventory(root)
	return digest, err
}

func siblingManifestInventory(root string) (digest string, inventory []string, resultErr error) {
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return "", nil, err
	}
	defer rootFS.Close()
	var paths []string
	var entries []string
	err = filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		paths = append(paths, name)
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == siblingLeaseFile {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("sibling contains symlink: %s", rel)
		}
		if entry.IsDir() {
			entries = append(entries, "d:"+filepath.ToSlash(rel))
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("sibling contains unsupported file: %s", rel)
		}
		f, err := rootFS.Open(rel)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("f:%s:%o:%s", filepath.ToSlash(rel), info.Mode().Perm(), hex.EncodeToString(h.Sum(nil))))
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	sort.Strings(entries)
	h := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(h[:]), paths, nil
}
