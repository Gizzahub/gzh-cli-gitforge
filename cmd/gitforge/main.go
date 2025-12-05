package main

import (
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/cmd/gitforge/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
