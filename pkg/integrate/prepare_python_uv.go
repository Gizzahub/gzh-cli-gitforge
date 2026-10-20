// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

// The infra-ops profile is deliberately a closed executor.  Its manifest
// supplies only the two immutable child revisions; remotes, paths and invoked
// commands live here so a repository declaration cannot turn preparation into
// an arbitrary command runner.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

type prepareMakeEnvKey struct{}

func withInfraOpsPrepareMakeEnv(ctx context.Context, dir, profile string) context.Context {
	if profile != config.PrepareProfileInfraOpsPythonUVV1 {
		return ctx
	}
	environment := filepath.Join(infraOpsOwnedRoot(dir), "venv")
	return context.WithValue(ctx, prepareMakeEnvKey{}, []string{"UV_PROJECT_ENVIRONMENT=" + environment, "VIRTUAL_ENV=" + environment, "UV_NO_SYNC=1"})
}

func prepareMakeEnv(ctx context.Context) []string {
	values, ok := ctx.Value(prepareMakeEnvKey{}).([]string)
	if !ok {
		return nil
	}
	return append([]string(nil), values...)
}

const (
	infraOpsPythonUVV1     = config.PrepareProfileInfraOpsPythonUVV1
	infraOpsInputsPath     = "scripts/worktree-env-inputs.json"
	infraOpsInputsMaxSize  = 64 << 10
	infraOpsPrepareRoot    = "infra-ops-python-uv-v1"
	infraOpsPrepareTimeout = 30 * time.Minute
)

func infraOpsOwnedRoot(dir string) string {
	return filepath.Join(filepath.Dir(dir), infraOpsPrepareRoot, filepath.Base(dir))
}

var infraOpsDependencies = []struct{ name, remote string }{
	{"sbkube", "git@github.com:ScriptonBasestar/sb-kube-app-manager.git"},
	{"cloud-script-transformer", "ssh://git@gitlab.polypia.net:2224/scripton-cloud/cwrapper/cloud-script-transformer.git"},
}

type infraOpsInputManifest struct {
	SchemaVersion int `json:"schema_version"`
	Dependencies  []struct {
		Name     string `json:"name"`
		Revision string `json:"revision"`
	} `json:"dependencies"`
}

// snapshotInfraOpsPythonUVInputs reads one commit into an immutable snapshot.
// Source and target normally use their own snapshots; first adoption is explicit.
func snapshotInfraOpsPythonUVInputs(ctx context.Context, g gitRepo, sourceSHA string) ([]prepareInput, error) {
	entry, present, err := g.treeEntry(ctx, sourceSHA, infraOpsInputsPath)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf("%s is required by infra-ops-python-uv-v1", infraOpsInputsPath)
	}
	if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return nil, fmt.Errorf("%s must be a regular file", infraOpsInputsPath)
	}
	size, err := g.objectSize(ctx, entry.OID)
	if err != nil {
		return nil, err
	}
	if size < 1 || size > infraOpsInputsMaxSize {
		return nil, fmt.Errorf("%s has invalid size", infraOpsInputsPath)
	}
	data, _, err := g.showFile(ctx, sourceSHA, infraOpsInputsPath)
	if err != nil {
		return nil, err
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", infraOpsInputsPath, err)
	}
	var manifest infraOpsInputManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", infraOpsInputsPath, err)
	}
	if manifest.SchemaVersion != 1 || len(manifest.Dependencies) != len(infraOpsDependencies) {
		return nil, fmt.Errorf("%s has unsupported schema or dependencies", infraOpsInputsPath)
	}
	inputs := []prepareInput{{Name: "manifest", OID: entry.OID, data: data}}
	for i, spec := range infraOpsDependencies {
		dependency := manifest.Dependencies[i]
		if dependency.Name != spec.name || !fullSHA1(dependency.Revision) {
			return nil, fmt.Errorf("%s has invalid dependency at index %d", infraOpsInputsPath, i)
		}
		inputs = append(inputs, prepareInput{Name: dependency.Name, OID: dependency.Revision})
	}
	return inputs, nil
}

