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

func TestSnapshotInfraOpsTargetInputsRejectsMissingManifestOutsideFirstAdoption(t *testing.T) {
	for _, targetDeclared := range []bool{false, true} {
		t.Run(map[bool]string{false: "controller-only", true: "declared-target"}[targetDeclared], func(t *testing.T) {
			fx := testutil.TempWorktreeWithBareOrigin(t)
			if targetDeclared {
				writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: infra-ops-python-uv-v1\n")
				runGitInTest(t, fx.Worktree, "add", ".")
				runGitInTest(t, fx.Worktree, "commit", "-m", "declared profile without manifest")
			}
			target := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
			if err := os.MkdirAll(filepath.Join(fx.Worktree, "scripts"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, fx.Worktree, infraOpsInputsPath, validInfraOpsInputs)
			if targetDeclared {
				writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: infra-ops-python-uv-v1\n")
			}
			runGitInTest(t, fx.Worktree, "add", ".")
			runGitInTest(t, fx.Worktree, "commit", "-m", "source manifest")
			source := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
			g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
			inputs, err := snapshotInfraOpsPythonUVInputs(context.Background(), g, source)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = snapshotInfraOpsTargetInputs(context.Background(), g, TargetPlan{BranchSHA: source, TargetSHA: target}, inputs)
			if err == nil || !strings.Contains(err.Error(), "outside repository-owned first adoption") {
				t.Fatalf("missing target manifest accepted: %v", err)
			}
		})
	}
}

func TestInfraOpsMakeEnvironmentDoesNotOverlayOtherProfiles(t *testing.T) {
	for _, profile := range []string{"", cargoWorkspacePrepareV1} {
		ctx := withInfraOpsPrepareMakeEnv(context.Background(), "/tmp/root/source", profile)
		if got := prepareMakeEnv(ctx); len(got) != 0 {
			t.Fatalf("profile %q unexpectedly changed make environment: %#v", profile, got)
		}
	}
}

func TestInfraOpsGitRejectsTruncatedEvidence(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeFile(t, fx.Worktree, "large", strings.Repeat("x", (128<<10)+1))
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "large output fixture")
	output, err := runInfraOpsCommandOutput(context.Background(), fx.Worktree, nil, "git", "show", "HEAD:large")
	if err == nil || !strings.Contains(err.Error(), "refusing incomplete evidence") || output != "" {
		t.Fatalf("truncated evidence accepted: length=%d err=%v", len(output), err)
	}
}

func TestPrepareInfraOpsPythonUVCreatesSeparateOwnedEnvironmentsAndCleansUp(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	remotes, inputs := infraOpsChildRemotes(t)
	original := infraOpsDependencies
	infraOpsDependencies = remotes
	t.Cleanup(func() { infraOpsDependencies = original })
	writeInfraOpsFixture(t, fx.Worktree, inputs)
	if err := os.Remove(filepath.Join(fx.Worktree, infraOpsInputsPath)); err != nil {
		t.Fatal(err)
	}
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "infra fixture target")
	targetSHA := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	manifest := `{"schema_version":1,"dependencies":[{"name":"sbkube","revision":"` + inputs[1].OID + `"},{"name":"cloud-script-transformer","revision":"` + inputs[2].OID + `"}]}`
	writeFile(t, fx.Worktree, infraOpsInputsPath, manifest)
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: infra-ops-python-uv-v1\n")
	runGitInTest(t, fx.Worktree, "add", infraOpsInputsPath, ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "adopt profile")
	sourceSHA := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	bin := fakeInfraOpsTools(t)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("UV_PROJECT_ENVIRONMENT", "/unsafe/live")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	plan := TargetPlan{BranchSHA: sourceSHA, TargetSHA: targetSHA}
	prepared, err := prepareLegacySource(context.Background(), g, plan, infraOpsPythonUVV1)
	if err != nil {
		t.Fatalf("prepare source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(infraOpsOwnedRoot(prepared.source), "venv", ".ready")); err != nil {
		t.Fatalf("source owned venv: %v", err)
	}
	link, err := os.Lstat(filepath.Join(prepared.source, "codes", "sbkube"))
	if err != nil || link.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("source child must be a link, info=%v err=%v", link, err)
	}
	sourceProbe := runMakeTarget(withInfraOpsPrepareMakeEnv(context.Background(), prepared.source, infraOpsPythonUVV1), prepared.source, "check", 0)
	if sourceProbe.Err == nil || sourceProbe.Code == 0 || !strings.Contains(sourceProbe.Output, "fixture.py:1") {
		t.Fatalf("source make=%#v", sourceProbe)
	}
	baseline, err := prepared.measureBaseline(context.Background(), plan, []string{"check"}, 0, nil)
	if err != nil {
		t.Fatalf("prepare target and run make: %v", err)
	}
	if probe := baseline["check"]; probe.Err != nil || probe.Code != 0 {
		t.Fatalf("target make=%#v", probe)
	}
	if !strings.Contains(prepared.evidence(), "source:uv.lock@") || !strings.Contains(prepared.evidence(), "target:uv.lock@") {
		t.Fatalf("missing lock provenance: %s", prepared.evidence())
	}
	root := prepared.root
	if err := prepared.cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("prepare root remains: %v", err)
	}
}

