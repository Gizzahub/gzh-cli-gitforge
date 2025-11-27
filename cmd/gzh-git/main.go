// Package main is the entry point for the gzh-git CLI application.
// gzh-git provides advanced Git operations through a command-line interface.
package main

import (
	"github.com/gizzahub/gzh-cli-git/cmd/gzh-git/cmd"
)

// version is set during build time via ldflags
var version = "dev"

func main() {
	cmd.Execute(version)
}

// init sets up global application state
func init() {
	// Ensure clean exit on interrupt
	// Additional initialization can be added here
}
