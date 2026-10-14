package forgebootstrap

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplyLocalBareFixtureCreatesIndependentBranches(t *testing.T) {
	ignoreRoot := canonicalTemp(t)
	exclude := filepath.Join(ignoreRoot, "global-ignore")
	if err := os.WriteFile(exclude, []byte("site.config.json\n.ce\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	globalConfig := filepath.Join(ignoreRoot, "gitconfig")
	if err := os.WriteFile(globalConfig, []byte("[user]\n name = fixture\n email = fixture@example.invalid\n[core]\n excludesFile = "+exclude+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)

	template := filepath.Join(t.TempDir(), "template")
	bare := filepath.Join(t.TempDir(), "remote.git")
	checkout := filepath.Join(t.TempDir(), "site")
	for _, d := range []string{template, bare} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	git := func(dir string, args ...string) string {
		c := exec.CommandContext(context.Background(), "git", args...)
		c.Dir = dir
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %s", args, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("", "init", "--bare", bare)
	git(template, "init", "--initial-branch", "master")
	git(template, "config", "user.email", "test@example.invalid")
	git(template, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(template, "index.html"), []byte("template"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(template, ".ce"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(template, ".ce", "task-runtime.yaml"), []byte("repository-id: singlepageweb-template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(template, "wrangler.jsonc"), []byte(`{"name":"replace-with-worker-name"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(template, "tests", "fixtures"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(template, "tests", "fixtures", "site.config.json"), []byte("fixture source"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(template, "add", "--force", "tests/fixtures/site.config.json")
	git(template, "add", "--force", ".ce/task-runtime.yaml", "wrangler.jsonc")
	git(template, "add", "--all")
	git(template, "commit", "-m", "template")
	sha := git(template, "rev-parse", "HEAD")
	r := valid()
	r.TemplateURL = "file://" + template
	r.TemplateSHA = sha
	r.Remote = "file://" + bare
	checkout, err := filepath.EvalSymlinks(filepath.Dir(checkout))
	if err != nil {
		t.Fatal(err)
	}
	checkout = filepath.Join(checkout, "site")
	r.Checkout = checkout
	p, err := NewPlan(r, time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	remote := remoteFunc(func(context.Context, Request) error { return nil })
	failedPush := false
	options := ApplyOptions{Remote: remote, CommitModel: "gpt-6.1-sol", Git: func(ctx context.Context, dir string, args ...string) error {
		for _, arg := range args {
			if arg == "push" && !failedPush {
				failedPush = true
				return errors.New("injected push failure")
			}
		}
		return execGit(ctx, dir, args...)
	}}
	partial, err := Apply(context.Background(), p, options)
	if err == nil || partial.Phase != "committed" {
		t.Fatalf("expected retained committed phase: %v %s", err, partial.Phase)
	}
	git(bare, "fetch", checkout, "HEAD")
	git(bare, "update-ref", "refs/tags/preexisting", partial.InitialSHA)
	if _, err := Apply(context.Background(), p, options); err == nil {
		t.Fatal("remote tag content accepted before push")
	}
	if heads := git(bare, "for-each-ref", "refs/heads"); heads != "" {
		t.Fatal("pushed branches despite existing tag")
	}
	git(bare, "update-ref", "-d", "refs/tags/preexisting")
	got, err := Apply(context.Background(), p, options)
	if err != nil {
		t.Fatal(err)
	}
	if got.InitialSHA == "" {
		t.Fatal("missing initial SHA")
	}
	if _, err := os.Stat(filepath.Join(checkout, ".git")); err != nil {
		t.Fatal("independent repository missing")
	}
	if _, err := os.Stat(filepath.Join(checkout, "site.config.json")); err != nil {
		t.Fatal("site config missing")
	}
	if master := git(checkout, "rev-parse", "refs/heads/master"); master != got.InitialSHA {
		t.Fatalf("master = %s, initial = %s", master, got.InitialSHA)
	}
	git(checkout, "show-ref", "refs/remotes/origin/develop")
	if copied := git(checkout, "show", "HEAD:tests/fixtures/site.config.json"); copied != "fixture source" {
		t.Fatal("globalignore omitted pinned source fixture from initial commit")
	}
	if count := git(checkout, "rev-list", "--count", "HEAD"); count != "1" {
		t.Fatalf("copied template history: %s", count)
	}
	if message := git(checkout, "log", "-1", "--format=%B"); !strings.HasSuffix(message, "Model: gpt-6.1-sol") {
		t.Fatalf("missing runtime footer: %s", message)
	}
	repeated, err := Apply(context.Background(), p, options)
	if err != nil || repeated.InitialSHA != got.InitialSHA {
		t.Fatalf("completed resume changed SHA: %v", err)
	}
	git(checkout, "tag", "unexpected", got.InitialSHA)
	git(checkout, "push", "origin", "refs/tags/unexpected")
	if _, err := Apply(context.Background(), p, options); err == nil {
		t.Fatal("remote tag ignored after push")
	}
	git(bare, "update-ref", "-d", "refs/tags/unexpected")
	git(checkout, "notes", "add", "-m", "fixture note")
	git(checkout, "push", "origin", "refs/notes/commits")
	if _, err := Apply(context.Background(), p, options); err == nil {
		t.Fatal("remote notes ignored after push")
	}
}

func TestSeedApplyResumesEveryGitMutationStage(t *testing.T) {
	for _, stage := range []string{"init", "remote", "add", "commit", "push"} {
		t.Run(stage, func(t *testing.T) {
			root := canonicalTemp(t)
			seed := filepath.Join(root, "seed")
			bare := filepath.Join(root, "remote.git")
			if err := os.MkdirAll(filepath.Join(seed, ".ce"), 0o700); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{"index.html": "brand free", ".ce/task-runtime.yaml": "schema-version: 2\nrepository-id: singlepageweb-template\n", "wrangler.jsonc": `{"name":"replace-with-worker-name"}`}
			for path, content := range files {
				if err := os.WriteFile(filepath.Join(seed, path), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := execGit(context.Background(), root, "init", "--bare", bare); err != nil {
				t.Fatal(err)
			}
			r := valid()
			r.TemplateURL = ""
			r.TemplateSHA = ""
			r.SeedDir = seed
			r.Checkout = filepath.Join(root, "site")
			r.Remote = "file://" + bare
			r.RepoID = "fixture-site"
			r.WorkerName = "fixture-worker"
			p, err := NewPlan(r, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			failed := false
			options := ApplyOptions{Remote: fakeRemote{}, CommitModel: "gpt-6.1-sol", Git: func(ctx context.Context, dir string, args ...string) error {
				for _, arg := range args {
					if arg == stage && !failed {
						failed = true
						return errors.New("injected stage failure")
					}
				}
				return execGit(ctx, dir, args...)
			}}
			partial, err := Apply(context.Background(), p, options)
			if err == nil || partial.Phase == "pushed" {
				t.Fatalf("missing failure at %s", stage)
			}
			receipt, err := Apply(context.Background(), p, options)
			if err != nil {
				t.Fatalf("resume %s: %v", stage, err)
			}
			if receipt.Phase != "pushed" || receipt.InitialSHA == "" {
				t.Fatal("incomplete receipt")
			}
			for path, want := range map[string]string{".ce/task-runtime.yaml": "repository-id: fixture-site", "wrangler.jsonc": "fixture-worker", "site.config.json": `"name": "template"`} {
				b, err := os.ReadFile(filepath.Join(r.Checkout, path))
				if err != nil || !strings.Contains(string(b), want) {
					t.Fatalf("personalization %s: %v %s", path, err, b)
				}
			}
			if stage == "push" {
				if err := os.WriteFile(filepath.Join(r.Checkout, "index.html"), []byte("user edit"), 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := Apply(context.Background(), p, options); err == nil {
					t.Fatal("changed checkout accepted on resume")
				}
			}
		})
	}
}
