package forgebootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Receipt struct {
	SchemaVersion int       `json:"schema_version"`
	PlanDigest    string    `json:"plan_digest"`
	Phase         string    `json:"phase"`
	InitialSHA    string    `json:"initial_sha,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	Error         string    `json:"error,omitempty"`
}
type Remote interface {
	Ensure(context.Context, Request) error
}
type ApplyOptions struct {
	Remote      Remote
	ReceiptPath string
	Git         func(context.Context, ...string) error
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
		ents, _ := os.ReadDir(r.Checkout)
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
		g = execGit
	}
	rec := Receipt{SchemaVersion: 1, PlanDigest: p.PlanDigest, Phase: "remote_ready", UpdatedAt: time.Now().UTC()}
	_ = writeReceipt(o.ReceiptPath, rec)
	tmp, e := os.MkdirTemp("", "gz-git-template-")
	if e != nil {
		return rec, e
	}
	defer os.RemoveAll(tmp)
	if e = g(ctx, "clone", "--no-checkout", r.TemplateURL, tmp); e != nil {
		return rec, e
	}
	if e = g(ctx, "-C", tmp, "checkout", "--detach", r.TemplateSHA); e != nil {
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
	for _, args := range [][]string{{"-C", r.Checkout, "init", "--initial-branch", r.DefaultBranch}, {"-C", r.Checkout, "remote", "add", "origin", r.Remote}, {"-C", r.Checkout, "add", "--all"}, {"-C", r.Checkout, "commit", "-m", "chore(bootstrap): initialize from template"}, {"-C", r.Checkout, "push", "-u", "origin", r.DefaultBranch}} {
		if e := g(ctx, args...); e != nil {
			return rec, e
		}
	}
	out, e := exec.CommandContext(ctx, "git", "-C", r.Checkout, "rev-parse", "HEAD").Output()
	if e != nil {
		return rec, e
	}
	rec.InitialSHA = strings.TrimSpace(string(out))
	rec.Phase = "pushed"
	rec.UpdatedAt = time.Now().UTC()
	_ = writeReceipt(o.ReceiptPath, rec)
	return rec, nil
}

func execGit(ctx context.Context, args ...string) error {
	c := exec.CommandContext(ctx, "git", args...)
	c.Stdout = nil
	c.Stderr = nil
	return c.Run()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(src, path)
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
