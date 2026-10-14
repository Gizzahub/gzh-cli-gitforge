package forgebootstrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplyLocalBareFixtureCreatesIndependentBranches(t *testing.T) {
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
	git(template, "add", "--all")
	git(template, "commit", "-m", "template")
	sha := git(template, "rev-parse", "HEAD")
	r := valid()
	r.TemplateURL = "file://" + template
	r.TemplateSHA = sha
	r.Remote = "file://" + bare
	r.Checkout = checkout
	p, err := NewPlan(r, time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Apply(context.Background(), p, ApplyOptions{Remote: remoteFunc(func(context.Context, Request) error { return nil }), CommitModel: "GPT-6"})
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
	if got := git(checkout, "rev-parse", "refs/heads/master"); got != got.InitialSHA {
		t.Fatalf("master = %s, initial = %s", got, got.InitialSHA)
	}
	git(checkout, "show-ref", "refs/remotes/origin/develop")
}