func snapshotPrepareLockInputs(ctx context.Context, g gitRepo, profile, sha, side string) ([]prepareInput, error) {
	if profile != infraOpsPythonUVV1 {
		return nil, nil
	}
	var inputs []prepareInput
	for _, name := range []string{"pyproject.toml", "uv.lock", ".python-version", "library/ga4-admin/pnpm-lock.yaml", "library/m365ctl/pnpm-lock.yaml"} {
		entry, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return nil, err
		}
		if !present || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return nil, fmt.Errorf("%s lock input is not a regular file", name)
		}
		inputs = append(inputs, prepareInput{Name: side + ":" + name, OID: entry.OID})
	}
	return inputs, nil
}

func fullSHA1(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func prepareInfraOpsPythonUV(ctx context.Context, g gitRepo, dir string, inputs []prepareInput, guard preparedRefGuard) error {
	if len(inputs) < 3 || inputs[0].Name != "manifest" {
		return fmt.Errorf("infra-ops preparation has incomplete snapshot")
	}
	before, err := guard.snapshot(ctx, g)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, infraOpsPrepareTimeout)
	defer cancel()
	if err := validateInfraOpsProjectShape(dir); err != nil {
		return err
	}
	ownedRoot := infraOpsOwnedRoot(dir)
	if err := initializeInfraOpsResources(runCtx, dir, ownedRoot); err != nil {
		return err
	}
	if err := prepareInfraOpsChildren(runCtx, g, dir, ownedRoot, inputs); err != nil {
		return err
	}
	if err := prepareInfraOpsEnvironments(runCtx, dir, ownedRoot); err != nil {
		return err
	}
	if runCtx.Err() != nil {
		return fmt.Errorf("infra-ops preparation timed out")
	}
	if err := before.verify(ctx, g, dir); err != nil {
		return err
	}
	if err := validateInfraOpsCodesLayout(runCtx, dir, true); err != nil {
		return err
	}
	for _, spec := range infraOpsDependencies {
		actual, err := os.Readlink(filepath.Join(dir, "codes", spec.name))
		if err != nil || actual != filepath.Join(ownedRoot, "deps", spec.name) {
			return fmt.Errorf("preparation changed the owned dependency link: %s", spec.name)
		}
	}
	raw, err := runInfraOpsCommandOutput(ctx, dir, nil, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching")
	if err != nil {
		return fmt.Errorf("inspect prepared tree: %w", err)
	}
	return validatePreparedStatusRecords([]byte(raw), isInfraOpsPreparedPath)
}

func initializeInfraOpsResources(ctx context.Context, dir, ownedRoot string) error {
	if err := rejectPathSymlinks(filepath.Join(dir, "codes")); err != nil {
		return err
	}
	for _, packagePath := range []string{"library/ga4-admin", "library/m365ctl"} {
		if err := rejectPathSymlinkChain(dir, packagePath); err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Join(dir, packagePath, "node_modules")); !os.IsNotExist(err) {
			if err == nil {
				return fmt.Errorf("refusing pre-existing Node resource at %s", packagePath)
			}
			return err
		}
	}
	if err := rejectPathSymlinks(filepath.Dir(ownedRoot)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "codes"), 0o750); err != nil {
		return err
	}
	if err := validateInfraOpsCodesLayout(ctx, dir, false); err != nil {
		return err
	}
	if _, err := os.Lstat(ownedRoot); !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("refusing pre-existing preparation resource at %s", ownedRoot)
		}
		return err
	}
	if err := os.MkdirAll(ownedRoot, 0o750); err != nil {
		return err
	}
	return nil
}

func prepareInfraOpsChildren(ctx context.Context, g gitRepo, dir, ownedRoot string, inputs []prepareInput) error {
	for i, spec := range infraOpsDependencies {
		input := inputs[i+1]
		if input.Name != spec.name || !fullSHA1(input.OID) {
			return fmt.Errorf("infra-ops preparation has invalid snapshot for %s", spec.name)
		}
		destination := filepath.Join(ownedRoot, "deps", spec.name)
		link := filepath.Join(dir, "codes", spec.name)
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			if err == nil {
				return fmt.Errorf("refusing pre-existing dependency link at %s", link)
			}
			return err
		}
		if err := runInfraOpsCommand(ctx, dir, nil, "git", "clone", "--no-checkout", spec.remote, destination); err != nil {
			return fmt.Errorf("clone %s: %w", spec.name, err)
		}
		if err := runInfraOpsCommand(ctx, destination, nil, "git", "fetch", "--no-tags", "origin", input.OID); err != nil {
			return fmt.Errorf("fetch pinned %s: %w", spec.name, err)
		}
		if err := runInfraOpsCommand(ctx, destination, nil, "git", "checkout", "--detach", input.OID); err != nil {
			return fmt.Errorf("checkout pinned %s: %w", spec.name, err)
		}
		if err := verifyInfraOpsChild(ctx, g, destination, input.OID, spec.name, spec.remote); err != nil {
			return err
		}
		if err := os.Symlink(destination, link); err != nil {
			return fmt.Errorf("link %s: %w", spec.name, err)
		}
	}
	return nil
}

