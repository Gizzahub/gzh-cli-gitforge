package cmd

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// issuerRepo creates a repository whose config is isolated from the developer's
// own global git config, so the fallback chain is exercised exactly as written.
func issuerRepo(t *testing.T, keys map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	run := func(args ...string) {
		t.Helper()
		c := exec.CommandContext(t.Context(), "git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	for k, v := range keys {
		run("config", k, v)
	}
	return dir
}

func TestResolveIssuer(t *testing.T) {
	tests := []struct {
		name, flag, want string
		config           map[string]string
	}{
		{
			name:   "flag wins over every configured value",
			flag:   "  release-captain  ",
			config: map[string]string{issuerConfigKey: "from-key", "user.email": "from-email"},
			want:   "release-captain",
		},
		{
			name:   "gzgit.issuer is preferred over user.email",
			config: map[string]string{issuerConfigKey: "from-key", "user.email": "from-email"},
			want:   "from-key",
		},
		{
			name:   "user.email is the fallback",
			config: map[string]string{"user.email": "from-email"},
			want:   "from-email",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := issuerRepo(t, tc.config)
			got, err := resolveIssuer(context.Background(), gitcmd.NewExecutor(), dir, tc.flag)
			if err != nil {
				t.Fatalf("resolveIssuer: %v", err)
			}
			if got != tc.want {
				t.Errorf("issuer = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveIssuerUnsetNamesEverySource(t *testing.T) {
	dir := issuerRepo(t, nil)
	_, err := resolveIssuer(context.Background(), gitcmd.NewExecutor(), dir, "")
	if err == nil {
		t.Fatal("expected an error when no issuer is configured")
	}
	for _, want := range []string{"--issuer", issuerConfigKey, "user.email"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q: %v", want, err)
		}
	}
}

// The issuer is an audit field. An environment variable is the one source a
// process can set for itself in-flight, so no environment name may ever satisfy
// the chain -- this guards that decision against a well-meaning future patch.
func TestResolveIssuerIgnoresEnvironment(t *testing.T) {
	dir := issuerRepo(t, nil)
	for _, name := range []string{"GZGIT_ISSUER", "GZ_GIT_ISSUER", "GIT_WORK_ACTOR", "GIT_AUTHOR_EMAIL", "EMAIL"} {
		t.Setenv(name, "forged-identity")
	}
	if _, err := resolveIssuer(context.Background(), gitcmd.NewExecutor(), dir, ""); err == nil {
		t.Fatal("an environment variable satisfied the issuer chain; it must not")
	}
}
