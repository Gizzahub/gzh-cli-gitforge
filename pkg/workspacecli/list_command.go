// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	configpkg "github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// listFormats are the output formats `workspace list` accepts.
var listFormats = []string{"default", "json"}

// workspaceListReport is the JSON document `workspace list --format json`
// prints. Consumers match its field names, so they are a contract.
type workspaceListReport struct {
	Config     string                        `json:"config"`
	Workspaces []configpkg.DeclaredWorkspace `json:"workspaces"`
}

func (f CommandFactory) newListCmd() *cobra.Command {
	var configPath string
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the workspaces a config declares",
		Long: `List the top-level workspaces a config file declares.

Reads only the config file: nested configs, checkouts and remotes are not
touched, so the list shows what the config declares, not what is on disk.

` + cliutil.QuickStartHelp(`  # List workspaces of the auto-detected config
  gz-git workspace list

  # List workspaces of a specific config as JSON
  gz-git workspace list -c myworkspace.yaml --format json`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cliutil.ValidateFormat(format, listFormats); err != nil {
				return err
			}
			if configPath == "" {
				detected, err := detectConfigFile(".")
				if err != nil {
					return fmt.Errorf("no config file specified and auto-detection failed: %w", err)
				}
				configPath = detected
			}
			// The report names the file in one form however it was given,
			// like every workspace path it lists.
			absConfig, err := filepath.Abs(configPath)
			if err != nil {
				return fmt.Errorf("resolve config path %s: %w", configPath, err)
			}
			workspaces, err := configpkg.ListDeclaredWorkspaces(absConfig)
			if err != nil {
				return err
			}
			report := workspaceListReport{Config: absConfig, Workspaces: workspaces}
			if format == "json" {
				return writeWorkspaceListJSON(cmd.OutOrStdout(), report)
			}
			return writeWorkspaceListTable(cmd.OutOrStdout(), report)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file (auto-detects "+DefaultConfigFile+")")
	cmd.Flags().StringVar(&format, "format", "default", "Output format: "+strings.Join(listFormats, ", "))

	return cmd
}

func writeWorkspaceListJSON(w io.Writer, report workspaceListReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeWorkspaceListTable(w io.Writer, report workspaceListReport) error {
	if len(report.Workspaces) == 0 {
		_, err := fmt.Fprintf(w, "No workspaces declared in %s\n", report.Config)
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tACCESS\tBRANCH\tPATH")
	for _, ws := range report.Workspaces {
		branch := strings.Join(ws.DefaultBranch, ",")
		if branch == "" {
			branch = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", ws.Name, ws.Type, ws.Access, branch, ws.Path)
	}
	return tw.Flush()
}