func prepareInfraOpsEnvironments(ctx context.Context, dir, ownedRoot string) error {
	environment := filepath.Join(ownedRoot, "venv")
	if err := os.MkdirAll(filepath.Dir(environment), 0o750); err != nil {
		return err
	}
	uvEnv := []string{"UV_PROJECT_ENVIRONMENT=" + environment, "VIRTUAL_ENV=" + environment, "UV_NO_SYNC=0"}
	if err := runInfraOpsCommand(ctx, dir, uvEnv, "uv", "sync", "--frozen"); err != nil {
		return fmt.Errorf("uv sync --frozen: %w", err)
	}
	if err := rejectPathSymlinks(environment); err != nil {
		return err
	}
	if err := requireRegularPreparationFile(filepath.Join(environment, "pyvenv.cfg")); err != nil {
		return fmt.Errorf("uv did not create the owned environment: %w", err)
	}
	for _, packagePath := range []string{"library/ga4-admin", "library/m365ctl"} {
		if err := runInfraOpsPnpmFrozenInstall(ctx, filepath.Join(dir, packagePath)); err != nil {
			return fmt.Errorf("prepare %s: %w", packagePath, err)
		}
		if err := rejectPathSymlinkChain(dir, filepath.Join(packagePath, "node_modules")); err != nil {
			return err
		}
		if err := requireRegularPreparationFile(filepath.Join(dir, packagePath, "node_modules", ".modules.yaml")); err != nil {
			return fmt.Errorf("pnpm did not create the owned package environment: %w", err)
		}
	}
	return nil
}

// infraOpsPnpmInstallTimeout bounds the frozen-lockfile install. Dependency
// installation is network-bound and slower than a profile's other steps.
const infraOpsPnpmInstallTimeout = 10 * time.Minute

func runInfraOpsPnpmFrozenInstall(ctx context.Context, dir string) error {
	runCtx, cancel := context.WithTimeout(ctx, infraOpsPnpmInstallTimeout)
	defer cancel()
	if err := runInfraOpsCommand(runCtx, dir, []string{"CI=true", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"}, "pnpm", "install", "--frozen-lockfile"); err != nil {
		if runCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
			return fmt.Errorf("pnpm install --frozen-lockfile exceeded %s", infraOpsPnpmInstallTimeout)
		}
		return err
	}
	return nil
}

func validateInfraOpsProjectShape(dir string) error {
	for _, name := range []string{"pyproject.toml", "uv.lock", ".python-version"} {
		if err := requireRegularPreparationFile(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	for _, packagePath := range []string{"library/ga4-admin", "library/m365ctl"} {
		packageDir := filepath.Join(dir, packagePath)
		if err := rejectPathSymlinkChain(dir, packagePath); err != nil {
			return err
		}
		info, err := os.Lstat(packageDir)
		if err != nil {
			return fmt.Errorf("required Node package %s: %w", packagePath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe Node package: %s", packagePath)
		}
		for _, name := range []string{"package.json", "pnpm-lock.yaml"} {
			if err := requireRegularPreparationFile(filepath.Join(packageDir, name)); err != nil {
				return err
			}
		}
	}
	// #nosec G304 -- fixed filename in the guarded detached preparation tree, validated above.
	project, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return err
	}
	for _, line := range []string{
		`sbkube = { path = "codes/sbkube", editable = true }`,
		`"scripton-cloud-transformer" = { path = "codes/cloud-script-transformer", editable = true }`,
	} {
		if !strings.Contains(string(project), line) {
			return fmt.Errorf("unsupported infra-ops editable source contract")
		}
	}
	return nil
}

func rejectPathSymlinkChain(root, relative string) error {
	current := root
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		current = filepath.Join(current, part)
		if err := rejectPathSymlinks(current); err != nil {
			return err
		}
	}
	return nil
}

func requireRegularPreparationFile(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return fmt.Errorf("required preparation file %s: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe preparation file: %s", name)
	}
	return nil
}

