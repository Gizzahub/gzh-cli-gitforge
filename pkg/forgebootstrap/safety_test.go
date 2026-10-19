package forgebootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func canonicalTemp(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// isolateGitIdentity gives the seed commit a fixture author, so the test does
// not depend on the developer's global identity, which a CI runner lacks.
func isolateGitIdentity(t *testing.T) {
	t.Helper()
	config := filepath.Join(canonicalTemp(t), "gitconfig")
	if err := os.WriteFile(config, []byte("[user]\n name = fixture\n email = fixture@example.invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
}

func TestSeedPlanBindsContentsAndRejectsArtifacts(t *testing.T) {
	root := canonicalTemp(t)
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := valid()
	r.SeedDir = root
	r.TemplateURL = ""
	r.TemplateSHA = ""
	p, err := NewPlan(r, time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "index.html"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := seedDigest(root)
	if err != nil || digest == p.TemplateDigest {
		t.Fatal("seed digest does not bind contents")
	}
	if err = os.Mkdir(filepath.Join(root, "node_modules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err = NewPlan(r, time.Now()); err == nil {
		t.Fatal("accepted build artifacts")
	}
}

func TestSafetyRejectsSecretsAndAncestorSymlinks(t *testing.T) {
	for _, site := range []SiteConfig{{"token": "secret"}, {"url": "https://user:secret@example.invalid"}, {"value": "glpat-secret"}} {
		r := valid()
		r.Site = site
		if ValidateRequest(r) == nil {
			t.Fatal("accepted secret configuration")
		}
	}
	r := valid()
	r.Remote = "https://user:secret@example.invalid/repo.git"
	if ValidateRequest(r) == nil {
		t.Fatal("accepted credential URL")
	}
	root := canonicalTemp(t)
	if err := os.Symlink(root, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if validateCheckoutPath(filepath.Join(root, "link", "nested", "site")) == nil {
		t.Fatal("accepted symlink ancestor")
	}
}

func TestSeedDriftStopsApplyBeforeCopy(t *testing.T) {
	root := canonicalTemp(t)
	seed := filepath.Join(root, "seed")
	if err := os.Mkdir(seed, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(seed, "index.html")
	if err := os.WriteFile(file, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := valid()
	r.SeedDir = seed
	r.TemplateURL = ""
	r.TemplateSHA = ""
	r.Checkout = filepath.Join(root, "checkout")
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Apply(context.Background(), p, ApplyOptions{Remote: fakeRemote{}, CommitModel: "gpt-6.1-sol"})
	if err == nil || !strings.Contains(err.Error(), "seed changed") {
		t.Fatalf("error=%v", err)
	}
	if _, err = os.Stat(r.Checkout); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("checkout created from drifted source")
	}
}

func TestRemoteRefsRejectExtraOrWrongBranches(t *testing.T) {
	r := valid()
	sha := strings.Repeat("a", 40)
	good := sha + "\trefs/heads/master\n" + sha + "\trefs/heads/develop"
	if verifyRefs(good, r, sha) != nil {
		t.Fatal("expected branches rejected")
	}
	if verifyRefs(good+"\n"+sha+"\trefs/heads/other", r, sha) == nil {
		t.Fatal("extra branch accepted")
	}
	if verifyRefs(strings.Replace(good, sha, strings.Repeat("b", 40), 1), r, sha) == nil {
		t.Fatal("drift accepted")
	}
}

type inspectedRemote struct {
	state   RemoteState
	ensures int
}

func (r *inspectedRemote) Inspect(context.Context, Request) (RemoteState, error) { return r.state, nil }

func (r *inspectedRemote) Ensure(context.Context, Request) error { r.ensures++; return nil }

func TestApplyRejectsDestinationIdentityDrift(t *testing.T) {
	r := valid()
	r.Checkout = filepath.Join(canonicalTemp(t), "site")
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.RemoteState = RemoteState{Exists: true, Empty: true, ProjectID: 10}
	p.PlanDigest = Digest(p)
	remote := &inspectedRemote{state: RemoteState{Exists: true, Empty: true, ProjectID: 11}}
	_, err = Apply(context.Background(), p, ApplyOptions{Remote: remote, CommitModel: "gpt-6.1-sol"})
	if err == nil || remote.ensures != 0 {
		t.Fatal("project identity drift allowed mutation")
	}
}

func TestPartialCopyPreservesModeAndRejectsUnexpectedEntries(t *testing.T) {
	root := canonicalTemp(t)
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	for _, dir := range []string{src, dst} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("echo fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dst, "run"))
	if err != nil || info.Mode()&0o100 == 0 {
		t.Fatal("executable bit lost")
	}
	if err = os.WriteFile(filepath.Join(dst, "other"), []byte("user work"), 0o600); err != nil {
		t.Fatal(err)
	}
	if validatePartialTree(src, dst) == nil {
		t.Fatal("unexpected user work accepted")
	}
	if err = os.WriteFile(filepath.Join(dst, "run"), []byte("user edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if copyTree(src, dst) == nil {
		t.Fatal("overwrote user edit")
	}
}

func TestPlanFilesAndRequestValidation(t *testing.T) {
	root := canonicalTemp(t)
	p, err := NewPlan(valid(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "plan.json")
	if err = WritePlan(path, p); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadPlan(path)
	if err != nil || loaded.PlanDigest != p.PlanDigest {
		t.Fatal("plan roundtrip failed")
	}
	p.Request.Project = "changed"
	if WritePlan(path, p) == nil {
		t.Fatal("wrote tampered plan")
	}
	if err = os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadPlan(path); err == nil {
		t.Fatal("loaded unbound plan")
	}
	link := filepath.Join(root, "link")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if ReadJSONFile(link, &p) == nil {
		t.Fatal("read symlink")
	}
	for _, change := range []func(*Request){func(r *Request) { r.DefaultBranch = "main" }, func(r *Request) { r.Checkout = "relative" }, func(r *Request) { r.TemplateSHA = "bad" }, func(r *Request) { r.Namespace = "" }, func(r *Request) { r.Site = nil }, func(r *Request) { r.SeedDir = root }} {
		r := valid()
		change(&r)
		if ValidateRequest(r) == nil {
			t.Fatal("invalid request accepted")
		}
	}
}

func TestApplyRejectsInvalidReceiptAndInputsWithoutMutation(t *testing.T) {
	root := canonicalTemp(t)
	r := valid()
	r.Checkout = filepath.Join(root, "site")
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	options := ApplyOptions{Remote: fakeRemote{}, CommitModel: "gpt-6.1-sol"}
	changed := p
	changed.PlanDigest = "wrong"
	if _, err = Apply(context.Background(), changed, options); err == nil {
		t.Fatal("tampered plan accepted")
	}
	options.CommitModel = "unversioned"
	if _, err = Apply(context.Background(), p, options); err == nil {
		t.Fatal("unversioned model accepted")
	}
	options.CommitModel = "gpt-6.1-sol"
	options.Remote = nil
	if _, err = Apply(context.Background(), p, options); err == nil {
		t.Fatal("missing remote accepted")
	}
	options.Remote = fakeRemote{}
	options.ReceiptPath = filepath.Join(root, "receipt")
	if err = writeReceipt(options.ReceiptPath, Receipt{PlanDigest: "other"}); err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(context.Background(), p, options); err == nil {
		t.Fatal("foreign receipt accepted")
	}
	if err = os.WriteFile(options.ReceiptPath, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(context.Background(), p, options); err == nil {
		t.Fatal("broken receipt accepted")
	}
	if _, err = os.Stat(r.Checkout); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid input created checkout")
	}
}

func TestSourceAndJSONFailuresRemainVisible(t *testing.T) {
	root := canonicalTemp(t)
	var v any
	if ReadJSONFile(filepath.Join(root, "missing"), &v) == nil {
		t.Fatal("missing input accepted")
	}
	if ReadJSONFile(root, &v) == nil {
		t.Fatal("directory input accepted")
	}
	large := filepath.Join(root, "large")
	if err := os.WriteFile(large, []byte(strings.Repeat("x", (1<<20)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if ReadJSONFile(large, &v) == nil {
		t.Fatal("oversized input accepted")
	}
	if writeSiteConfig(root, SiteConfig{"bad": make(chan int)}) == nil {
		t.Fatal("nonJSON site accepted")
	}
	if validateSite(SiteConfig{"bad": make(chan int)}) == nil {
		t.Fatal("nonJSON config accepted")
	}
	seed := filepath.Join(root, "seed")
	if err := os.Mkdir(seed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(large, filepath.Join(seed, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := seedDigest(seed); err == nil {
		t.Fatal("symlink seed accepted")
	}
	if copyTree(seed, filepath.Join(root, "dst")) == nil {
		t.Fatal("symlink copied")
	}
	if validateCheckoutPath(large) == nil {
		t.Fatal("file checkout accepted")
	}
	if validateAncestors("relative") == nil {
		t.Fatal("relative ancestor accepted")
	}
	if validateAncestors("/") == nil {
		t.Fatal("root checkout accepted")
	}
	if validateSite(SiteConfig{"array": []any{map[string]any{"secret": "x"}}}) == nil {
		t.Fatal("nested secret accepted")
	}
	if validateSite(SiteConfig{"array": []any{"safe"}}) != nil {
		t.Fatal("safe nested JSON rejected")
	}
}

func TestFinalizationRejectsChangedLocalAndRemoteState(t *testing.T) {
	for _, defect := range []string{"origin", "head", "provenance", "dirty", "remote", "push"} {
		t.Run(defect, func(t *testing.T) {
			root := canonicalTemp(t)
			r := valid()
			r.Checkout = root
			p, err := NewPlan(r, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			sha := strings.Repeat("a", 40)
			marker := p.PlanDigest
			if defect == "provenance" {
				marker = "other"
			}
			if err = os.WriteFile(filepath.Join(root, ".bootstrap-provenance.json"), []byte(marker), 0o600); err != nil {
				t.Fatal(err)
			}
			options := ApplyOptions{ReceiptPath: filepath.Join(canonicalTemp(t), "receipt.json"), Git: func(_ context.Context, _ string, args ...string) error {
				if defect == "push" {
					for _, arg := range args {
						if arg == "push" {
							return errors.New("injected push failure")
						}
					}
				}
				return nil
			}, GitOutput: func(_ context.Context, _ string, args ...string) (string, error) {
				switch args[0] {
				case "remote":
					if defect == "origin" {
						return "https://other.invalid/repo", nil
					}
					return r.Remote, nil
				case "rev-parse":
					if defect == "head" {
						return strings.Repeat("b", 40), nil
					}
					return sha, nil
				case "status":
					if defect == "dirty" {
						return " M index.html", nil
					}
					return "", nil
				case "ls-remote":
					if defect == "push" {
						return "", nil
					}
					return sha + " refs/heads/master", nil
				}
				return "", errors.New("unexpected fixture query")
			}}
			rec := Receipt{SchemaVersion: 1, PlanDigest: p.PlanDigest, Phase: "committed", InitialSHA: sha}
			got, err := finishApply(context.Background(), p, options, rec)
			if err == nil || got.Phase == "pushed" {
				t.Fatalf("accepted %s defect", defect)
			}
			if got.Error == "" {
				t.Fatal("failure disappeared from receipt")
			}
		})
	}
}

func TestPlanRejectsBadOriginsAndOverlappingSources(t *testing.T) {
	for _, origin := range []string{"https://user:secret@gitlab.example", "https://gitlab.example?token=x", "http://gitlab.example"} {
		r := valid()
		r.GitLabURL = origin
		if ValidateRequest(r) == nil {
			t.Fatal("credential/insecure API URL persisted")
		}
	}
	root := canonicalTemp(t)
	r := valid()
	r.SeedDir = root
	r.TemplateURL = ""
	r.TemplateSHA = ""
	r.Checkout = filepath.Join(root, "nested")
	if ValidateRequest(r) == nil {
		t.Fatal("overlapping seed accepted")
	}
	r = valid()
	r.RepoID = "UPPER"
	if ValidateRequest(r) == nil {
		t.Fatal("unsafe repository ID accepted")
	}
	r = valid()
	r.Namespace = "../group"
	if ValidateRequest(r) == nil {
		t.Fatal("namespace traversal accepted")
	}
}

type failedCreationRemote struct{ calls int }

func (r *failedCreationRemote) Ensure(context.Context, Request) error {
	r.calls++
	return errors.New("uncertain server outcome")
}

func TestUncertainCreationNeverBlindlyRetries(t *testing.T) {
	root := canonicalTemp(t)
	r := valid()
	r.Checkout = filepath.Join(root, "site")
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	remote := &failedCreationRemote{}
	options := ApplyOptions{Remote: remote, CommitModel: "gpt-6.1-sol"}
	receipt, err := Apply(context.Background(), p, options)
	if err == nil || receipt.Phase != "provisioning" {
		t.Fatalf("uncertain create phase=%s err=%v", receipt.Phase, err)
	}
	if _, err = Apply(context.Background(), p, options); err == nil {
		t.Fatal("uncertain creation resumed blindly")
	}
	if remote.calls != 1 {
		t.Fatal("retried creation")
	}
}

func TestReceiptFailuresAndRemoteStateValidation(t *testing.T) {
	root := canonicalTemp(t)
	if err := writeReceipt("", Receipt{}); err != nil {
		t.Fatal(err)
	}
	if writeReceipt(root, Receipt{}) == nil {
		t.Fatal("directory receipt overwritten")
	}
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if writeReceipt(filepath.Join(path, "receipt"), Receipt{}) == nil {
		t.Fatal("invalid receipt ancestor accepted")
	}
	if _, err := failReceipt(ApplyOptions{ReceiptPath: root}, Receipt{}, errors.New("original")); err == nil || !strings.Contains(err.Error(), "original") {
		t.Fatal("original error lost")
	}
	r := valid()
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	state := &inspectedRemote{state: RemoteState{Exists: true, Empty: false, ProjectID: 10}}
	options := ApplyOptions{Remote: state}
	if validateRemoteState(context.Background(), p, options, Receipt{Phase: "copying", ProjectID: 10}, true) == nil {
		t.Fatal("populated remote accepted before initial commit")
	}
	state.state = RemoteState{Exists: true, Empty: true, ProjectID: 11}
	if validateRemoteState(context.Background(), p, options, Receipt{Phase: "committed", ProjectID: 10}, true) == nil {
		t.Fatal("changed receipt project ID accepted")
	}
}

func TestTemplatePreparationFailureRetainsReceiptWithoutCheckout(t *testing.T) {
	for _, stage := range []string{"clone", "sha", "checkout"} {
		t.Run(stage, func(t *testing.T) {
			root := canonicalTemp(t)
			r := valid()
			r.Checkout = filepath.Join(root, "site")
			p, err := NewPlan(r, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			options := ApplyOptions{ReceiptPath: filepath.Join(root, "receipt.json"), Git: func(_ context.Context, _ string, args ...string) error {
				for _, arg := range args {
					if arg == stage {
						return errors.New("injected preparation failure")
					}
				}
				return nil
			}, GitOutput: func(context.Context, string, ...string) (string, error) {
				if stage == "sha" {
					return strings.Repeat("b", 40), nil
				}
				return r.TemplateSHA, nil
			}}
			receipt, err := copySource(context.Background(), p, options, Receipt{Phase: "remote_ready", PlanDigest: p.PlanDigest})
			if err == nil || receipt.Phase != "remote_ready" {
				t.Fatalf("preparation phase=%s err=%v", receipt.Phase, err)
			}
			if _, err = os.Stat(r.Checkout); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("checkout created before verified source")
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".bootstrap-template-") {
					t.Fatal("template temporary clone leaked")
				}
			}
		})
	}
}

func TestPreparedCheckoutEditsCannotEnterResumedCommit(t *testing.T) {
	root := canonicalTemp(t)
	r := valid()
	r.Checkout = root
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "index.html")
	if err = os.WriteFile(file, []byte("prepared"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := treeDigest(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, []byte("user changes"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	options := ApplyOptions{ReceiptPath: filepath.Join(canonicalTemp(t), "receipt"), Git: func(context.Context, string, ...string) error { called = true; return nil }}
	_, err = commitSite(context.Background(), p, options, Receipt{Phase: "copied", ContentDigest: digest})
	if err == nil || called {
		t.Fatal("edited prepared tree entered Git commit")
	}
}

func TestMaterializationFailureDoesNotClaimCopiedPhase(t *testing.T) {
	root := canonicalTemp(t)
	r := valid()
	r.Checkout = root
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(root, "site.config.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	options := ApplyOptions{ReceiptPath: filepath.Join(canonicalTemp(t), "receipt")}
	digest, err := treeDigest(r.Checkout, false)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := materializeSite(p, options, Receipt{Phase: "materialized", ContentDigest: digest})
	if err == nil || receipt.Phase != "materialized" {
		t.Fatal("failed configuration reported copied")
	}
}

func TestPlanBindsResolvedGitLabAPIOrigin(t *testing.T) {
	for _, fixture := range []struct{ remote, api string }{{"https://gitlab.polypia.net/group/repo.git", "https://gitlab.polypia.net"}, {"https://gitlab.polypia.net:8443/group/repo.git", "https://gitlab.polypia.net:8443"}, {"ssh://git@gitlab.polypia.net:2224/group/repo.git", "https://gitlab.polypia.net"}, {"ssh://git@[2001:db8::1]:2224/group/repo.git", "https://[2001:db8::1]"}, {"https://[2001:db8::1]:8443/group/repo.git", "https://[2001:db8::1]:8443"}} {
		t.Run(fixture.remote, func(t *testing.T) {
			r := valid()
			r.Remote = fixture.remote
			p, err := NewPlan(r, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if p.Request.GitLabURL != fixture.api {
				t.Fatalf("API origin=%s want=%s", p.Request.GitLabURL, fixture.api)
			}
			if err = ValidateGitLabRequest(p.Request); err != nil {
				t.Fatal(err)
			}
			changed := p
			changed.Request.GitLabURL = "https://gitlab.com"
			if Digest(changed) == p.PlanDigest {
				t.Fatal("resolved host not digest bound")
			}
			path := filepath.Join(canonicalTemp(t), "plan")
			if err = WritePlan(path, p); err != nil {
				t.Fatal(err)
			}
			if _, err = ReadPlan(path); err != nil {
				t.Fatal(err)
			}
			p.Request.GitLabURL = ""
			p.PlanDigest = Digest(p)
			if err = WritePlan(path, p); err != nil {
				t.Fatal(err)
			}
			if _, err = ReadPlan(path); err == nil {
				t.Fatal("unnormalized persisted plan accepted")
			}
		})
	}
	r := valid()
	r.GitLabURL = "https://api.operator-approved.example/"
	p, err := NewPlan(r, time.Now())
	if err != nil || p.Request.GitLabURL != "https://api.operator-approved.example" {
		t.Fatalf("explicit API origin lost: %v", err)
	}
	r = valid()
	r.Remote = "file:///tmp/remote.git"
	p, err = NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if ValidateGitLabRequest(p.Request) == nil {
		t.Fatal("file fixture sent to authenticated adapter")
	}
	for _, api := range []string{"https://gitlab.example/api/v4", "https://gitlab.example#fragment", "https://gitlab.example?token=x"} {
		r = valid()
		r.GitLabURL = api
		if _, err = NewPlan(r, time.Now()); err == nil {
			t.Fatal("unsafe API origin accepted")
		}
	}
	r = valid()
	r.Remote = "ssh://git:secret@gitlab.example/group/repo.git"
	if _, err = NewPlan(r, time.Now()); err == nil {
		t.Fatal("SSH password persisted")
	}
}

func TestTemplateContractFailuresRemainVisible(t *testing.T) {
	for _, defect := range []string{"repository-placeholder", "worker-placeholder", "provenance-output", "clone-parent"} {
		t.Run(defect, func(t *testing.T) {
			root := canonicalTemp(t)
			r := valid()
			r.Checkout = filepath.Join(root, "site")
			r.RepoID = "fixture"
			r.WorkerName = "fixture"
			if err := os.MkdirAll(filepath.Join(r.Checkout, ".ce"), 0o700); err != nil {
				t.Fatal(err)
			}
			repo := "repository-id: singlepageweb-template"
			worker := `{"name":"replace-with-worker-name"}`
			if defect == "repository-placeholder" {
				repo = "repository-id: unexpected"
			}
			if defect == "worker-placeholder" {
				worker = `{"name":"unexpected-worker"}`
			}
			if err := os.WriteFile(filepath.Join(r.Checkout, ".ce", "task-runtime.yaml"), []byte(repo), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(r.Checkout, "wrangler.jsonc"), []byte(worker), 0o600); err != nil {
				t.Fatal(err)
			}
			if defect == "provenance-output" {
				if err := os.Mkdir(filepath.Join(r.Checkout, ".bootstrap-provenance.json"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			p, err := NewPlan(r, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			options := ApplyOptions{ReceiptPath: filepath.Join(root, "receipt")}
			var receipt Receipt
			if defect == "clone-parent" {
				p.Request.Checkout = filepath.Join(root, "missing", "site")
				receipt, err = copySource(context.Background(), p, options, Receipt{Phase: "remote_ready"})
			} else {
				digest, digestErr := treeDigest(r.Checkout, false)
				if digestErr != nil {
					t.Fatal(digestErr)
				}
				receipt, err = materializeSite(p, options, Receipt{Phase: "materialized", ContentDigest: digest})
			}
			if err == nil || receipt.Phase == "copied" {
				t.Fatalf("template defect=%s phase=%s err=%v", defect, receipt.Phase, err)
			}
		})
	}
}

func TestResolvedOriginConfiguresAdapterAndEmptyCheckoutIsSafe(t *testing.T) {
	r := valid()
	r.Remote = "ssh://git@gitlab.polypia.net:2224/group/repo.git"
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewGitLabRemote(p.Request.GitLabURL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if actual := adapter.Client.BaseURL().String(); actual != "https://gitlab.polypia.net/api/v4/" {
		t.Fatalf("token would target unexpected API origin: %s", actual)
	}
	if err = validateCheckoutPath(canonicalTemp(t)); err != nil {
		t.Fatalf("empty checkout rejected: %v", err)
	}
	r = valid()
	r.Remote = "file:///tmp/repo.git"
	r.GitLabURL = "https://gitlab.example"
	if ValidateGitLabRequest(r) == nil {
		t.Fatal("file remote accepted with explicit API origin")
	}
	r = valid()
	r.Remote = "https://gitlab.example/repo.git"
	r.GitLabURL = "https://gitlab.example/api"
	if ValidateGitLabRequest(r) == nil {
		t.Fatal("API path accepted at CLI boundary")
	}
}

func TestSecretsRejectAPIKeySpellingAndNestedValues(t *testing.T) {
	for _, key := range []string{"apiKey", "api_key", "api-key", "API_KEY", "Api-Key"} {
		for _, site := range []SiteConfig{{key: "sensitive-value"}, {"nested": []any{map[string]any{key: "sensitive-value"}}}} {
			r := valid()
			r.Site = site
			if _, err := NewPlan(r, time.Now()); err == nil {
				t.Fatalf("secret key %s entered plan", key)
			}
		}
	}
}

func TestMaterializedResumePreservesSymlinkTargetsAndUserEdits(t *testing.T) {
	for _, entry := range []string{"site.config.json", ".ce", "wrangler.jsonc", ".bootstrap-provenance.json", "user-edit"} {
		t.Run(entry, func(t *testing.T) {
			root := canonicalTemp(t)
			seed := filepath.Join(root, "seed")
			if err := os.MkdirAll(filepath.Join(seed, ".ce"), 0o700); err != nil {
				t.Fatal(err)
			}
			for file, content := range map[string]string{"index.html": "original", ".ce/task-runtime.yaml": "repository-id: singlepageweb-template", "wrangler.jsonc": `{"name":"replace-with-worker-name"}`} {
				if err := os.WriteFile(filepath.Join(seed, file), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			r := valid()
			r.TemplateURL = ""
			r.TemplateSHA = ""
			r.SeedDir = seed
			r.Checkout = filepath.Join(root, "checkout")
			p, err := NewPlan(r, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			options := ApplyOptions{Remote: fakeRemote{}, CommitModel: "gpt-6.1-sol", ReceiptPath: filepath.Join(root, "receipt.json")}
			receipt, err := copySource(context.Background(), p, options, Receipt{Phase: "remote_ready", PlanDigest: p.PlanDigest})
			if err != nil || receipt.Phase != "materialized" {
				t.Fatalf("preparation=%s err=%v", receipt.Phase, err)
			}
			outside := filepath.Join(root, "outside")
			if err = os.Mkdir(outside, 0o700); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(outside, "sentinel")
			if entry == ".ce" {
				sentinel = filepath.Join(outside, "task-runtime.yaml")
			}
			if err = os.WriteFile(sentinel, []byte("outside sentinel"), 0o600); err != nil {
				t.Fatal(err)
			}
			if entry == "user-edit" {
				if err = os.WriteFile(filepath.Join(r.Checkout, "wrangler.jsonc"), []byte("user change"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				path := filepath.Join(r.Checkout, entry)
				target := sentinel
				if entry == ".ce" {
					target = outside
					if err = os.Rename(path, filepath.Join(root, "retained-ce")); err != nil {
						t.Fatal(err)
					}
				} else if _, statErr := os.Lstat(path); statErr == nil {
					if err = os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				if err = os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = Apply(context.Background(), p, options); err == nil {
				t.Fatalf("changed materialized entry %s accepted", entry)
			}
			b, err := os.ReadFile(sentinel)
			if err != nil || string(b) != "outside sentinel" {
				t.Fatalf("outside sentinel overwritten: %s %v", b, err)
			}
			if entry == "user-edit" {
				b, err = os.ReadFile(filepath.Join(r.Checkout, "wrangler.jsonc"))
				if err != nil || string(b) != "user change" {
					t.Fatal("user change overwritten")
				}
			}
		})
	}
}

func TestInitialSeedRemainsUnconfiguredAndCEIsCommitted(t *testing.T) {
	isolateGitIdentity(t)
	root := canonicalTemp(t)
	seed := filepath.Join(root, "seed")
	bare := filepath.Join(root, "remote.git")
	if err := os.MkdirAll(filepath.Join(seed, ".ce"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{".ce/task-runtime.yaml": "repository-id: singlepageweb-template", "wrangler.jsonc": `{"name":"replace-with-worker-name"}`, "index.html": "brand free template"} {
		if err := os.WriteFile(filepath.Join(seed, path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := execGit(context.Background(), root, "init", "--bare", bare); err != nil {
		t.Fatal(err)
	}
	r := valid()
	r.SeedDir = seed
	r.TemplateURL = ""
	r.TemplateSHA = ""
	r.Site = nil
	r.WorkerName = ""
	r.Checkout = filepath.Join(root, "template")
	r.Remote = "file://" + bare
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.Request.WorkerName != "" {
		t.Fatal("worker assigned to brand-free seed")
	}
	if _, err = Apply(context.Background(), p, ApplyOptions{Remote: fakeRemote{}, CommitModel: "gpt-6.1-sol"}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(r.Checkout, "site.config.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("real site configuration created for initial template")
	}
	worker, err := os.ReadFile(filepath.Join(r.Checkout, "wrangler.jsonc"))
	if err != nil || string(worker) != `{"name":"replace-with-worker-name"}` {
		t.Fatal("worker placeholder replaced")
	}
	declaration, err := execGitOutput(context.Background(), r.Checkout, "show", "HEAD:.ce/task-runtime.yaml")
	if err != nil || !strings.Contains(declaration, "repository-id: "+r.Project) {
		t.Fatal("CE declaration missing from initial commit")
	}
	r = valid()
	r.Site = SiteConfig{}
	if _, err = NewPlan(r, time.Now()); err == nil {
		t.Fatal("normal bootstrap accepted missing site")
	}
	if err = os.WriteFile(filepath.Join(seed, "site.config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	r = p.Request
	if _, err = NewPlan(r, time.Now()); err == nil {
		t.Fatal("configured root file admitted into brand-free seed")
	}
}

func TestMaterializationAtomicReplacementPreservesHardlinkTarget(t *testing.T) {
	root := canonicalTemp(t)
	checkout := filepath.Join(root, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, ".ce"), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside-sentinel")
	if err := os.WriteFile(outside, []byte(`{"name":"replace-with-worker-name"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(checkout, "wrangler.jsonc")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, ".ce", "task-runtime.yaml"), []byte("repository-id: singlepageweb-template"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := valid()
	r.Checkout = checkout
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	digest, err := treeDigest(checkout, false)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := materializeSite(p, ApplyOptions{ReceiptPath: filepath.Join(root, "receipt")}, Receipt{Phase: "materialized", ContentDigest: digest})
	if err != nil || receipt.Phase != "copied" {
		t.Fatalf("materialization=%s err=%v", receipt.Phase, err)
	}
	sentinel, err := os.ReadFile(outside)
	if err != nil || string(sentinel) != `{"name":"replace-with-worker-name"}` {
		t.Fatal("outside hardlink inode was truncated")
	}
	inside, err := os.ReadFile(filepath.Join(checkout, "wrangler.jsonc"))
	if err != nil || !strings.Contains(string(inside), `"name": "`+r.Project+`"`) {
		t.Fatal("inside configuration not personalized")
	}
}

func TestPersonalizationUsesExactPropertiesForPrefixIdentifiers(t *testing.T) {
	root := canonicalTemp(t)
	if err := os.Mkdir(filepath.Join(root, ".ce"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ce", "task-runtime.yaml"), []byte("# singlepageweb appears in documentation\nrepository-id: singlepageweb-template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wrangler.jsonc"), []byte(`{"name":"replace-with-worker-name","description":"replace also appears here"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := personalizeFile(root, ".ce/task-runtime.yaml", "repository-id: singlepageweb-template", "repository-id: singlepageweb"); err != nil {
		t.Fatal(err)
	}
	if err := personalizeFile(root, "wrangler.jsonc", "replace-with-worker-name", "replace"); err != nil {
		t.Fatal(err)
	}
	repo, err := os.ReadFile(filepath.Join(root, ".ce", "task-runtime.yaml"))
	if err != nil || !strings.Contains(string(repo), "\nrepository-id: singlepageweb\n") {
		t.Fatalf("prefix repo ID not applied: %s", repo)
	}
	worker, err := os.ReadFile(filepath.Join(root, "wrangler.jsonc"))
	if err != nil || !strings.Contains(string(worker), `"name": "replace"`) {
		t.Fatalf("prefix worker name not applied: %s", worker)
	}
	if err := personalizeFile(root, "wrangler.jsonc", "replace-with-worker-name", "replace"); err != nil {
		t.Fatal("exact already-applied value rejected")
	}
	for _, source := range []string{`{"name":"other-worker","comment":"replace"}`, `{"name":42}`, `{bad`} {
		if _, err := personalizeWorkerConfig([]byte(source), "replace-with-worker-name", "replace"); err == nil {
			t.Fatal("nonmatching worker property accepted")
		}
	}
	for _, source := range []string{"# repository-id: singlepageweb", "repository-id: unexpected", "repository-id: singlepageweb\nrepository-id: singlepageweb"} {
		if _, err := personalizeRepositoryConfig([]byte(source), "repository-id: singlepageweb-template", "repository-id: singlepageweb"); err == nil {
			t.Fatal("nonmatching repository identity accepted")
		}
	}
}

type replacementRemote struct {
	inspected   int
	provisioned bool
	strict      bool
}

func (r *replacementRemote) Ensure(context.Context, Request) error { r.provisioned = true; return nil }

func (r *replacementRemote) Inspect(context.Context, Request) (RemoteState, error) {
	r.inspected++
	id := int64(10)
	if r.provisioned {
		id = 11
	}
	return RemoteState{Exists: true, Empty: true, ProjectID: id}, nil
}

func TestEnsureRechecksExistingProjectIdentityBeforeCopy(t *testing.T) {
	root := canonicalTemp(t)
	r := valid()
	r.Checkout = filepath.Join(root, "site")
	p, err := NewPlan(r, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.RemoteState = RemoteState{Exists: true, Empty: true, ProjectID: 10}
	p.PlanDigest = Digest(p)
	remote := &replacementRemote{}
	receipt, err := Apply(context.Background(), p, ApplyOptions{Remote: remote, CommitModel: "gpt-6.1-sol"})
	if err == nil || receipt.ProjectID == 11 {
		t.Fatal("replacement project ID adopted during Ensure")
	}
	if _, err = os.Stat(r.Checkout); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("copied into replaced destination")
	}
}

type strictProvisioningFixture struct {
	result, observed, expected RemoteState
	err, inspectErr            error
	legacyCalled               bool
}

func (r *strictProvisioningFixture) Ensure(context.Context, Request) error {
	r.legacyCalled = true
	return errors.New("legacy creation called")
}

func (r *strictProvisioningFixture) EnsureProject(_ context.Context, _ Request, expected RemoteState) (RemoteState, error) {
	r.expected = expected
	return r.result, r.err
}

func (r *strictProvisioningFixture) Inspect(context.Context, Request) (RemoteState, error) {
	return r.observed, r.inspectErr
}

func TestProvisioningBindsReturnedAndObservedIdentity(t *testing.T) {
	expected := RemoteState{Exists: true, Empty: true, ProjectID: 10}
	for _, fixture := range []struct {
		name                       string
		result, observed           RemoteState
		creationErr, inspectionErr error
		valid                      bool
	}{
		{name: "exact-existing", result: expected, observed: expected, valid: true},
		{name: "changed-returned-id", result: RemoteState{Exists: true, Empty: true, ProjectID: 11}, observed: expected},
		{name: "changed-after-creation", result: expected, observed: RemoteState{Exists: true, Empty: true, ProjectID: 11}},
		{name: "zero-id", result: RemoteState{Exists: true, Empty: true}, observed: expected},
		{name: "missing-result", result: RemoteState{}, observed: expected},
		{name: "populated-result", result: RemoteState{Exists: true, ProjectID: 10}, observed: expected},
		{name: "creation-error", creationErr: errors.New("ambiguous creation")},
		{name: "inspection-error", result: expected, inspectionErr: errors.New("inspection unavailable")},
		{name: "populated-after-creation", result: expected, observed: RemoteState{Exists: true, ProjectID: 10}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			p, err := NewPlan(valid(), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			p.RemoteState = expected
			remote := &strictProvisioningFixture{result: fixture.result, observed: fixture.observed, err: fixture.creationErr, inspectErr: fixture.inspectionErr}
			id, err := provisionDestination(context.Background(), p, ApplyOptions{Remote: remote})
			if fixture.valid {
				if err != nil || id != 10 {
					t.Fatalf("identity=%d err=%v", id, err)
				}
			} else if err == nil {
				t.Fatal("unsafe provisioning accepted")
			}
			if remote.legacyCalled || remote.expected != expected {
				t.Fatal("strict identity-bound adapter bypassed")
			}
		})
	}
}

func TestPlanRejectsAuthenticationFieldVariantsWithoutRejectingProse(t *testing.T) {
	for _, key := range []string{"authorization", "Authorization", "AUTHORIZATION", "auth_header", "Auth-Header", "authHeader", "access_key", "ACCESS-KEY", "accessKey", "session_key", "Session-Key", "sessionKey"} {
		for _, site := range []SiteConfig{{key: "Bearer synthetic-private-value"}, {"nested": map[string]any{key: "synthetic-private-value"}}, {"nested": []any{map[string]any{key: "synthetic-private-value"}}}} {
			r := valid()
			r.Site = site
			if _, err := NewPlan(r, time.Now()); err == nil {
				t.Fatalf("authentication field %s was accepted into a plan", key)
			}
		}
	}
	r := valid()
	r.Site = SiteConfig{"description": "Documentation explains authorization headers, access keys and session keys without containing credentials.", "displayName": "Authorization reference"}
	if _, err := NewPlan(r, time.Now()); err != nil {
		t.Fatalf("ordinary descriptive prose rejected: %v", err)
	}
}

func TestPublishedRefsAllowOnlyMatchingGitLabPipelineIDs(t *testing.T) {
	r := valid()
	sha := strings.Repeat("a", 40)
	other := strings.Repeat("b", 40)
	heads := sha + "\trefs/heads/master\n" + sha + "\trefs/heads/develop"
	for _, fixture := range []struct {
		name, refs string
		accepted   bool
	}{
		{"exact-heads", heads, true},
		{"pipeline", heads + "\n" + sha + "\trefs/pipelines/1", true},
		{"multiple-pipelines", heads + "\n" + sha + "\trefs/pipelines/26880\n" + sha + "\trefs/pipelines/26881", true},
		{"pipeline-other-sha", heads + "\n" + other + "\trefs/pipelines/26880", false},
		{"tag", heads + "\n" + sha + "\trefs/tags/v1", false},
		{"notes", heads + "\n" + sha + "\trefs/notes/commits", false},
		{"extra-head", heads + "\n" + sha + "\trefs/heads/feature", false},
		{"non-numeric", heads + "\n" + sha + "\trefs/pipelines/latest", false},
		{"zero-id", heads + "\n" + sha + "\trefs/pipelines/0", false},
		{"negative-id", heads + "\n" + sha + "\trefs/pipelines/-1", false},
		{"noncanonical-id", heads + "\n" + sha + "\trefs/pipelines/01", false},
		{"nested-pipeline", heads + "\n" + sha + "\trefs/pipelines/1/extra", false},
		{"pipeline-only", sha + "\trefs/pipelines/26880", false},
		{"partial-heads", sha + "\trefs/heads/master\n" + sha + "\trefs/pipelines/26880", false},
		{"wrong-head-sha", other + "\trefs/heads/master\n" + sha + "\trefs/heads/develop", false},
		{"malformed", heads + "\nmalformed-ref-line", false},
		{"duplicate", heads + "\n" + sha + "\trefs/heads/master", false},
		{"empty", "", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			err := verifyPublishedRefs(fixture.refs, r, sha)
			if (err == nil) != fixture.accepted {
				t.Fatalf("published verification=%v accepted=%v", err, fixture.accepted)
			}
			if fixture.name != "exact-heads" && verifyRefs(fixture.refs, r, sha) == nil {
				t.Fatal("strict initial ref verifier accepted additional or incomplete refs")
			}
		})
	}
}
