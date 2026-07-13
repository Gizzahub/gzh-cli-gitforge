package cmd

import (
	"github.com/gizzahub/gzh-cli-gitforge/pkg/workspacecli"
)

func init() {
	factory := workspacecli.CommandFactory{}

	rootCmd.AddCommand(factory.NewRootCmd())
}