func verifyInfraOpsChild(ctx context.Context, _ gitRepo, dir, want, name, remote string) error {
	head, err := runInfraOpsCommandOutput(ctx, dir, nil, "git", "rev-parse", "--verify", "HEAD")
	if err != nil || strings.TrimSpace(head) != want {
		return fmt.Errorf("pinned %s checkout did not retain %s", name, want)
	}
	ref, err := runInfraOpsCommandOutput(ctx, dir, nil, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || strings.TrimSpace(ref) != "HEAD" {
		return fmt.Errorf("pinned %s checkout is not detached", name)
	}
	actual, err := runInfraOpsCommandOutput(ctx, dir, nil, "git", "remote", "get-url", "origin")
	if err != nil || strings.TrimSpace(actual) != remote {
		return fmt.Errorf("pinned %s checkout has an unexpected canonical remote", name)
	}
	return nil
}

func isInfraOpsPreparedPath(path string) bool {
	path = filepath.ToSlash(path)
	if path == "codes/" {
		return true
	}
	for _, prefix := range []string{"codes/sbkube", "codes/cloud-script-transformer", "library/ga4-admin/node_modules", "library/m365ctl/node_modules"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func runInfraOpsCommand(ctx context.Context, dir string, extraEnv []string, name string, args ...string) error {
	_, err := runInfraOpsCommandOutput(ctx, dir, extraEnv, name, args...)
	return err
}

func runInfraOpsCommandOutput(ctx context.Context, dir string, extraEnv []string, name string, args ...string) (string, error) {
	if name == "git" {
		args = append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "init.templateDir="}, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- fixed closed profile commands only.
	cmd.Dir = dir
	env := os.Environ()
	for _, key := range []string{"UV_PROJECT_ENVIRONMENT", "VIRTUAL_ENV", "UV_NO_SYNC", "UV_CACHE_DIR", "UV_PROJECT"} {
		env = withoutEnv(env, key)
	}
	if name == "git" {
		for _, entry := range append([]string(nil), env...) {
			key, _, _ := strings.Cut(entry, "=")
			if strings.HasPrefix(key, "GIT_CONFIG") {
				env = withoutEnv(env, key)
			}
		}
		for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_REPLACE_REF_BASE", "GIT_PREFIX"} {
			env = withoutEnv(env, key)
		}
		env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_COUNT=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0")
	}
	cmd.Env = hostSlotsHeldForChild(ctx, append(env, extraEnv...))
	var out, diagnostic bytes.Buffer
	stdout := &infraOpsExactWriter{buffer: &out, remaining: 128 << 10}
	cmd.Stdout = stdout
	cmd.Stderr = &limitedWriter{w: &diagnostic, n: 128 << 10}
	if err := runPrepareCommand(cmd); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s preparation stopped: %w", name, ctx.Err())
		}
		return "", fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(out.String()+"\n"+diagnostic.String()))
	}
	if stdout.overflow {
		return "", fmt.Errorf("%s preparation stdout exceeded 128 KiB; refusing incomplete evidence", name)
	}
	return out.String(), nil
}

type infraOpsExactWriter struct {
	buffer    *bytes.Buffer
	remaining int
	overflow  bool
}

func (w *infraOpsExactWriter) Write(data []byte) (int, error) {
	length := len(data)
	if length > w.remaining {
		w.overflow = true
		data = data[:w.remaining]
	}
	w.remaining -= len(data)
	if _, err := w.buffer.Write(data); err != nil {
		return 0, err
	}
	return length, nil
}

func addPreparedWorktree(ctx context.Context, g gitRepo, path, sha, profile string) error {
	if profile == infraOpsPythonUVV1 {
		return runInfraOpsCommand(ctx, g.dir, nil, "git", "worktree", "add", "--detach", path, sha)
	}
	return g.worktreeAddDetach(ctx, path, sha)
}
