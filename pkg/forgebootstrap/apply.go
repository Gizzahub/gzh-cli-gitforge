package forgebootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

type Receipt struct {
	SchemaVersion int       `json:"schema_version"`
	PlanDigest    string    `json:"plan_digest"`
	Phase         string    `json:"phase"`
	InitialSHA    string    `json:"initial_sha,omitempty"`
	TemplateURL   string    `json:"template_url"`
	TemplateSHA   string    `json:"template_sha"`
	SiteDigest    string    `json:"site_digest"`
	UpdatedAt     time.Time `json:"updated_at"`
	Error         string    `json:"error,omitempty"`
}
type Remote interface {
	Ensure(context.Context, Request) error
}
type ApplyOptions struct {
	Remote      Remote
	ReceiptPath string
	Git         func(context.Context, string, ...string) error
	GitOutput   func(context.Context, string, ...string) (string, error)
}

func Apply(ctx context.Context, p Plan, o ApplyOptions) (Receipt, error) {
	if p.PlanDigest != Digest(p) {
		return Receipt{}, errors.New("plan digest mismatch")
	}
	if e := ValidateRequest(p.Request); e != nil {
		return Receipt{}, e
	}
	r := p.Request
	if st, e := os.Stat(r.Checkout); e == nil {
		if !st.IsDir() {
			return Receipt{}, errors.New("checkout path is occupied")
		}
		ents, e := os.ReadDir(r.Checkout)
		if e != nil {
			return Receipt{}, e
		}
		if len(ents) > 0 {
			return Receipt{}, errors.New("checkout path is occupied")
		}
	} else if !os.IsNotExist(e) {
		return Receipt{}, e
	}
	if o.Remote == nil {
		return Receipt{}, errors.New("remote client is required")
	}
	if e := o.Remote.Ensure(ctx, r); e != nil {
		return Receipt{}, fmt.Errorf("ensure remote: %w", e)
	}
	g := o.Git
	if g == nil {
		g = func(c context.Context, _ string, a ...string) error { return execGit(c, r.Checkout, a...) }
	}
	rec := Receipt{SchemaVersion: 1, PlanDigest: p.PlanDigest, Phase: "remote_ready", TemplateURL: r.TemplateURL, TemplateSHA: r.TemplateSHA, SiteDigest: TemplateDigest(r), UpdatedAt: time.Now().UTC()}
	if e := writeReceipt(o.ReceiptPath, rec); e != nil {
		return rec, e
	}
	tmp, e := os.MkdirTemp("", "gz-git-template-")
	if e != nil {
		return rec, e
	}
	defer os.RemoveAll(tmp)
	if e = g(ctx, "", "clone", "--no-checkout", r.TemplateURL, tmp); e != nil {
		return rec, e
	}
	if e := g(ctx, tmp, "checkout", "--detach", r.TemplateSHA); e != nil {
		return rec, e
	}
	if e := copyTree(tmp, r.Checkout); e != nil {
		return rec, e
	}
	if e := os.RemoveAll(filepath.Join(r.Checkout, ".git")); e != nil {
		return rec, e
	}
	if e := writeSiteConfig(r.Checkout, r.Site); e != nil {
		return rec, e
	}
	if r.DefaultBranch != "master" || r.IntegrationBranch != "develop" {
		return rec, errors.New("bootstrap supports master default and develop integration branches only")
	}
	for _, args := range [][]string{{"-C", r.Checkout, "init", "--initial-branch", r.DefaultBranch}, {"-C", r.Checkout, "remote", "add", "origin", r.Remote}, {"-C", r.Checkout, "add", "--all"}, {"-C", r.Checkout, "commit", "-m", "chore(bootstrap): initialize from template"}, {"-C", r.Checkout, "push", "-u", "origin", r.DefaultBranch}, {"-C", r.Checkout, "branch", r.IntegrationBranch}, {"-C", r.Checkout, "push", "-u", "origin", r.IntegrationBranch}} {
		if e := g(ctx, r.Checkout, args...); e != nil {
			return rec, e
		}
	}
	if e := writeReceipt(o.ReceiptPath, rec); e != nil {
		return rec, e
	}
	if o.GitOutput == nil {
		o.GitOutput = execGitOutput
	}
	if o.GitOutput != nil {
		out, e := o.GitOutput(ctx, r.Checkout, "rev-parse", "HEAD")
		if e != nil {
			return rec, e
		}
		rec.InitialSHA = strings.TrimSpace(out)
	}
	rec.Phase = "pushed"
	rec.UpdatedAt = time.Now().UTC()
	if e := writeReceipt(o.ReceiptPath, rec); e != nil {
		return rec, e
	}
	return rec, nil
}

func execGit(ctx context.Context, dir string, args ...string) error {
	r, err := gitcmd.NewExecutor().Run(ctx, dir, args...)
	if err != nil {
		return err
	}
	if r.Error != nil {
		return fmt.Errorf("git failed: %w: %s", r.Error, strings.TrimSpace(r.Stderr))
	}
	if r.ExitCode != 0 {
		return fmt.Errorf("git exited %d: %s", r.ExitCode, strings.TrimSpace(r.Stderr))
	}
	return nil
}

func execGitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	r, err := gitcmd.NewExecutor().Run(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	if r.Error != nil {
		return "", r.Error
	}
	if r.ExitCode != 0 {
		return "", fmt.Errorf("git exited %d: %s", r.ExitCode, strings.TrimSpace(r.Stderr))
	}
	return r.Stdout, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(to, b, 0o644)
	})
}

func writeSiteConfig(root string, s SiteConfig) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, ".site-config.json"), append(b, '\n'), 0o644)
}

func writeReceipt(path string, r Receipt) error {
	if path == "" {
		return nil
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0o755); e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
