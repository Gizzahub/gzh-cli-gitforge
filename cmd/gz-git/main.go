// Package main is the entry point for the gz-git CLI application.
// gz-git provides advanced Git operations through a command-line interface.
package main

import (
	gzhcligitforge "github.com/gizzahub/gzh-cli-gitforge"
	"github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git/cmd"
)

// version is set during build time via ldflags.
var version = "dev"

func main() {
	cmd.Execute(resolveVersion(version))
}

// resolveVersion covers the one install path ldflags cannot reach:
// `go install .../cmd/gz-git@<tag>` builds from the module proxy, so its binary
// would report the "dev" default forever no matter which tag produced it. The
// root package's init already read the version the toolchain embedded for
// exactly that build, under the same "ldflags wins" rule -- report what it
// found so both version surfaces agree.
func resolveVersion(stamped string) string {
	if stamped != "dev" {
		return stamped
	}
	return gzhcligitforge.Version
}

// init sets up global application state.
func init() {
	// Ensure clean exit on interrupt
	// Additional initialization can be added here
}
