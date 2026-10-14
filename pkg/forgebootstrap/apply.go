package forgebootstrap

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// Receipt records bootstrap provenance and progress.
type Receipt struct {
	ProjectID           int64     `json:"project_id,omitempty"`
	MaterializationStep int       `json:"materialization_step,omitempty"`
	SchemaVersion       int       `json:"schema_version"`
	PlanDigest          string    `json:"plan_digest"`
	Phase               string    `json:"phase"`
	InitialSHA          string    `json:"initial_sha,omitempty"`
	TemplateURL         string    `json:"template_url"`
	TemplateSHA         string    `json:"template_sha"`
	ContentDigest       string    `json:"content_digest,omitempty"`
	SiteDigest          string    `json:"site_digest"`
	UpdatedAt           time.Time `json:"updated_at"`
	Error               string    `json:"error,omitempty"`
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
	r, o, err := prepareApply(p, o)
	if err != nil {
		return Receipt{}, err
	}
	rec := Receipt{SchemaVersion: 1, PlanDigest: p.PlanDigest, Phase: "planned", TemplateURL: r.TemplateURL, TemplateSHA: r.TemplateSHA, SiteDigest: siteDigest(r.Site)}
	resumed := false
	if _, err := os.Lstat(o.ReceiptPath); err == nil {
		if err := ReadJSONFile(o.ReceiptPath, &rec); err != nil {
			return rec, err
		}
		if rec.PlanDigest != p.PlanDigest {
			return rec, errors.New("receipt belongs to a different plan")
		}
		resumed = true
	} else if !os.IsNotExist(err) {
		return rec, err
	}
	if err := validateAncestors(r.Checkout); err != nil {
		return rec, err
	}
	if !resumed || rec.Phase == "planned" || rec.Phase == "remote_ready" {
		if err := validateCheckoutPath(r.Checkout); err != nil {
			return rec, err
		}
	}
	if err := validateRemoteState(ctx, p, o, rec, resumed); err != nil {
		return rec, err
	}
	if rec.Phase == "planned" {
		var err error
		rec, err = ensureDestination(ctx, p, o, rec)
		if err != nil {
			return rec, err
		}
	}
	if rec.Phase == "remote_ready" || rec.Phase == "copying" {
		var err error
		rec, err = copySource(ctx, p, o, rec)
		if err != nil {
			return rec, err
		}
	}

	if rec.Phase == "materialized" {
		var err error
		rec, err = materializeSite(p, o, rec)
		if err != nil {
			return rec, err
		}
	}

	if rec.Phase == "copied" {
		var err error
		rec, err = commitSite(ctx, p, o, rec)
		if err != nil {
			return rec, err
		}
	}

	return finishApply(ctx, p, o, rec)
}

func verifyRefs(raw string, r Request, sha string) error {
	return verifyBootstrapRefs(raw, r, sha, false)
}

// Already published GitLab pipelines may retain the exact initial commit.
func verifyPublishedRefs(raw string, r Request, sha string) error {
	return verifyBootstrapRefs(raw, r, sha, true)
}

func verifyBootstrapRefs(raw string, r Request, sha string, allowPipelines bool) error {
	required := map[string]bool{"refs/heads/" + r.DefaultBranch: false, "refs/heads/" + r.IntegrationBranch: false}
	seen := map[string]bool{}
	pipeline := regexp.MustCompile(`^refs/pipelines/[1-9]\d*$`)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return errors.New("remote ref response is malformed")
		}
		name := parts[1]
		if seen[name] {
			return errors.New("remote ref response contains duplicates")
		}
		seen[name] = true
		if parts[0] != sha {
			return errors.New("remote ref differs from the initial commit")
		}
		if _, ok := required[name]; ok {
			required[name] = true
			continue
		}
		if !allowPipelines || !pipeline.MatchString(name) {
			return errors.New("remote contains an unexpected ref")
		}
	}
	for _, present := range required {
		if !present {
			return errors.New("remote branches do not match the initial commit")
		}
	}
	return nil
}

