// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package forgebootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func validatePublicURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("URLs must not contain credentials, query or fragment")
	}
	if u.User != nil {
		_, password := u.User.Password()
		if u.Scheme != "ssh" || u.User.Username() != "git" || password {
			return errors.New("URLs must not contain authentication credentials")
		}
	}
	if (u.Host == "" && u.Scheme != "file") || (u.Scheme == "file" && !filepath.IsAbs(u.Path)) {
		return errors.New("URL must be absolute")
	}
	if u.Scheme != "https" && u.Scheme != "ssh" && u.Scheme != "file" {
		return errors.New("URL scheme must be HTTPS, SSH or file")
	}
	return nil
}

func validateSite(site SiteConfig) error {
	if len(site) == 0 {
		return errors.New("site configuration is required")
	}

	if _, err := json.Marshal(site); err != nil {
		return errors.New("site configuration must contain JSON values")
	}
	return validateSiteValue(site)
}

func seedDigest(root string) (string, error) { return treeDigest(root, true) }
func treeDigest(root string, rejectArtifacts bool) (string, error) {
	if err := validateAncestors(root); err != nil {
		return "", err
	}
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source contains symlink: %s", rel)
		}
		if rel == "." {
			if !d.IsDir() {
				return errors.New("seed must be a directory")
			}
			return nil
		}
		if !rejectArtifacts && (rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator))) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return errors.New("checkout git metadata is not a directory")
		}
		if err := validateSourceEntry(rel); err != nil {
			return err
		}
		if d.IsDir() {
			fmt.Fprintf(h, "dir:%s\x00", filepath.ToSlash(rel))
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("source contains nonregular entry")
		}
		b, err := os.ReadFile(path) // #nosec G304,G122 -- WalkDir under validated seed root; symlinks rejected and digest revalidated before copying
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), info.Mode().Perm()&0o111)
		h.Write(b)
		h.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func validateAncestors(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == string(filepath.Separator) {
		return errors.New("path must be a clean absolute non-root path")
	}
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("path ancestor is a symlink")
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

func validateSiteValue(v any) error {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			lower := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(k))
			for _, bad := range []string{"password", "secret", "token", "credential", "privatekey", "apikey", "authorization", "authheader", "accesskey", "sessionkey"} {
				if strings.Contains(lower, bad) {
					return errors.New("site configuration must not contain secret fields")
				}
			}
			if err := validateSiteValue(val); err != nil {
				return err
			}
		}
	case SiteConfig:
		return validateSiteValue(map[string]any(x))
	case []any:
		for _, val := range x {
			if err := validateSiteValue(val); err != nil {
				return err
			}
		}
	case string:
		if strings.Contains(x, "-----BEGIN") || strings.Contains(x, "glpat-") || strings.Contains(x, "ghp_") {
			return errors.New("site configuration contains secret material")
		}
		if strings.Contains(x, "://") {
			u, err := url.Parse(x)
			if err != nil || u.User != nil || u.RawQuery != "" {
				return errors.New("site configuration contains credential-bearing URL")
			}
		}
	}
	return nil
}

func validateIdentifiers(r Request) error {
	if r.GitLabURL != "" {
		if err := validateAPIOrigin(r.GitLabURL); err != nil {
			return err
		}
	}
	for _, segment := range strings.Split(r.Namespace, "/") {
		if !namePattern.MatchString(segment) {
			return errors.New("namespace must be slash-separated lowercase identifiers")
		}
	}
	for _, value := range []string{r.RepoID, r.WorkerName} {
		if value != "" && !namePattern.MatchString(value) {
			return errors.New("repository ID and worker name must be safe lowercase identifiers")
		}
	}
	if r.SeedDir != "" {
		seed := filepath.Clean(r.SeedDir)
		checkout := filepath.Clean(r.Checkout)
		if seed == checkout || strings.HasPrefix(checkout, seed+string(filepath.Separator)) || strings.HasPrefix(seed, checkout+string(filepath.Separator)) {
			return errors.New("seed and checkout paths must not overlap")
		}
	}
	return nil
}

func validateSourceEntry(rel string) error {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		switch part {
		case ".git", "node_modules", "dist", "test-results", "playwright-report", ".astro", ".env", ".npmrc":
			return fmt.Errorf("seed contains forbidden artifact: %s", rel)
		}
	}
	return nil
}

func validateAPIOrigin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("GitLab API origin must be HTTPS without credentials, query, fragment or path")
	}
	return nil
}

func resolveAPIOrigin(r Request) (string, error) {
	if r.GitLabURL != "" {
		if err := validateAPIOrigin(r.GitLabURL); err != nil {
			return "", err
		}
		return strings.TrimRight(r.GitLabURL, "/"), nil
	}
	remote, err := url.Parse(r.Remote)
	if err != nil {
		return "", errors.New("cannot resolve GitLab API origin")
	}
	if remote.Scheme == "file" {
		return "", nil
	}
	if remote.Hostname() == "" {
		return "", errors.New("remote hostname is required for GitLab API origin")
	}
	host := remote.Host
	if remote.Scheme == "ssh" {
		host = remote.Hostname()
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	} else if remote.Port() != "" {
		host = net.JoinHostPort(remote.Hostname(), remote.Port())
	}
	origin := "https://" + host
	if err := validateAPIOrigin(origin); err != nil {
		return "", err
	}
	return origin, nil
}

// ValidateGitLabRequest refuses local fixture remotes at the authenticated CLI boundary.
func ValidateGitLabRequest(r Request) error {
	if r.GitLabURL == "" {
		return errors.New("GitLab CLI requires a remote HTTPS or SSH origin")
	}
	u, err := url.Parse(r.Remote)
	if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") {
		return errors.New("GitLab CLI requires an HTTPS or SSH remote")
	}
	return validateAPIOrigin(r.GitLabURL)
}

func unconfiguredSeed(r Request) bool { return r.SeedDir != "" && len(r.Site) == 0 }
