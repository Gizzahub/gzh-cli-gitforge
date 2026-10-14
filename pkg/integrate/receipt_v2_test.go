// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestBeginRunEvidenceRejectsPushEndpointReplay(t *testing.T) {
	repo := testutil.TempGitRepoWithCommit(t)
	decoy := filepath.Join(t.TempDir(), "decoy.git")
	runGit(t, repo, "clone", "--bare", repo, decoy)
	runGit(t, repo, "remote", "add", "origin", repo)
	g := newGitRepo(gitcmd.NewExecutor(), repo)
	head := gitOutput(t, repo, "rev-parse", "HEAD")
	common := gitOutput(t, repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	opts := RunOptions{Execution: &ExecutionContext{
		ExecutionID: "endpoint-replay", Generation: "g1",
		Owner: OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"}, ExpectSource: head,
	}}
	check := &CheckReport{Plan: TargetPlan{Branch: "dev/codex/task", BranchSHA: head, Remote: "origin", TargetSHA: head}}
	first, err := beginRunEvidence(context.Background(), g.exec, g, common, opts, check, head, "master")
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "config", "remote.origin.pushurl", decoy)
	if _, err := beginRunEvidence(context.Background(), g.exec, g, common, opts, check, head, "master"); !errors.Is(err, ErrEvidenceReplay) {
		t.Fatalf("changed push endpoint must refuse intent replay: %v", err)
	}
	after, ok, err := readReceipt(common, opts.Execution.ExecutionID)
	if err != nil || !ok || after.PushRemote != first.PushRemote || after.Revision != first.Revision {
		t.Fatalf("replay rewrote endpoint or revision: %+v %v", after, err)
	}
	if got := gitOutput(t, decoy, "rev-parse", "HEAD"); got != head {
		t.Fatalf("decoy was changed: %s", got)
	}
}

func TestReceiptV1IsReadableButCannotBeRewritten(t *testing.T) {
	common := t.TempDir()
	r := seedCASReceipt("legacy-read", "refs/heads/develop")
	r.RepositoryCommonDir = common
	path := evidenceReceiptPath(common, r.ExecutionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	r.SchemaVersion = evidenceSchemaVersion
	if err := atomicStateWrite(path, mustMarshalJSON(r)); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := readReceipt(common, r.ExecutionID)
	if err != nil || !ok || loaded.SchemaVersion != evidenceSchemaVersion {
		t.Fatalf("read legacy receipt: loaded=%+v ok=%t err=%v", loaded, ok, err)
	}
	if err := writeReceipt(context.Background(), common, loaded); !errors.Is(err, ErrReceiptReadOnly) {
		t.Fatalf("write legacy receipt error=%v, want ErrReceiptReadOnly", err)
	}
}

func TestReceiptLookupBindsExecutionAndCanonicalCommonDirectory(t *testing.T) {
	common := t.TempDir()
	r := seedCASReceipt("lookup-binding", "refs/heads/develop")
	r.SchemaVersion = receiptSchemaVersion
	r.RepositoryCommonDir = common
	if err := createReceipt(context.Background(), common, r); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readReceipt(common, "different-execution"); err != nil || ok {
		t.Fatalf("lookup with a different execution key: ok=%t err=%v", ok, err)
	}
	alias := filepath.Join(t.TempDir(), "common-link")
	if err := os.Symlink(common, alias); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalCommonDir(common)
	if err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := readReceipt(alias, r.ExecutionID)
	if err != nil || !ok || loaded.RepositoryCommonDir != canonical {
		t.Fatalf("canonical common lookup: loaded=%+v ok=%t err=%v", loaded, ok, err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Receipt)
	}{
		{"execution key", func(r *Receipt) { r.ExecutionID = "different-execution" }},
		{"common directory", func(r *Receipt) { r.RepositoryCommonDir = t.TempDir() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			altered := *r
			tc.mutate(&altered)
			if err := atomicStateWrite(evidenceReceiptPath(common, r.ExecutionID), mustMarshalJSON(&altered)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readReceipt(common, r.ExecutionID); !errors.Is(err, ErrEvidenceReplay) {
				t.Fatalf("altered lookup identity must refuse: %v", err)
			}
		})
	}
}

func TestReceiptIdentityIncludesEndpointAndProvenance(t *testing.T) {
	base := seedCASReceipt("identity-binding", "refs/heads/develop")
	base.SchemaVersion = receiptSchemaVersion
	base.TargetBeforeSHA = "1111111111111111111111111111111111111111"
	base.Remote = "origin"
	base.PushRemote = "https://primary.example/repo.git"
	base.Provenance = &evidenceProvenance{
		DeclarationPatterns: []string{"dev/*"},
		ReclaimRefs:         &evidenceReclaimRefs{Default: "main", Integration: "develop"},
	}
	cases := []struct {
		name   string
		mutate func(*Receipt)
		want   string
	}{
		{"target before", func(r *Receipt) { r.TargetBeforeSHA = "2222222222222222222222222222222222222222" }, "target_before_sha"},
		{"remote", func(r *Receipt) { r.Remote = "upstream" }, "remote"},
		{"push endpoint", func(r *Receipt) { r.PushRemote = "https://decoy.example/repo.git" }, "push_remote"},
		{"provenance", func(r *Receipt) { r.Provenance.ReclaimRefs.Integration = "release" }, "provenance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := *base
			if base.Provenance != nil {
				provenance := *base.Provenance
				if base.Provenance.ReclaimRefs != nil {
					refs := *base.Provenance.ReclaimRefs
					provenance.ReclaimRefs = &refs
				}
				candidate.Provenance = &provenance
			}
			tc.mutate(&candidate)
			conflicts := base.identityConflicts(&candidate)
			for _, got := range conflicts {
				if got == tc.want {
					return
				}
			}
			t.Fatalf("conflicts=%v, want %q", conflicts, tc.want)
		})
	}
}