func siteDigest(site SiteConfig) string {
	b, _ := json.Marshal(site) //nolint:errcheck // SiteConfig is validated JSON before hashing.
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

func personalizeFile(root, file, old, value string) error {
	if value == "" || value == "repository-id: " {
		return nil
	}
	if !namePattern.MatchString(strings.TrimPrefix(value, "repository-id: ")) {
		return errors.New("repository ID and worker name must be safe lowercase identifiers")
	}
	scoped, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer scoped.Close()
	b, err := scoped.ReadFile(file)
	if err != nil {
		return err
	}
	var updated []byte
	if file == "wrangler.jsonc" {
		updated, err = personalizeWorkerConfig(b, old, value)
	} else {
		updated, err = personalizeRepositoryConfig(b, old, value)
	}
	if err != nil {
		return err
	}
	return atomicRootFile(scoped, file, updated)
}

func validateCheckoutPath(path string) error {
	if err := validateAncestors(path); err != nil {
		return err
	}
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
		return errors.New("git execution failed")
	}
	if r.Error != nil {
		return errors.New("git command failed; inspect retained resources")
	}
	if r.ExitCode != 0 {
		return fmt.Errorf("git exited %d", r.ExitCode)
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
		return "", fmt.Errorf("git exited %d", r.ExitCode)
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
		return copyFile(path, to, d)
	})
}

func writeSiteConfig(root string, s SiteConfig) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return writeCheckoutFile(root, "site.config.json", append(b, '\n'))
}

func writeReceipt(path string, r Receipt) error {
	if path == "" {
		return nil
	}
	if err := validateAncestors(path); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("receipt path is not a regular file")
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

func validatePartialTree(source, destination string) error {
	return filepath.WalkDir(destination, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(destination, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("partial checkout contains symlink")
		}
		expected, err := os.Lstat(filepath.Join(source, rel))
		if err != nil {
			return errors.New("partial checkout contains unexpected entry")
		}
		if entry.IsDir() != expected.IsDir() || rel == ".git" {
			return errors.New("partial checkout contains unexpected entry")
		}
		return nil
	})
}

func copySource(ctx context.Context, p Plan, o ApplyOptions, rec Receipt) (Receipt, error) {
	r := p.Request
	fail := func(err error) (Receipt, error) { return failReceipt(o, rec, err) }
	g := o.Git
	if g == nil {
		g = execGit
	}
	out := o.GitOutput
	if out == nil {
		out = execGitOutput
	}
	source := r.SeedDir
	if source != "" {
		digest, err := seedDigest(source)
		if err != nil {
			return fail(err)
		}
		if digest != p.TemplateDigest {
			return fail(errors.New("seed changed after planning"))
		}
	} else {
		tmp, err := os.MkdirTemp(filepath.Dir(r.Checkout), ".bootstrap-template-")
		if err != nil {
			return fail(err)
		}
		defer os.RemoveAll(tmp)
		if err = g(ctx, "", "clone", "--no-checkout", "--", r.TemplateURL, tmp); err != nil {
			return fail(err)
		}
		actual, err := out(ctx, tmp, "rev-parse", r.TemplateSHA+"^{commit}")
		if err != nil || strings.TrimSpace(actual) != r.TemplateSHA {
			return fail(errors.New("template SHA is not the requested commit"))
		}
		if err = g(ctx, tmp, "-c", "core.hooksPath=/dev/null", "checkout", "--detach", r.TemplateSHA); err != nil {
			return fail(err)
		}
		source = tmp
	}
	rec.Phase = "copying"
	if err := writeReceipt(o.ReceiptPath, rec); err != nil {
		return rec, err
	}
	if err := os.MkdirAll(r.Checkout, 0o700); err != nil {
		return fail(err)
	}
	if err := validatePartialTree(source, r.Checkout); err != nil {
		return fail(err)
	}
	if err := copyTree(source, r.Checkout); err != nil {
		return fail(err)
	}
	rawDigest, err := treeDigest(r.Checkout, false)
	if err != nil {
		return fail(err)
	}
	rec.ContentDigest = rawDigest
	rec.Phase = "materialized"
	if err := writeReceipt(o.ReceiptPath, rec); err != nil {
		return rec, err
	}

	return rec, nil
}

