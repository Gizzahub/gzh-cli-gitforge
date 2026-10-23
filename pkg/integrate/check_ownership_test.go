// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// ownedCheckInputs is what Check reads before and after its fetch, captured
// the way Check captures it, so a test can change one input in between.
type ownedCheckInputs struct {
	g                gitRepo
	root             string
	preflight        targetPreflight
	plan             TargetPlan
	controller       *controllerBinding
	decl             config.TaskPatternDecl
	repositoryTarget bool
}

func captureOwnedCheckInputs(t *testing.T, fx *testutil.WorktreeOrigin, controllerPath string) ownedCheckInputs {
	t.Helper()
	ctx := context.Background()
	executor := gitcmd.NewExecutor()
	opts := CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", ControllerConfig: controllerPath}
	g, root, err := openCheckRepo(ctx, executor, opts.RepoPath)
	if err != nil {
		t.Fatal(err)
	}
	in := ownedCheckInputs{g: g, root: root, repositoryTarget: controllerPath == ""}
	if in.controller, err = resolveCheckController(ctx, g, &opts); err != nil {
		t.Fatal(err)
	}
	if in.decl, err = config.LoadRepoRootTaskPattern(root); err != nil {
		t.Fatal(err)
	}
	if len(opts.IntegrationConfig) == 0 {
		opts.IntegrationConfig = in.decl.IntegrationBranch
	}
	if in.preflight, err = preflightTargetRef(ctx, g, executor, opts, in.controller); err != nil {
		t.Fatal(err)
	}
	if in.plan, err = resolveTarget(ctx, g, executor, opts); err != nil {
		t.Fatal(err)
	}
	return in
}

func (in ownedCheckInputs) validate() error {
	_, err := validateOwnedCheckPlan(context.Background(), in.g, in.root, in.preflight, in.plan, in.controller, in.decl, in.repositoryTarget)
	return err
}

// TestValidateOwnedCheckPlanRefusesChangedDeclaration changes, between the
// preflight and the post-fetch validation, the source branch, the target, the
// file the declaration came from, and the declared integration branch. Each
// change must refuse the check instead of adopting the post-fetch plan.
func TestValidateOwnedCheckPlanRefusesChangedDeclaration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(t *testing.T, fx *testutil.WorktreeOrigin, in *ownedCheckInputs)
		want   string
	}{
		{"unchanged", func(*testing.T, *testutil.WorktreeOrigin, *ownedCheckInputs) {}, ""},
		{"source branch", func(_ *testing.T, _ *testutil.WorktreeOrigin, in *ownedCheckInputs) {
			in.plan.Branch = "dev/actor/feat/other"
		}, "integration source or target changed between preflight and fetch; re-run check"},
		{"target", func(_ *testing.T, _ *testutil.WorktreeOrigin, in *ownedCheckInputs) {
			in.plan.Target = in.plan.Remote + "/main"
		}, "integration source or target changed between preflight and fetch; re-run check"},
		{"declaration source", func(t *testing.T, fx *testutil.WorktreeOrigin, _ *ownedCheckInputs) {
			t.Helper()
			if err := os.Remove(filepath.Join(fx.Worktree, ".gz-git.yaml")); err != nil {
				t.Fatal(err)
			}
			writeRepoFile(t, fx.Worktree, ".gz-git.yml", "branch:\n  integrationBranch: develop\n")
		}, "integration declaration changed between preflight and fetch; re-run check"},
		{"integration branch", func(t *testing.T, fx *testutil.WorktreeOrigin, _ *ownedCheckInputs) {
			t.Helper()
			writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: main\n")
		}, "integration declaration changed between preflight and fetch; re-run check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := runFixture(t, "")
			in := captureOwnedCheckInputs(t, fx, "")
			tc.change(t, fx, &in)
			err := in.validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unchanged inputs refused: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("validate = %v, want %q", err, tc.want)
			}
		})
	}
}

// TestValidateOwnedCheckPlanRefusesChangedController edits the controller
// config between the preflight and the post-fetch validation; the check must
// refuse rather than run against a controller nobody read.
func TestValidateOwnedCheckPlanRefusesChangedController(t *testing.T) {
	t.Parallel()
	fx := runFixture(t, "")
	controllerPath := filepath.Join(t.TempDir(), "controller.yaml")
	body := "workspaces:\n  engine: {url: " + fx.Origin + ", branch: {integrationBranch: develop}, integration: {}}\n"
	writeFile(t, filepath.Dir(controllerPath), filepath.Base(controllerPath), body)
	in := captureOwnedCheckInputs(t, fx, controllerPath)
	if err := in.validate(); err != nil {
		t.Fatalf("unchanged controller refused: %v", err)
	}
	writeFile(t, filepath.Dir(controllerPath), filepath.Base(controllerPath), body+"# changed after preflight\n")
	err := in.validate()
	if err == nil || !strings.Contains(err.Error(), "controller config changed during readiness; re-run check") {
		t.Fatalf("validate = %v, want a controller change refusal", err)
	}
}
