// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package forgebootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// SiteConfig contains site-specific settings.
	SiteConfig map[string]any
	// Request describes a bootstrap operation.
	Request struct {
		Namespace         string     `json:"namespace"`
		Project           string     `json:"project"`
		Remote            string     `json:"remote"`
		Checkout          string     `json:"checkout"`
		DefaultBranch     string     `json:"default_branch"`
		IntegrationBranch string     `json:"integration_branch"`
		TemplateURL       string     `json:"template_url"`
		TemplateSHA       string     `json:"template_sha"`
		Site              SiteConfig `json:"site"`
		SeedDir           string     `json:"seed_dir,omitempty"`
		RepoID            string     `json:"repo_id,omitempty"`
		WorkerName        string     `json:"worker_name,omitempty"`
		GitLabURL         string     `json:"git_lab_url,omitempty"`
	}
)

// Plan is a digest-bound bootstrap request and observed destination state.
type Plan struct {
	RemoteState    RemoteState `json:"remote_state"`
	SchemaVersion  int         `json:"schema_version"`
	Request        Request     `json:"request"`
	TemplateDigest string      `json:"template_digest"`
	PlanDigest     string      `json:"plan_digest"`
	CreatedAt      time.Time   `json:"created_at"`
}

// ValidateRequest rejects unsafe or unsupported bootstrap inputs.
func ValidateRequest(r Request) error {
	if r.Namespace == "" || strings.ContainsAny(r.Namespace, " \t\n") {
		return errors.New("namespace is required")
	}
	if !namePattern.MatchString(r.Project) {
		return errors.New("project must be lowercase")
	}
	u, e := url.Parse(r.Remote)
	if e != nil || (u.Host == "" && u.Scheme != "file") {
		return errors.New("remote must be absolute URL")
	}
	if !filepath.IsAbs(r.Checkout) {
		return errors.New("checkout must be absolute")
	}
	if r.DefaultBranch != "master" || r.IntegrationBranch != "develop" {
		return errors.New("bootstrap requires master default and develop integration branches")
	}
	if err := validatePublicURL(r.Remote); err != nil {
		return err
	}
	if err := validateIdentifiers(r); err != nil {
		return err
	}
	if !unconfiguredSeed(r) {
		if err := validateSite(r.Site); err != nil {
			return err
		}
	}
	if r.SeedDir != "" {
		if r.TemplateURL != "" || r.TemplateSHA != "" || !filepath.IsAbs(r.SeedDir) {
			return errors.New("seed directory requires an absolute path and no template URL/SHA")
		}
		return nil
	}

	if err := validatePublicURL(r.TemplateURL); err != nil {
		return err
	}
	if !shaPattern.MatchString(strings.ToLower(r.TemplateSHA)) {
		return errors.New("template SHA must be 40 lowercase hex characters")
	}
	return nil
}

// TemplateDigest binds the template URL and exact commit identity.
func TemplateDigest(r Request) string {
	b, _ := json.Marshal(struct{ URL, SHA string }{r.TemplateURL, strings.ToLower(r.TemplateSHA)}) //nolint:errcheck // fixed-shape values cannot fail
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// NewPlan validates and binds the requested source and site configuration.
func NewPlan(r Request, now time.Time) (Plan, error) {
	origin, err := resolveAPIOrigin(r)
	if err != nil {
		return Plan{}, err
	}
	r.GitLabURL = origin
	if e := ValidateRequest(r); e != nil {
		return Plan{}, e
	}
	if r.RepoID == "" {
		r.RepoID = r.Project
	}
	if r.WorkerName == "" && !unconfiguredSeed(r) {
		r.WorkerName = r.Project
	}
	r.TemplateSHA = strings.ToLower(r.TemplateSHA)

	p := Plan{SchemaVersion: 1, Request: r, TemplateDigest: TemplateDigest(r), CreatedAt: now.UTC()}
	if r.SeedDir != "" {
		if unconfiguredSeed(r) {
			if _, err := os.Lstat(filepath.Join(r.SeedDir, "site.config.json")); err == nil {
				return Plan{}, errors.New("unconfigured template seed must not contain root site.config.json")
			} else if !os.IsNotExist(err) {
				return Plan{}, err
			}
		}
		var err error
		p.TemplateDigest, err = seedDigest(r.SeedDir)
		if err != nil {
			return Plan{}, err
		}
	}
	p.PlanDigest = Digest(p)
	return p, nil
}

// Digest computes the deterministic plan confirmation digest.
func Digest(p Plan) string {
	p.PlanDigest = ""
	b, _ := json.Marshal(p) //nolint:errcheck // Plan contains only JSON values
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// WritePlan persists a validated plan without authentication material.
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
	if err := validateAncestors(path); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("plan output is not a regular file")
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// ReadJSONFile reads a bounded regular JSON input file.
func ReadJSONFile(path string, dst any) error {
	st, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("input must be a regular non-symlink file")
	}
	f, e := os.Open(path) // #nosec G304 -- regular non-symlink operator file checked above
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return errors.New("input exceeds 1 MiB")
	}
	if e := json.Unmarshal(b, dst); e != nil {
		return fmt.Errorf("decode JSON: %w", e)
	}
	return nil
}

// ReadPlan loads and validates a persisted bootstrap plan.
func ReadPlan(path string) (Plan, error) {
	var p Plan
	if e := ReadJSONFile(path, &p); e != nil {
		return p, e
	}

	if p.SchemaVersion != 1 || p.PlanDigest == "" || p.PlanDigest != Digest(p) {
		return p, errors.New("plan digest mismatch")
	}
	origin, err := resolveAPIOrigin(p.Request)
	if err != nil {
		return p, err
	}
	if origin != p.Request.GitLabURL {
		return p, errors.New("plan GitLab API origin is not normalized")
	}
	return p, ValidateRequest(p.Request)
}

// RemoteState binds a plan to the destination project identity.
type RemoteState struct {
	ProjectID int64 `json:"project_id"`
	Exists    bool  `json:"exists"`
	Empty     bool  `json:"empty"`
}

// Inspector reads destination identity without creating it.
type Inspector interface {
	Inspect(context.Context, Request) (RemoteState, error)
}

// Provisioner binds creation or empty reuse to the expected destination identity.
type Provisioner interface {
	EnsureProject(context.Context, Request, RemoteState) (RemoteState, error)
}