func materializeSite(p Plan, o ApplyOptions, rec Receipt) (Receipt, error) {
	r := p.Request
	fail := func(err error) (Receipt, error) { return failReceipt(o, rec, err) }
	current, err := treeDigest(r.Checkout, false)
	if err != nil {
		return fail(err)
	}
	if rec.ContentDigest == "" || current != rec.ContentDigest {
		return fail(errors.New("materialized checkout changed before configuration write"))
	}
	steps := []func() error{
		func() error {
			if unconfiguredSeed(r) {
				return nil
			}
			return writeSiteConfig(r.Checkout, r.Site)
		},
		func() error {
			return personalizeFile(r.Checkout, ".ce/task-runtime.yaml", "repository-id: singlepageweb-template", "repository-id: "+r.RepoID)
		},
		func() error {
			if unconfiguredSeed(r) {
				return nil
			}
			return personalizeFile(r.Checkout, "wrangler.jsonc", "replace-with-worker-name", r.WorkerName)
		},
		func() error { return writeProvenance(p, rec) },
	}
	if rec.MaterializationStep < 0 || rec.MaterializationStep > len(steps) {
		return fail(errors.New("invalid materialization receipt step"))
	}
	for rec.MaterializationStep < len(steps) {
		current, err := treeDigest(r.Checkout, false)
		if err != nil {
			return fail(err)
		}
		if current != rec.ContentDigest {
			return fail(errors.New("materialized checkout changed between configuration steps"))
		}
		if err := steps[rec.MaterializationStep](); err != nil {
			return fail(err)
		}
		digest, err := treeDigest(r.Checkout, false)
		if err != nil {
			return fail(err)
		}
		rec.ContentDigest = digest
		rec.MaterializationStep++
		if err := writeReceipt(o.ReceiptPath, rec); err != nil {
			return rec, err
		}
	}
	rec.Phase = "copied"
	if err := writeReceipt(o.ReceiptPath, rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func writeProvenance(p Plan, rec Receipt) error {
	marker := struct {
		PlanDigest     string `json:"plan_digest"`
		TemplateURL    string `json:"template_url"`
		TemplateSHA    string `json:"template_sha"`
		TemplateDigest string `json:"template_digest"`
		SiteDigest     string `json:"site_digest"`
	}{p.PlanDigest, p.Request.TemplateURL, p.Request.TemplateSHA, p.TemplateDigest, rec.SiteDigest}
	b, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return writeCheckoutFile(p.Request.Checkout, ".bootstrap-provenance.json", append(b, '\n'))
}

func writeCheckoutFile(root, path string, contents []byte) error {
	scoped, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer scoped.Close()
	return atomicRootFile(scoped, path, contents)
}

func atomicRootFile(scoped *os.Root, path string, contents []byte) error {
	if info, err := scoped.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("configuration target must be a regular non-symlink file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(path), ".bootstrap-write-"+hex.EncodeToString(nonce[:]))
	file, err := scoped.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	count, writeErr := file.Write(contents)
	if writeErr == nil && count != len(contents) {
		writeErr = io.ErrShortWrite
	}
	closeErr := file.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		removeErr := scoped.Remove(temp)
		return errors.Join(err, removeErr)
	}
	if err = scoped.Rename(temp, path); err != nil {
		return errors.Join(err, scoped.Remove(temp))
	}
	return nil
}

func commitSite(ctx context.Context, p Plan, o ApplyOptions, rec Receipt) (Receipt, error) {
	r := p.Request
	fail := func(err error) (Receipt, error) { return failReceipt(o, rec, err) }
	g := o.Git
	if g == nil {
		g = execGit
	}
	out := o.GitOutput
	if out == nil {
		out = execGitOutput
	}
	contentDigest, err := treeDigest(r.Checkout, false)
	if err != nil {
		return fail(err)
	}
	if rec.ContentDigest == "" || contentDigest != rec.ContentDigest {
		return fail(errors.New("prepared checkout contents differ from receipt"))
	}
	if err := g(ctx, r.Checkout, "init", "--initial-branch", r.DefaultBranch); err != nil {
		return fail(err)
	}
	origin, originErr := out(ctx, r.Checkout, "remote", "get-url", "origin")
	if originErr != nil {
		if err := g(ctx, r.Checkout, "remote", "add", "origin", r.Remote); err != nil {
			return fail(err)
		}
	} else if strings.TrimSpace(origin) != r.Remote {
		return fail(errors.New("checkout origin differs from plan"))
	}
	head, headErr := out(ctx, r.Checkout, "rev-parse", "HEAD")
	if headErr == nil {
		count, err := out(ctx, r.Checkout, "rev-list", "--count", "HEAD")
		if err != nil || strings.TrimSpace(count) != "1" || !shaPattern.MatchString(strings.TrimSpace(head)) {
			return fail(errors.New("checkout history is not an initial bootstrap commit"))
		}
	} else {
		if err := g(ctx, r.Checkout, "add", "--force", "--all"); err != nil {
			return fail(err)
		}

		if err := g(ctx, r.Checkout, "-c", "core.hooksPath=/dev/null", "commit", "-m", "chore(bootstrap): initialize from template", "-m", "Model: "+o.CommitModel); err != nil {
			return fail(err)
		}
	}
	sha, err := out(ctx, r.Checkout, "rev-parse", "HEAD")
	if err != nil {
		return fail(err)
	}
	rec.InitialSHA = strings.TrimSpace(sha)
	rec.Phase = "committed"
	if err := writeReceipt(o.ReceiptPath, rec); err != nil {
		return rec, err
	}

	return rec, nil
}

func failReceipt(o ApplyOptions, rec Receipt, err error) (Receipt, error) {
	rec.Error = "bootstrap failed; inspect retained resources before resuming"
	rec.UpdatedAt = time.Now().UTC()
	if e := writeReceipt(o.ReceiptPath, rec); e != nil {
		return rec, errors.Join(err, fmt.Errorf("receipt: %w", e))
	}
	return rec, err
}

func finishApply(ctx context.Context, p Plan, o ApplyOptions, rec Receipt) (Receipt, error) {
	r := p.Request
	g := o.Git
	if g == nil {
		g = execGit
	}
	out := o.GitOutput
	if out == nil {
		out = execGitOutput
	}
	fail := func(err error) (Receipt, error) { return failReceipt(o, rec, err) }
	if rec.Phase != "committed" && rec.Phase != "pushed" {
		return fail(errors.New("unknown receipt phase"))
	}
	origin, err := out(ctx, r.Checkout, "remote", "get-url", "origin")
	if err != nil || strings.TrimSpace(origin) != r.Remote {
		return fail(errors.New("checkout origin differs from plan"))
	}
	sha, err := out(ctx, r.Checkout, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(sha) != rec.InitialSHA {
		return fail(errors.New("local initial commit differs from receipt"))
	}
	marker, err := os.ReadFile(filepath.Join(r.Checkout, ".bootstrap-provenance.json"))
	if err != nil || !strings.Contains(string(marker), p.PlanDigest) {
		return fail(errors.New("checkout provenance differs from plan"))
	}
	status, err := out(ctx, r.Checkout, "status", "--porcelain")
	if err != nil || strings.TrimSpace(status) != "" {
		return fail(errors.New("bootstrap checkout has changed"))
	}
	if rec.Phase == "committed" {
		if err = g(ctx, r.Checkout, "branch", r.IntegrationBranch, rec.InitialSHA); err != nil {
			existing, e := out(ctx, r.Checkout, "rev-parse", "refs/heads/"+r.IntegrationBranch)
			if e != nil || strings.TrimSpace(existing) != rec.InitialSHA {
				return fail(err)
			}
		}
		remoteRefs, e := out(ctx, r.Checkout, "ls-remote", "--refs", "origin")
		if e != nil {
			return fail(e)
		}
		if strings.TrimSpace(remoteRefs) != "" {
			if err := verifyPublishedRefs(remoteRefs, r, rec.InitialSHA); err != nil {
				return fail(err)
			}
		} else if err = g(ctx, r.Checkout, "-c", "core.hooksPath=/dev/null", "push", "--atomic", "-u", "origin", r.DefaultBranch, r.IntegrationBranch); err != nil {
			return fail(err)
		}
	}
	refs, err := out(ctx, r.Checkout, "ls-remote", "--refs", "origin")
	if err != nil {
		return fail(err)
	}
	if err = verifyPublishedRefs(refs, r, rec.InitialSHA); err != nil {
		return fail(err)
	}
	rec.Phase = "pushed"
	rec.Error = ""
	rec.UpdatedAt = time.Now().UTC()
	if err := writeReceipt(o.ReceiptPath, rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func copyFile(path, to string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("template contains nonregular entry")
	}
	b, e := os.ReadFile(path) // #nosec G304,G122 -- path is produced by WalkDir under the validated template root
	if e != nil {
		return e
	}
	mode := info.Mode().Perm()
	if mode&0o100 != 0 {
		mode = 0o600 | (mode & 0o111)
	} else {
		mode = 0o600
	}
	if existing, err := os.Lstat(to); err == nil {
		if !existing.Mode().IsRegular() {
			return errors.New("copy destination is not regular")
		}
		previous, err := os.ReadFile(to) // #nosec G304 -- destination derived from validated template relative path
		if err != nil {
			return err
		}
		if !bytes.Equal(previous, b) {
			return errors.New("partial checkout file changed")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(to, b, mode) // #nosec G703 -- destination is filepath.Join of validated relative path
}

func prepareApply(p Plan, o ApplyOptions) (Request, ApplyOptions, error) {
	if p.PlanDigest != Digest(p) {
		return Request{}, o, errors.New("plan digest mismatch")
	}
	if err := ValidateRequest(p.Request); err != nil {
		return Request{}, o, err
	}
	r := p.Request
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*\d[A-Za-z0-9._/-]*$`).MatchString(o.CommitModel) {
		return Request{}, o, errors.New("explicit versioned commit model is required")
	}
	if r.SeedDir != "" {
		digest, err := seedDigest(r.SeedDir)
		if err != nil {
			return Request{}, o, err
		}
		if digest != p.TemplateDigest {
			return Request{}, o, errors.New("seed changed after planning")
		}
	}
	if o.Remote == nil {
		return Request{}, o, errors.New("remote client is required")
	}
	if o.ReceiptPath == "" {
		o.ReceiptPath = r.Checkout + ".bootstrap-receipt.json"
	}
	if o.ReceiptPath == r.Checkout || strings.HasPrefix(o.ReceiptPath, r.Checkout+string(filepath.Separator)) {
		return Request{}, o, errors.New("receipt must be outside the bootstrap checkout")
	}
	if err := validateAncestors(o.ReceiptPath); err != nil {
		return Request{}, o, err
	}
	return r, o, nil
}

func validateRemoteState(ctx context.Context, p Plan, o ApplyOptions, rec Receipt, resumed bool) error {
	r := p.Request
	if inspector, ok := o.Remote.(Inspector); ok {
		state, err := inspector.Inspect(ctx, r)
		if err != nil {
			return err
		}
		if state.Exists && !state.Empty && rec.Phase != "committed" && rec.Phase != "pushed" {
			return errors.New("destination contains content before bootstrap commit")
		}
		if resumed && rec.ProjectID != 0 && (!state.Exists || state.ProjectID != rec.ProjectID) {
			return errors.New("receipt destination project identity changed")
		}
		if state.Exists && state.ProjectID != p.RemoteState.ProjectID && (p.RemoteState.Exists || !resumed) {
			return errors.New("destination project identity changed after planning")
		}
		if !resumed && (state.Exists != p.RemoteState.Exists || !state.Empty && state.Exists) {
			return errors.New("destination state changed after planning")
		}
	}
	return nil
}

func ensureDestination(ctx context.Context, p Plan, o ApplyOptions, rec Receipt) (Receipt, error) {
	if rec.Phase != "planned" {
		return rec, nil
	}
	rec.Phase = "provisioning"
	if err := writeReceipt(o.ReceiptPath, rec); err != nil {
		return rec, err
	}
	id, err := provisionDestination(ctx, p, o)
	if err != nil {
		return failReceipt(o, rec, err)
	}
	rec.ProjectID = id
	rec.Phase = "remote_ready"
	if err := writeReceipt(o.ReceiptPath, rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func provisionDestination(ctx context.Context, p Plan, o ApplyOptions) (int64, error) {
	var id int64
	if provisioner, ok := o.Remote.(Provisioner); ok {
		state, err := provisioner.EnsureProject(ctx, p.Request, p.RemoteState)
		if err != nil {
			return 0, err
		}
		if !state.Exists || !state.Empty || state.ProjectID <= 0 || (p.RemoteState.Exists && state.ProjectID != p.RemoteState.ProjectID) {
			return 0, errors.New("provisioning returned incompatible project identity")
		}
		id = state.ProjectID
	} else if err := o.Remote.Ensure(ctx, p.Request); err != nil {
		return 0, err
	}
	return observeProvisionedIdentity(ctx, p, o, id)
}

func observeProvisionedIdentity(ctx context.Context, p Plan, o ApplyOptions, id int64) (int64, error) {
	inspector, ok := o.Remote.(Inspector)
	if !ok {
		return id, nil
	}
	state, err := inspector.Inspect(ctx, p.Request)
	if err != nil {
		return 0, err
	}
	if !state.Exists || !state.Empty {
		return 0, errors.New("created remote state is incompatible")
	}
	if id != 0 && state.ProjectID != id {
		return 0, errors.New("destination project identity changed after provisioning")
	}
	if p.RemoteState.Exists && state.ProjectID != p.RemoteState.ProjectID {
		return 0, errors.New("destination project identity changed during provisioning")
	}
	return state.ProjectID, nil
}

func personalizeRepositoryConfig(contents []byte, old, value string) ([]byte, error) {
	lines := strings.Split(string(contents), "\n")
	found := false
	for i, line := range lines {
		property := strings.TrimSpace(line)
		if !strings.HasPrefix(property, "repository-id:") {
			continue
		}
		if found {
			return nil, errors.New("template has duplicate repository identity")
		}
		found = true
		if property == old {
			lines[i] = value
		} else if property != value {
			return nil, errors.New("template repository identity is incompatible")
		}
	}
	if !found {
		return nil, errors.New("template repository identity missing")
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func personalizeWorkerConfig(contents []byte, old, value string) ([]byte, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(contents, &config); err != nil {
		return nil, errors.New("template worker configuration must be a JSON object")
	}
	var name string
	if err := json.Unmarshal(config["name"], &name); err != nil {
		return nil, errors.New("template worker name is missing or invalid")
	}
	if name == value {
		return contents, nil
	}
	if name != old {
		return nil, errors.New("template worker placeholder is incompatible")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	config["name"] = encoded
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
