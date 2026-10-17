package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

func reviewUpdateFixture(t *testing.T, runner string) (string, ReadinessUpdateReviewOptions) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("readiness runners are unsupported on Windows")
	}
	work := readinessUpdateFixture(t, ".gz-git.yaml")
	if err := os.WriteFile(filepath.Join(work, ".gz-git", "readiness", "check"), []byte(runner), 0o755); err != nil {
		t.Fatal(err)
	}
	runGitInTest(t, work, "add", ".gz-git/readiness/check")
	runGitInTest(t, work, "commit", "-m", "candidate contract")
	return work, ReadinessUpdateReviewOptions{RepoPath: work, Target: "origin/master", Issuer: "test", ExpectSource: runGitInTest(t, work, "rev-parse", "HEAD")}
}

func setReviewApproval(t *testing.T, mode string) {
	t.Helper()
	// os.UserConfigDir uses different native locations on Darwin and Unix.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("AppData", home)
	paths, err := config.NewPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.ConfigDir, "config.yaml"), []byte("integration:\n  readinessApproval: "+mode+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

const candidateReady = "#!/bin/sh\n# candidate\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"candidate checked\"}'\n"

func TestReadinessReviewSingleDiffApproval(t *testing.T) {
	work, opts := reviewUpdateFixture(t, candidateReady)
	calls := 0
	opts.Review = func(p ReadinessUpdatePlan, diff string) (bool, error) {
		calls++
		if !strings.Contains(diff, "+# candidate") || p.SourceSHA != opts.ExpectSource || p.TargetSHA == p.SourceSHA {
			t.Fatalf("wrong review: %+v %s", p, diff)
		}
		return true, nil
	}
	p, err := ReviewReadinessUpdate(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.HasPrefix(runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master"), opts.ExpectSource) {
		t.Fatal("approved diff was not applied exactly once")
	}
	if !strings.HasPrefix(runGitInTest(t, work, "ls-remote", "origin", p.OperationRef), opts.ExpectSource) {
		t.Fatal("one-use marker missing")
	}
}

func TestReadinessReviewDeclineAndDriftDoNotPush(t *testing.T) {
	for _, drift := range []bool{false, true} {
		t.Run(map[bool]string{false: "decline", true: "drift"}[drift], func(t *testing.T) {
			work, opts := reviewUpdateFixture(t, candidateReady)
			before := runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master")
			opts.Review = func(_ ReadinessUpdatePlan, _ string) (bool, error) {
				if drift {
					runGitInTest(t, work, "commit", "--allow-empty", "--amend", "-m", "changed after review")
				}
				return drift, nil
			}
			if _, err := ReviewReadinessUpdate(context.Background(), gitcmd.NewExecutor(), opts); err == nil {
				t.Fatal("declined/changed review was applied")
			}
			if got := runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master"); got != before {
				t.Fatal("remote changed after refusal")
			}
		})
	}
}

func TestReadinessAutomaticRequiresGlobalAuthorityAndRequest(t *testing.T) {
	for _, mode := range []string{"manual", "automatic"} {
		for _, statement := range []string{"", "テスト完了", "테스트완료"} {
			t.Run(mode+statement, func(t *testing.T) {
				setReviewApproval(t, mode)
				opts := ReadinessUpdateReviewOptions{Automatic: true, Statement: statement, ExpectSource: strings.Repeat("a", 40)}
				err := validateReadinessReview(opts)
				if (err == nil) != (mode == "automatic" && statement == "테스트완료") {
					t.Fatalf("validation: %v", err)
				}
				opts.ExpectSource = ""
				if validateReadinessReview(opts) == nil {
					t.Fatal("automatic request accepted without commit")
				}
			})
		}
	}
}

func TestReadinessAutomaticRunsCandidateAndRecordsObservedPair(t *testing.T) {
	runner := "#!/bin/sh\nset -eu\ntest \"$(git rev-parse HEAD)\" = \"$6\"\ntest \"$(git -C \"$2\" rev-parse HEAD)\" = \"$4\"\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"source runner executed\"}'\n"
	work, opts := reviewUpdateFixture(t, runner)
	setReviewApproval(t, "automatic")
	opts.Automatic, opts.Statement = true, "테스트완료"
	var notice bytes.Buffer
	opts.Notice = &notice
	p, err := ReviewReadinessUpdate(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatal(err)
	}
	common := runGitInTest(t, work, "rev-parse", "--path-format=absolute", "--git-common-dir")
	data, err := os.ReadFile(filepath.Join(common, "gz-git-readiness-updates", p.OperationID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Plan    ReadinessUpdatePlan `json:"plan"`
		Status  string              `json:"status"`
		Summary string              `json:"summary"`
	}
	if err := json.Unmarshal(data, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "ready" || evidence.Plan.SourceSHA != p.SourceSHA || evidence.Plan.TargetSHA != p.TargetSHA || evidence.Summary != "source runner executed" || !strings.Contains(notice.String(), "READINESS_MEASURED") {
		t.Fatalf("wrong observed evidence: %s", data)
	}
}

func TestReadinessAutomaticRejectsFalseCompletionAndWrongCommit(t *testing.T) {
	for i, runner := range []string{
		"#!/bin/sh\nprintf '{\"version\":1,\"status\":\"not_ready\",\"summary\":\"tests failed\"}'\n",
		"#!/bin/sh\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"claimed pass\"}'\nexit 1\n",
		"#!/bin/sh\nprintf '테스트완료'\n",
	} {
		t.Run(fmt.Sprintf("failed-runner-%d", i), func(t *testing.T) {
			work, opts := reviewUpdateFixture(t, runner)
			setReviewApproval(t, "automatic")
			before := runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master")
			opts.Automatic, opts.Statement = true, "테스트완료"
			if _, err := ReviewReadinessUpdate(context.Background(), gitcmd.NewExecutor(), opts); err == nil {
				t.Fatal("false completion accepted")
			}
			if runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master") != before {
				t.Fatal("failed candidate was pushed")
			}
			opts.ExpectSource = strings.Repeat("a", 40)
			if _, err := ReviewReadinessUpdate(context.Background(), gitcmd.NewExecutor(), opts); err == nil || !strings.Contains(err.Error(), "requested source changed") {
				t.Fatalf("wrong commit accepted: %v", err)
			}
		})
	}
}

func TestReadinessAutomaticRejectsMovementAndRevokedAuthorityAfterCheck(t *testing.T) {
	for _, scenario := range []string{"source-moved", "policy-revoked"} {
		t.Run(scenario, func(t *testing.T) {
			mutate := "git update-ref refs/heads/dev/update \"$6\"\n"
			if scenario == "policy-revoked" {
				mutate = "printf 'integration:\\n  readinessApproval: manual\\n' > \"$READINESS_TEST_REVOKE\"\n"
			}
			runner := "#!/bin/sh\nset -eu\n" + mutate + "printf '{\"version\":1,\"status\":\"ready\",\"summary\":\"check finished\"}'\n"
			work, opts := reviewUpdateFixture(t, runner)
			setReviewApproval(t, "automatic")
			paths, err := config.NewPaths()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("READINESS_TEST_REVOKE", paths.GlobalConfigFile)
			before := runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master")
			opts.Automatic, opts.Statement = true, "테스트완료"
			if _, err := ReviewReadinessUpdate(context.Background(), gitcmd.NewExecutor(), opts); err == nil {
				t.Fatal("changed authority/source accepted after successful check")
			}
			if runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master") != before {
				t.Fatal("remote changed after successful but invalidated check")
			}
		})
	}
}

func TestReadinessAutomaticHonorsCandidateExecutionBudget(t *testing.T) {
	work, opts := reviewUpdateFixture(t, "#!/bin/sh\nsleep 2\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"too late\"}'\n")
	manifest := filepath.Join(work, ".gz-git.yaml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, append(data, []byte("    timeout: 20ms\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitInTest(t, work, "add", ".gz-git.yaml")
	runGitInTest(t, work, "commit", "--amend", "--no-edit")
	opts.ExpectSource = runGitInTest(t, work, "rev-parse", "HEAD")
	setReviewApproval(t, "automatic")
	before := runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master")
	opts.Automatic, opts.Statement = true, "테스트완료"
	if _, err := ReviewReadinessUpdate(context.Background(), gitcmd.NewExecutor(), opts); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("candidate budget not enforced: %v", err)
	}
	if runGitInTest(t, work, "ls-remote", "origin", "refs/heads/master") != before {
		t.Fatal("timed-out candidate was applied")
	}
}
