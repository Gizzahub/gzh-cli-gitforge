// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// hostNamePattern is the device catalog grammar for declared host names
// (ce-devenv config/agents/work-identities.yaml). The catalog itself stays
// outside gz-git; the grammar is the shared contract, so every host field
// accepts exactly what the catalog can name and nothing a reader could split.
var hostNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// maxHostNameLen matches the opaque-token cap so no host field grows past
// what other execution-context fields allow.
const maxHostNameLen = 128

// isHostName reports whether value is a device name within the length cap.
func isHostName(value string) bool {
	return len(value) <= maxHostNameLen && hostNamePattern.MatchString(value)
}

// validateHostName checks a declared device name against the catalog
// grammar. Only host fields use it; execution ids, generations, and actors
// stay opaque tokens.
func validateHostName(value, name string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is empty", ErrInvalidExecutionContext, name)
	}
	if len(value) > maxHostNameLen {
		return fmt.Errorf("%w: %s is longer than %d characters", ErrInvalidExecutionContext, name, maxHostNameLen)
	}
	if !hostNamePattern.MatchString(value) {
		return fmt.Errorf("%w: %s %q is not a device name (lowercase letters, digits, single hyphens)", ErrInvalidExecutionContext, name, value)
	}
	return nil
}

// Device labels for a run whose device is not a usable declared name.
const (
	DeviceUndeclared = "undeclared"
	DeviceInvalid    = "invalid"
)

// DeviceIdentity is the declared device of the running process, resolved for
// display: provenance and lock notices. It never authorizes anything, so an
// absent or malformed declaration is reported instead of failing the run.
type DeviceIdentity struct {
	// Host is the declared device name, empty unless it passed the grammar.
	Host string
	// Rejected marks a declared but malformed name, never written as Host.
	Rejected bool
}

// Label is the host name, or DeviceInvalid / DeviceUndeclared.
func (d DeviceIdentity) Label() string {
	switch {
	case d.Host != "":
		return d.Host
	case d.Rejected:
		return DeviceInvalid
	default:
		return DeviceUndeclared
	}
}

// Declared reports whether the device has a valid declared name.
func (d DeviceIdentity) Declared() bool {
	return d.Host != ""
}

// resolveDeclaredDevice follows the caller-owner device chain (GIT_WORK_HOST,
// then global Git ce.workHost) with no hostname fallback. Within pkg/integrate
// it is the single resolution behind both the run provenance line and the
// host-lock label, so those two cannot disagree. Other packages (for example
// internal/runtask, which normalizes case) may still resolve differently.
func resolveDeclaredDevice(ctx context.Context, g gitRepo) DeviceIdentity {
	host := strings.TrimSpace(os.Getenv(callerHostEnv))
	if host == "" {
		configured, err := g.output(ctx, "config", "--global", "--includes", "--get", callerHostGitKey)
		if err != nil {
			return DeviceIdentity{}
		}
		host = strings.TrimSpace(configured)
	}
	if host == "" {
		return DeviceIdentity{}
	}
	if !isHostName(host) {
		return DeviceIdentity{Rejected: true}
	}
	return DeviceIdentity{Host: host}
}

// ResolveDevice resolves the declared device for a command run in dir.
func ResolveDevice(ctx context.Context, exec *gitcmd.Executor, dir string) DeviceIdentity {
	return resolveDeclaredDevice(ctx, newGitRepo(exec, dir))
}

// FormatRunProvenance renders the line an integrate run prints before any
// other notice, so a captured log names the device and the gz-git build that
// wrote it. A log without the line comes from a build older than it. An
// undeclared device adds one line saying how to declare it; an invalid one
// adds one line saying the declared name must be fixed.
func FormatRunProvenance(device DeviceIdentity, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "unknown"
	}
	line := fmt.Sprintf("integrate: run device=%s gz-git=%s\n", device.Label(), version)
	switch {
	case device.Rejected:
		line += fmt.Sprintf("integrate: the declared device name is invalid; fix %s or git config --global %s to lowercase letters, digits, single hyphens\n", callerHostEnv, callerHostGitKey)
	case !device.Declared():
		line += fmt.Sprintf("integrate: declare the device with %s=<name> or git config --global %s <name> (lowercase letters, digits, single hyphens)\n", callerHostEnv, callerHostGitKey)
	}
	return line
}

// WriteRunProvenance prints FormatRunProvenance to the run's notice stream.
func WriteRunProvenance(w io.Writer, device DeviceIdentity, version string) error {
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, FormatRunProvenance(device, version))
	return err
}
