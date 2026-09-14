package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// issuerConfigKey is the gz-git-owned git config key holding the identity
// recorded in an integrate confirmation plan. gz-git only ever reads it, so
// external identity tooling can populate it without gz-git depending on any
// particular identity framework or config format.
const issuerConfigKey = "gzgit.issuer"

// resolveIssuer determines the identity recorded in a confirmation plan,
// preferring an explicit flag, then the gz-git config key, then the git
// identity already configured for commits.
//
// The environment is deliberately never consulted. The issuer is an audit
// field, and an environment variable is the one source a process can set for
// itself, in-flight, without leaving a reviewable trace on disk.
func resolveIssuer(ctx context.Context, exec *gitcmd.Executor, dir, flag string) (string, error) {
	if v := strings.TrimSpace(flag); v != "" {
		return v, nil
	}
	for _, key := range []string{issuerConfigKey, "user.email"} {
		v, err := gitConfigValue(ctx, exec, dir, key)
		if err != nil {
			return "", err
		}
		if v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf(
		"issuer is required: pass --issuer <identity>, or record one with\n"+
			"  git config --global %s <identity>\n"+
			"(user.email is used when neither is set; the environment is not consulted)",
		issuerConfigKey,
	)
}

// gitConfigValue reads one git config key, reporting an unset key as an empty
// value rather than an error. "git config --get" exits non-zero when the key is
// absent, which is an answer here, not a failure.
func gitConfigValue(ctx context.Context, exec *gitcmd.Executor, dir, key string) (string, error) {
	res, err := exec.Run(ctx, dir, "config", "--get", key)
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", nil
	}
	return strings.TrimSpace(res.Stdout), nil
}