func TestPrepareInfraOpsPythonUVRejectsUnsafeProjectBeforeTools(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(root, filepath.Join(root, "pyproject.toml")); err != nil {
		t.Fatal(err)
	}
	if err := validateInfraOpsProjectShape(root); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("err=%v", err)
	}
}

func TestInfraOpsFirstAdoptionChecksBothSidesThroughLegacyGate(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	remotes, inputs := infraOpsChildRemotes(t)
	original := infraOpsDependencies
	infraOpsDependencies = remotes
	t.Cleanup(func() { infraOpsDependencies = original })
	writeInfraOpsFixture(t, fx.Worktree, inputs)
	if err := os.Remove(filepath.Join(fx.Worktree, infraOpsInputsPath)); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "probes")
	t.Setenv("PREP_PROBE_LOG", log)
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@test \"$$UV_PROJECT_ENVIRONMENT\" = \"$$VIRTUAL_ENV\"\n\t@test \"$$UV_NO_SYNC\" = 1\n\t@printf '%s\\n' \"$$UV_PROJECT_ENVIRONMENT\" >> \"$$PREP_PROBE_LOG\"\n\t@if test -f fail-source; then echo 'fixture.py:1: source regression'; exit 1; fi\nlint:\n\t@true\n")
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "unprepared target")
	targetSHA := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	manifest := `{"schema_version":1,"dependencies":[{"name":"sbkube","revision":"` + inputs[1].OID + `"},{"name":"cloud-script-transformer","revision":"` + inputs[2].OID + `"}]}`
	writeFile(t, fx.Worktree, infraOpsInputsPath, manifest)
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: infra-ops-python-uv-v1\n")
	writeFile(t, fx.Worktree, "fail-source", "forces lazy baseline measurement\n")
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "first profile adoption")
	sourceSHA := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	bin := fakeInfraOpsTools(t)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("UV_PROJECT_ENVIRONMENT", "/unsafe/live")
	t.Setenv("VIRTUAL_ENV", "/unsafe/live")
	t.Setenv("UV_NO_SYNC", "0")
	poison := t.TempDir()
	marker := filepath.Join(poison, "hook-ran")
	hook := "#!/bin/sh\nprintf hook >> '" + marker + "'\n"
	for _, name := range []string{"post-checkout", "fsmonitor"} {
		if err := os.WriteFile(filepath.Join(poison, name), []byte(hook), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	global := filepath.Join(poison, "global")
	if err := os.WriteFile(global, []byte("[core]\n hooksPath = "+poison+"\n fsmonitor = "+filepath.Join(poison, "fsmonitor")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	plan := TargetPlan{HeadSHA: sourceSHA, BranchSHA: sourceSHA, TargetSHA: targetSHA}
	report := &CheckReport{}
	checkLegacyMake(context.Background(), g, plan, nil, false, false, false, CheckOptions{}, report, func(item CheckItem) {
		report.Items = append(report.Items, item)
	})
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ambient hook executed during legacy preparation: %v", err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("source/target probes did not execute: %v; items=%#v", err, report.Items)
	}
	environments := strings.Fields(string(data))
	if len(environments) != 2 || environments[0] == environments[1] || !strings.Contains(environments[0], "/source/venv") || !strings.Contains(environments[1], "/target/venv") {
		t.Fatalf("probe environments=%v", environments)
	}
	if report.PrepareProfile != infraOpsPythonUVV1 || !strings.Contains(report.PrepareInputs, "target-input-mode=source-bootstrap") || !strings.Contains(report.PrepareInputs, "target:uv.lock@") {
		t.Fatalf("incomplete upfront provenance: %#v", report)
	}
	measured := false
	for _, baseline := range report.Baselines {
		if baseline.Target == "check" && baseline.Measured {
			measured = true
		}
	}
	if !measured {
		t.Fatal("source failure did not trigger the lazy target baseline")
	}
	for _, environment := range environments {
		if _, err := os.Stat(environment); !os.IsNotExist(err) {
			t.Fatalf("environment was not reclaimed: %s: %v", environment, err)
		}
	}
}

func TestInfraOpsTargetManifestDoesNotHideChildPinRegression(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	remotes, oldInputs := infraOpsChildRemotes(t)
	original := infraOpsDependencies
	infraOpsDependencies = remotes
	t.Cleanup(func() { infraOpsDependencies = original })
	newChild := filepath.Join(t.TempDir(), "child")
	runGitInTest(t, fx.Worktree, "clone", remotes[0].remote, newChild)
	runGitInTest(t, newChild, "config", "user.name", "Test")
	runGitInTest(t, newChild, "config", "user.email", "test@example.invalid")
	runGitInTest(t, newChild, "checkout", "--detach", oldInputs[1].OID)
	writeFile(t, newChild, "breaks-import", "broken dependency revision\n")
	runGitInTest(t, newChild, "add", ".")
	runGitInTest(t, newChild, "commit", "-m", "broken child pin")
	newOID := strings.TrimSpace(runGitInTest(t, newChild, "rev-parse", "HEAD"))
	runGitInTest(t, newChild, "push", "origin", "HEAD:refs/heads/new-pin")
	writeInfraOpsFixture(t, fx.Worktree, oldInputs)
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: infra-ops-python-uv-v1\n")
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@test \"$$UV_PROJECT_ENVIRONMENT\" = \"$$VIRTUAL_ENV\"\n\t@test \"$$UV_NO_SYNC\" = 1\n\t@if test -f codes/sbkube/breaks-import; then echo 'fixture.py:1: broken child'; exit 1; fi\n")
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "valid target child pin")
	targetSHA := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	manifest := `{"schema_version":1,"dependencies":[{"name":"sbkube","revision":"` + newOID + `"},{"name":"cloud-script-transformer","revision":"` + oldInputs[2].OID + `"}]}`
	writeFile(t, fx.Worktree, infraOpsInputsPath, manifest)
	runGitInTest(t, fx.Worktree, "add", infraOpsInputsPath)
	runGitInTest(t, fx.Worktree, "commit", "-m", "introduce a child-pin regression")
	sourceSHA := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	bin := fakeInfraOpsTools(t)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	plan := TargetPlan{HeadSHA: sourceSHA, BranchSHA: sourceSHA, TargetSHA: targetSHA}
	prepared, err := prepareLegacySource(context.Background(), g, plan, infraOpsPythonUVV1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := prepared.cleanup(context.Background()); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	source := runMakeTarget(withInfraOpsPrepareMakeEnv(context.Background(), prepared.source, infraOpsPythonUVV1), prepared.source, "check", 0)
	if source.Code == 0 || !strings.Contains(source.Output, "broken child") {
		t.Fatalf("source did not exercise the new broken child: %#v", source)
	}
	baseline, err := prepared.measureBaseline(context.Background(), plan, []string{"check"}, 0, nil)
	if err != nil || baseline["check"].Code != 0 || baseline["check"].Err != nil {
		t.Fatalf("target was contaminated with the source's broken pin: %#v, err=%v", baseline, err)
	}
	if !strings.Contains(prepared.evidence(), "source:sbkube@"+newOID) || !strings.Contains(prepared.evidence(), "target:sbkube@"+oldInputs[1].OID) {
		t.Fatalf("receipt does not distinguish committed child inputs: %s", prepared.evidence())
	}
}

func TestPrepareInfraOpsPythonUVRejectsPreexistingNodeResourceBeforeTools(t *testing.T) {
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
