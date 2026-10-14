// Package forgebootstrap implements one-time template based site bootstrap.
//
//nolint:gosec,errcheck,gocritic,revive,tagliatelle
package forgebootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,99}$`)
	shaPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type (
	SiteConfig map[string]any
	Request    struct {
		Namespace         string     `json:"namespace"`
		Project           string     `json:"project"`
		Remote            string     `json:"remote"`
		Checkout          string     `json:"checkout"`
		DefaultBranch     string     `json:"default_branch"`
		IntegrationBranch string     `json:"integration_branch"`
		TemplateURL       string     `json:"template_url"`
		TemplateSHA       string     `json:"template_sha"`
		Site              SiteConfig `json:"site"`
		GitLabURL         string     `json:"gitlab_url,omitempty"`
	}
)

type Plan struct {
	SchemaVersion  int       `json:"schema_version"`
	Request        Request   `json:"request"`
	TemplateDigest string    `json:"template_digest"`
	PlanDigest     string    `json:"plan_digest"`
	CreatedAt      time.Time `json:"created_at"`
}

func ValidateRequest(r Request) error {
	if r.Namespace == "" || strings.ContainsAny(r.Namespace, " \t\n") {
		return errors.New("namespace is required")
	}
	if !namePattern.MatchString(r.Project) {
		return errors.New("project must be lowercase")
	}
	u, e := url.Parse(r.Remote)
	if e != nil || u.Host == "" {
		return errors.New("remote must be absolute URL")
	}
	if !filepath.IsAbs(r.Checkout) {
		return errors.New("checkout must be absolute")
	}
	if r.DefaultBranch == "" || r.IntegrationBranch == "" || r.DefaultBranch == r.IntegrationBranch {
		return errors.New("branches must be distinct")
	}
	u, e = url.Parse(r.TemplateURL)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "ssh") {
		return errors.New("template URL must be absolute")
	}
	if !shaPattern.MatchString(strings.ToLower(r.TemplateSHA)) {
		return errors.New("template SHA must be 40 lowercase hex characters")
	}
	if r.Site == nil {
		return errors.New("site configuration is required")
	}
	return nil
}

func TemplateDigest(r Request) string {
	b, _ := json.Marshal(struct{ URL, SHA string }{r.TemplateURL, strings.ToLower(r.TemplateSHA)})
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

func NewPlan(r Request, now time.Time) (Plan, error) {
	if e := ValidateRequest(r); e != nil {
		return Plan{}, e
	}
	r.TemplateSHA = strings.ToLower(r.TemplateSHA)
	p := Plan{SchemaVersion: 1, Request: r, TemplateDigest: TemplateDigest(r), CreatedAt: now.UTC()}
	p.PlanDigest = Digest(p)
	return p, nil
}

func Digest(p Plan) string {
	p.PlanDigest = ""
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

func WritePlan(path string, p Plan) error {
	if p.PlanDigest != Digest(p) {
		return errors.New("plan digest is invalid")
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	if path == "" {
		_, e = os.Stdout.Write(append(b, '\n'))
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func ReadPlan(path string) (Plan, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return Plan{}, e
	}
	var p Plan
	if e = json.Unmarshal(b, &p); e != nil {
		return p, e
	}
	if p.SchemaVersion != 1 || p.PlanDigest == "" || p.PlanDigest != Digest(p) {
		return p, errors.New("plan digest mismatch")
	}
	return p, ValidateRequest(p.Request)
}
