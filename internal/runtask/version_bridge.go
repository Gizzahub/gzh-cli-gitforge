package runtask

import "strings"

const unknownToolRevision = "unknown"

// runtimeVersion and runtimeRevision carry the build-stamped identity so
// provider reports and receipts record the same strings the version command
// prints. The command wiring sets them at startup; tests may pin them.
var (
	runtimeVersion  string
	runtimeRevision string
)

// SetRuntimeVersion records the build version for lifecycle reporting.
func SetRuntimeVersion(v string) { runtimeVersion = strings.TrimSpace(v) }

// SetRuntimeRevision records the build commit for receipt evidence. An
// unstamped build says unknown rather than inventing an identifier.
func SetRuntimeRevision(v string) { runtimeRevision = strings.TrimSpace(v) }

func gzGitRuntimeVersion() string {
	if runtimeVersion == "" {
		return "gz-git version 0.0.0"
	}
	return "gz-git version " + runtimeVersion
}

func toolRevision() string {
	if runtimeRevision == "" {
		return unknownToolRevision
	}
	return runtimeRevision
}

// toolVersion stamps finish receipts with the identity of the process that
// performed the integration.
func toolVersion() string {
	return gzGitRuntimeVersion()
}
