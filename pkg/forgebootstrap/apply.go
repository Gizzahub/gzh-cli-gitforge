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

// Receipt records bootstrap provenance and progress.
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

// Remote provisions or reuses the destination project.
type Remote interface {
	Ensure(context.Context, Request) error
}

// ApplyOptions controls bootstrap execution.
type ApplyOptions struct {
	Remote      Remote
	ReceiptPath string
	CommitModel string
	Git         func(context.Context, string, ...string) error
	GitOutput   func(context.Context, string, ...string) (string, error)
}

// Apply initializes an independent repository from a pinned template.
func Apply(ctx context.Context, p Plan, o ApplyOptions) (Receipt, error) {
	if p.PlanDigest != Digest(p) {
		return Receipt{}, errors.New("plan digest mismatch")
	}
	if e := ValidateRequest(p.Request); e != nil {
		return Receipt{}, e
	}
	r := p.Request
	if e := validateCheckoutPath(r.Checkout); e != nil {
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
		g = execGit
	}
	if o.CommitModel == "" {
		return Receipt{}, errors.New("commit model is required")
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
	if e := g(ctx, "", "clone", "--no-checkout", r.TemplateURL, tmp); e != nil {
		return rec, e
	}
	if e := g(ctx, tmp, "checkout", "--detach", r.TemplateSHA); e != nil {
		return rec, e
	}
	if e := os.MkdirAll(r.Checkout, 0o700); e != nil {
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

func validateCheckoutPath(path string) error {
	if st, e := os.Lstat(path); e == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			return errors.New("checkout path is a symlink")
		}
		if !st.IsDir() {
			return errors.New("checkout path is occupied")
		}
		ents, e := os.ReadDir(path)
		if e != nil {
			return e
		}
		if len(ents) > 0 {
			return errors.New("checkout path is occupied")
		}
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	parent := filepath.Dir(path)
	if st, e := os.Lstat(parent); e == nil && st.Mode()&os.ModeSymlink != 0 {
		return errors.New("checkout ancestor is a symlink")
	}
	return nil
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
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("template contains symlink: %s", rel)
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
			return os.MkdirAll(to, 0o700)
		}
		b, e := os.ReadFile(path) // #nosec G304,G122 -- path is produced by WalkDir under the validated template root
		if e != nil {
			return e
		}
		mode := d.Type().Perm()
		if mode&0o100 != 0 {
			mode = 0o600 | (mode & 0o111)
		} else {
			mode = 0o600
		}
		return os.WriteFile(to, b, mode) // #nosec G703 -- destination is filepath.Join of validated relative path
	})
}

func writeSiteConfig(root string, s SiteConfig) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, "site.config.json"), append(b, '\n'), 0o600)
}

func writeReceipt(path string, r Receipt) error {
	if path == "" {
		return nil
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0o700); e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
