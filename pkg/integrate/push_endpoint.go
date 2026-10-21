// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func boundPushEndpoint(ctx context.Context, g gitRepo, remote, explicit string) (string, error) {
	endpoint := explicit
	if endpoint == "" {
		var err error
		endpoint, err = configuredPushEndpoint(ctx, g, remote)
		if err != nil {
			return "", err
		}
	}
	return validateBoundPushEndpoint(ctx, g, endpoint)
}

func pushEndpoint(ctx context.Context, g gitRepo, remote string) (string, error) {
	endpoint, err := configuredPushEndpoint(ctx, g, remote)
	if err != nil {
		return "", err
	}
	if gitcmd.IsSCPRemoteEndpoint(endpoint) {
		return validateBoundPushEndpoint(ctx, g, endpoint)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		if filepath.IsAbs(endpoint) {
			return validateBoundPushEndpoint(ctx, g, endpoint)
		}
		anchor, err := filepath.Abs(g.dir)
		if err != nil {
			return "", err
		}
		anchor, err = filepath.EvalSymlinks(anchor)
		if err != nil {
			return "", err
		}
		// Preserve symlink/.. ordering until physical resolution; Join would
		// lexically clean it and select a different repository.
		endpoint = anchor + string(os.PathSeparator) + endpoint //nolint:gocritic // filepath.Join changes Git symlink/.. semantics and the tested destination.
		physical, err := filepath.EvalSymlinks(endpoint)
		if err != nil {
			return "", err
		}
		return validateBoundPushEndpoint(ctx, g, physical)
	}
	return validateBoundPushEndpoint(ctx, g, endpoint)
}

func validateBoundPushEndpoint(ctx context.Context, g gitRepo, raw string) (string, error) {
	endpoint, err := immutablePushEndpoint(raw)
	if err != nil {
		return "", err
	}
	if err := refusePushEndpointRewrites(ctx, g, endpoint); err != nil {
		return "", err
	}
	return endpoint, nil
}

func immutablePushEndpoint(raw string) (string, error) {
	endpoint, err := canonicalPushEndpoint(raw)
	if err != nil {
		return "", err
	}
	if gitcmd.IsSCPRemoteEndpoint(endpoint) {
		return endpoint, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if (u.Scheme == "" && !filepath.IsAbs(endpoint)) || (u.Scheme == "file" && (u.Opaque != "" || !filepath.IsAbs(u.Path))) {
		return "", fmt.Errorf("relative push endpoint has no immutable execution-directory binding: %s", endpoint)
	}
	return endpoint, nil
}

func configuredPushEndpoint(ctx context.Context, g gitRepo, remote string) (string, error) {
	res, err := g.run(ctx, "remote", "get-url", "--push", "--all", remote)
	if err != nil {
		return "", err
	}
	vals := splitNonEmpty(res.Stdout)
	if len(vals) != 1 {
		return "", fmt.Errorf("remote %s has ambiguous push endpoint", remote)
	}
	endpoint, err := canonicalPushEndpoint(strings.TrimSpace(vals[0]))
	if err != nil {
		return "", err
	}
	if err := refusePushEndpointRewrites(ctx, g, endpoint); err != nil {
		return "", err
	}
	return endpoint, nil
}

func refusePushEndpointRewrites(ctx context.Context, g gitRepo, endpoint string) error {
	rewrites, err := g.run(ctx, "config", "--get-regexp", `^url\.`)
	if err != nil {
		return err
	}
	if rewrites.ExitCode != 0 && rewrites.ExitCode != 1 {
		return fmt.Errorf("cannot query URL rewrite configuration: exit %d", rewrites.ExitCode)
	}
	if rewrites.ExitCode == 1 && strings.TrimSpace(rewrites.Stdout) != "" {
		return fmt.Errorf("URL rewrite configuration query is incomplete")
	}
	for _, line := range strings.Split(rewrites.Stdout, "\n") {
		key, prefix, hasValue := strings.Cut(line, " ")
		key = strings.ToLower(key)
		isRewrite := strings.HasSuffix(key, ".insteadof") || strings.HasSuffix(key, ".pushinsteadof")
		if isRewrite && (!hasValue || strings.HasPrefix(endpoint, prefix)) {
			return fmt.Errorf("effective push endpoint is subject to another URL rewrite")
		}
	}
	return nil
}

func canonicalPushEndpoint(raw string) (string, error) {
	return gitcmd.CanonicalRemoteEndpoint(raw)
}

func exactRemoteSHA(ctx context.Context, g gitRepo, endpoint, destination string) (string, error) {
	res, err := g.run(ctx, "ls-remote", endpoint, destination)
	if err != nil || res.ExitCode != 0 {
		return "", fmt.Errorf("resolve target from push endpoint: %w", err)
	}
	fields := strings.Fields(res.Stdout)
	if len(fields) < 2 || fields[1] != destination {
		return "", fmt.Errorf("target is missing at push endpoint")
	}
	return fields[0], nil
}
