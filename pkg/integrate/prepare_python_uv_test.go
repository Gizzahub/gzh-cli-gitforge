// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

const validInfraOpsInputs = `{"schema_version":1,"dependencies":[{"name":"sbkube","revision":"d41f830288d12206c0a07107ed75137656e35135"},{"name":"cloud-script-transformer","revision":"6bec7323ede79cf477f5b4c945b8e29d33721502"}]}`

func TestSnapshotInfraOpsPythonUVInputsUsesPinnedSourceCommit(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	if err := os.MkdirAll(filepath.Join(fx.Worktree, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fx.Worktree, infraOpsInputsPath, validInfraOpsInputs)
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "manifest")
	source := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	inputs, err := snapshotInfraOpsPythonUVInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 3 || inputs[0].Name != "manifest" || inputs[1].OID != "d41f830288d12206c0a07107ed75137656e35135" {
		t.Fatalf("inputs=%#v", inputs)
	}
	// A later worktree edit must not change the captured source-commit input.
	writeFile(t, fx.Worktree, infraOpsInputsPath, `{"schema_version":1,"dependencies":[]}`)
	if inputs[1].OID != "d41f830288d12206c0a07107ed75137656e35135" {
		t.Fatal("snapshot changed after worktree edit")
	}
}

func TestSnapshotInfraOpsPythonUVInputsRejectsUnsafeManifestShapes(t *testing.T) {
	t.Parallel()
	for name, manifest := range map[string]string{
		"trailing object":    validInfraOpsInputs + `{}`,
		"trailing garbage":   validInfraOpsInputs + `garbage`,
		"unknown field":      `{"schema_version":1,"extra":true,"dependencies":[{"name":"sbkube","revision":"d41f830288d12206c0a07107ed75137656e35135"},{"name":"cloud-script-transformer","revision":"6bec7323ede79cf477f5b4c945b8e29d33721502"}]}`,
		"duplicate revision": `{"schema_version":1,"dependencies":[{"name":"sbkube","revision":"d41f830288d12206c0a07107ed75137656e35135","revision":"d41f830288d12206c0a07107ed75137656e35135"},{"name":"cloud-script-transformer","revision":"6bec7323ede79cf477f5b4c945b8e29d33721502"}]}`,
		"wrong order":        `{"schema_version":1,"dependencies":[{"name":"cloud-script-transformer","revision":"6bec7323ede79cf477f5b4c945b8e29d33721502"},{"name":"sbkube","revision":"d41f830288d12206c0a07107ed75137656e35135"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			fx := testutil.TempWorktreeWithBareOrigin(t)
			if err := os.MkdirAll(filepath.Join(fx.Worktree, "scripts"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, fx.Worktree, infraOpsInputsPath, manifest)
			runGitInTest(t, fx.Worktree, "add", ".")
			runGitInTest(t, fx.Worktree, "commit", "-m", "manifest")
			sha := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
			if _, err := snapshotInfraOpsPythonUVInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), sha); err == nil {
				t.Fatal("unsafe manifest accepted")
			}
		})
	}
}

func TestInfraOpsMakeEnvironmentIsOwnedAndNotAmbient(t *testing.T) {
	t.Setenv("UV_PROJECT_ENVIRONMENT", "/unsafe/live")
	t.Setenv("VIRTUAL_ENV", "/unsafe/live")
	ctx := withInfraOpsPrepareMakeEnv(context.Background(), "/tmp/root/source", "infra-ops-python-uv-v1")
	got := strings.Join(prepareMakeEnv(ctx), "\n")
	if !strings.Contains(got, "/tmp/root/infra-ops-python-uv-v1/source/venv") || !strings.Contains(got, "UV_NO_SYNC=1") {
		t.Fatalf("environment=%q", got)
	}
	if strings.Contains(got, "/unsafe/live") {
		t.Fatalf("ambient live environment leaked: %q", got)
	}
}

func TestInfraOpsMakeEnvironmentDoesNotOverlayOtherProfiles(t *testing.T) {
	t.Parallel()
	for _, profile := range []string{"", "other-profile-v1"} {
		ctx := withInfraOpsPrepareMakeEnv(context.Background(), "/tmp/root/source", profile)
		if got := prepareMakeEnv(ctx); len(got) != 0 {
			t.Fatalf("profile %q unexpectedly changed make environment: %#v", profile, got)
		}
	}
}

func TestInfraOpsGitRejectsTruncatedEvidence(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeFile(t, fx.Worktree, "large", strings.Repeat("x", (128<<10)+1))
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "large output fixture")
	output, err := runInfraOpsCommandOutput(context.Background(), fx.Worktree, nil, "git", "show", "HEAD:large")
	if err == nil || !strings.Contains(err.Error(), "refusing incomplete evidence") || output != "" {
		t.Fatalf("truncated evidence accepted: length=%d err=%v", len(output), err)
	}
}

func TestPrepareInfraOpsPythonUVRejectsUnsafeProjectBeforeTools(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Symlink(root, filepath.Join(root, "pyproject.toml")); err != nil {
		t.Fatal(err)
	}
	if err := validateInfraOpsProjectShape(root); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("err=%v", err)
	}
}

func TestPrepareInfraOpsPythonUVRejectsPreexistingNodeResourceBeforeTools(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	inputs := []prepareInput{{Name: "manifest", OID: strings.Repeat("a", 40)}, {Name: "sbkube", OID: strings.Repeat("b", 40)}, {Name: "cloud-script-transformer", OID: strings.Repeat("c", 40)}}
	writeInfraOpsFixture(t, fx.Worktree, inputs)
	if err := os.MkdirAll(filepath.Join(fx.Worktree, "library", "ga4-admin", "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := prepareInfraOpsPythonUV(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), fx.Worktree, inputs, preparedRefGuard{})
	if err == nil || !strings.Contains(err.Error(), "pre-existing Node resource") {
		t.Fatalf("err=%v", err)
	}
}

func TestInfraOpsGitIgnoresTransportRewriteAndCheckoutHooks(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	sha := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	root := t.TempDir()
	marker := filepath.Join(root, "hook-ran")
	hooks := filepath.Join(root, "hooks")
	if err := os.Mkdir(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	poison := filepath.Join(root, "global.gitconfig")
	missingRemote := "file://" + filepath.Join(root, "does-not-exist")
	configText := "[url \"" + missingRemote + "\"]\n insteadOf = " + fx.Origin + "\n[core]\n hooksPath = " + hooks + "\n[init]\n templateDir = " + root + "\n"
	if err := os.WriteFile(poison, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", poison)
	t.Setenv("GIT_CONFIG_SYSTEM", poison)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+missingRemote+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", fx.Origin)
	t.Setenv("GIT_CONFIG_PARAMETERS", "'url."+missingRemote+".insteadOf'='"+fx.Origin+"'")
	clone := filepath.Join(root, "clone")
	ctx := context.Background()
	if err := runInfraOpsCommand(ctx, root, nil, "git", "clone", "--no-checkout", fx.Origin, clone); err != nil {
		t.Fatalf("canonical clone was redirected by ambient configuration: %v", err)
	}
	if err := runInfraOpsCommand(ctx, clone, nil, "git", "checkout", "--detach", sha); err != nil {
		t.Fatal(err)
	}
	if err := verifyInfraOpsChild(ctx, gitRepo{}, clone, sha, "fixture", fx.Origin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ambient checkout hook executed: %v", err)
	}
}

func TestInfraOpsPreparationReceiptRetainsInputsAcrossRetry(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	check := &CheckReport{PrepareProfile: infraOpsPythonUVV1, PrepareInputs: "infra-ops-python-uv-v1 manifest@" + strings.Repeat("a", 40) + " sbkube@" + strings.Repeat("b", 40) + " source:uv.lock@" + strings.Repeat("c", 40)}
	r := seedCASReceipt("prepare-provenance", "refs/heads/master")
	r.Provenance = runProvenance(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), check)
	common := t.TempDir()
	if err := createReceipt(context.Background(), common, r); err != nil {
		t.Fatal(err)
	}
	for attempt := range 2 {
		loaded, found, err := readReceipt(common, r.ExecutionID)
		if err != nil || !found {
			t.Fatalf("read attempt %d: found=%v err=%v", attempt, found, err)
		}
		if loaded.Provenance == nil || loaded.Provenance.PrepareProfile != check.PrepareProfile || loaded.Provenance.PrepareInputs != check.PrepareInputs {
			t.Fatalf("input provenance lost on attempt %d: %#v", attempt, loaded.Provenance)
		}
		if err := writeReceipt(context.Background(), common, loaded); err != nil {
			t.Fatal(err)
		}
	}
}

func infraOpsChildRemotes(t *testing.T) ([]struct{ name, remote string }, []prepareInput) {
	t.Helper()
	deps := make([]struct{ name, remote string }, 0, 2)
	inputs := make([]prepareInput, 1, 3)
	inputs[0] = prepareInput{Name: "manifest", OID: strings.Repeat("a", 40)}
	for _, name := range []string{"sbkube", "cloud-script-transformer"} {
		fx := testutil.TempWorktreeWithBareOrigin(t)
		writeFile(t, fx.Worktree, "pyproject.toml", "[project]\nname='fixture'\n")
		runGitInTest(t, fx.Worktree, "add", ".")
		runGitInTest(t, fx.Worktree, "commit", "-m", "child")
		sha := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
		runGitInTest(t, fx.Worktree, "push", "origin", "HEAD")
		deps = append(deps, struct{ name, remote string }{name, fx.Origin})
		inputs = append(inputs, prepareInput{Name: name, OID: sha})
	}
	return deps, inputs
}

const infraOpsFixtureMakefile = `check:
	@case "$$UV_PROJECT_ENVIRONMENT" in *infra-ops-python-uv-v1/*/venv) ;; *) exit 9;; esac
	@test "$$VIRTUAL_ENV" = "$$UV_PROJECT_ENVIRONMENT"
	@test "$$UV_NO_SYNC" = 1
	@grep -Fx '# tracked direnv boundary; preparation must preserve this file' codes/.envrc >/dev/null
	@if test "$$(basename "$$PWD")" = source; then echo fixture.py:1: expected; exit 1; fi

lint:
	@true
`

func writeInfraOpsFixture(t *testing.T, dir string, inputs []prepareInput) {
	t.Helper()
	for _, path := range []string{"scripts", "codes", "library/ga4-admin", "library/m365ctl"} {
		if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"schema_version":1,"dependencies":[{"name":"sbkube","revision":"` + inputs[1].OID + `"},{"name":"cloud-script-transformer","revision":"` + inputs[2].OID + `"}]}`
	writeFile(t, dir, infraOpsInputsPath, manifest)
	writeFile(t, dir, "pyproject.toml", "[tool.uv.sources]\nsbkube = { path = \"codes/sbkube\", editable = true }\n\"scripton-cloud-transformer\" = { path = \"codes/cloud-script-transformer\", editable = true }\n")
	writeFile(t, dir, "uv.lock", "version = 1\n")
	writeFile(t, dir, ".python-version", "3.14\n")
	writeFile(t, dir, ".gitignore", "codes/*\n!codes/.envrc\nnode_modules/\nlibrary/*/node_modules/\n")
	writeFile(t, dir, "codes/.envrc", "# tracked direnv boundary; preparation must preserve this file\n")
	writeFile(t, dir, "Makefile", infraOpsFixtureMakefile)
	for _, packagePath := range []string{"library/ga4-admin", "library/m365ctl"} {
		writeFile(t, dir, filepath.Join(packagePath, "package.json"), "{}\n")
		writeFile(t, dir, filepath.Join(packagePath, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n")
	}
}

func fakeInfraOpsTools(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"uv":   `test "$1" = sync; test "$2" = --frozen; test "$UV_NO_SYNC" = 0; mkdir -p "$UV_PROJECT_ENVIRONMENT"; : > "$UV_PROJECT_ENVIRONMENT/.ready"; : > "$UV_PROJECT_ENVIRONMENT/pyvenv.cfg"`,
		"pnpm": `test "$1" = install; test "$2" = --frozen-lockfile; test "$CI" = true; mkdir -p node_modules; : > node_modules/.modules.yaml`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	return dir
}
