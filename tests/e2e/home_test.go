package e2e

import (
	"os"
	"path/filepath"
)

// isolateHome points HOME at an empty directory under dir for the whole test
// binary and every gz-git and git process it starts. gz-git fails on a global
// config it cannot load, so the operator's own config must not decide these
// tests. The isolated home gets a global git identity of its own, at the
// lowest precedence, so repository-level identities set by tests still win.
func isolateHome(dir string) error {
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	gitconfig := "[user]\n\tname = Test User\n\temail = test@example.com\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o600); err != nil {
		return err
	}
	env := map[string]string{
		"HOME":                home,
		"XDG_CONFIG_HOME":     filepath.Join(home, ".config"),
		"GIT_CONFIG_NOSYSTEM": "1",
		// An inherited GIT_CONFIG_GLOBAL would replace the isolated .gitconfig.
		"GIT_CONFIG_GLOBAL": filepath.Join(home, ".gitconfig"),
	}
	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}
