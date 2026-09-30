// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	gzhcligitforge "github.com/gizzahub/gzh-cli-gitforge"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

type genDocsOptions struct {
	manDir      string
	markdownDir string
}

// newGenDocsCommand writes the command tree as man pages (git-style
// hyphenated names, e.g. gz-git-integrate-bootstrap-plan.1) and markdown.
// It is an installer's tool, hidden from help, and kept apart from help
// because it writes files under names it chooses.
func newGenDocsCommand() *cobra.Command {
	opts := &genDocsOptions{}
	c := &cobra.Command{
		Use:    "gen-docs [command]",
		Short:  "Write man and markdown pages for the command tree",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if opts.manDir == "" && opts.markdownDir == "" {
				return cliutil.NewExitError(2, fmt.Errorf("set --man-dir, --markdown-dir, or both"))
			}
			target, _, err := c.Root().Find(args)
			if err != nil || target == nil {
				return cliutil.NewExitError(2, fmt.Errorf("unknown command %v", args))
			}
			return writeDocTrees(target, opts)
		},
	}
	c.Flags().StringVar(&opts.manDir, "man-dir", "", "write man pages for the tree under [command] into this directory")
	c.Flags().StringVar(&opts.markdownDir, "markdown-dir", "", "write markdown pages for the tree under [command] into this directory")
	return c
}

// treeState is what Cobra's generators change on a command: they add a
// help child to every parent and set DisableAutoGenTag up the parent chain,
// and the effect line is appended to Long for the render.
type treeState struct {
	long       string
	noGenTag   bool
	helpBefore bool
}

func snapshotTree(root *cobra.Command) map[*cobra.Command]treeState {
	states := map[*cobra.Command]treeState{}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		st := treeState{long: c.Long, noGenTag: c.DisableAutoGenTag}
		for _, child := range c.Commands() {
			if child.Name() == "help" {
				st.helpBefore = true
			}
			walk(child)
		}
		states[c] = st
	}
	walk(root)
	return states
}

func restoreTree(states map[*cobra.Command]treeState) {
	for c, st := range states {
		c.Long = st.long
		c.DisableAutoGenTag = st.noGenTag
		if st.helpBefore {
			continue
		}
		for _, child := range c.Commands() {
			if child.Name() == "help" {
				c.RemoveCommand(child)
			}
		}
	}
}

// manDate pins the man header date so a rebuild of the same commit writes
// the same pages: SOURCE_DATE_EPOCH first, then the commit date the build
// stamps into BuildDate, then the Unix epoch.
func manDate() (time.Time, error) {
	if s := os.Getenv("SOURCE_DATE_EPOCH"); s != "" {
		sec, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH %q is not a Unix timestamp", s)
		}
		return time.Unix(sec, 0).UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, gzhcligitforge.BuildDate); err == nil {
		return t.UTC(), nil
	}
	return time.Unix(0, 0).UTC(), nil
}

func writeDocTrees(target *cobra.Command, opts *genDocsOptions) error {
	root := target.Root()
	defer restoreTree(snapshotTree(root))
	root.DisableAutoGenTag = true
	_ = walkReference(target, func(c *cobra.Command) error {
		if c.Runnable() {
			body := c.Long
			if body == "" {
				body = c.Short
			}
			c.Long = body + "\n\nEffect: " + effectSummary(c)
		}
		return nil
	})

	if opts.manDir != "" {
		// #nosec G301 -- generated man pages must stay readable to other users of the install prefix.
		if err := os.MkdirAll(opts.manDir, 0o755); err != nil {
			return cliutil.NewExitError(2, fmt.Errorf("create man directory: %w", err))
		}
		date, err := manDate()
		if err != nil {
			return cliutil.NewExitError(2, err)
		}
		header := &doc.GenManHeader{Section: "1", Date: &date, Source: "gz-git " + root.Version, Manual: "gz-git Manual"}
		if err := doc.GenManTree(target, header, opts.manDir); err != nil {
			return cliutil.NewExitError(1, fmt.Errorf("write man pages: %w", err))
		}
	}
	if opts.markdownDir != "" {
		// #nosec G301 -- generated reference pages must stay readable to other users of the install prefix.
		if err := os.MkdirAll(opts.markdownDir, 0o755); err != nil {
			return cliutil.NewExitError(2, fmt.Errorf("create markdown directory: %w", err))
		}
		if err := genMarkdownTree(target, opts.markdownDir); err != nil {
			return cliutil.NewExitError(1, fmt.Errorf("write markdown pages: %w", err))
		}
	}
	return nil
}

// docName is the git-style page name shared by man and markdown pages:
// "gz-git integrate bootstrap plan" becomes gz-git-integrate-bootstrap-plan.
func docName(c *cobra.Command) string {
	return strings.ReplaceAll(c.CommandPath(), " ", "-")
}

// genMarkdownTree mirrors doc.GenMarkdownTree but names pages like the man
// pages; Cobra's own tree hard-codes underscore file names. Command names
// never contain an underscore, so rewriting the See Also links is exact.
func genMarkdownTree(c *cobra.Command, dir string) error {
	for _, child := range c.Commands() {
		if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := genMarkdownTree(child, dir); err != nil {
			return err
		}
	}
	// #nosec G304 -- the directory is the operator's --markdown-dir and the name comes from the command tree.
	f, err := os.Create(filepath.Join(dir, docName(c)+".md"))
	if err != nil {
		return err
	}
	link := func(name string) string { return strings.ReplaceAll(name, "_", "-") }
	if err := doc.GenMarkdownCustom(c, f, link); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func init() {
	rootCmd.AddCommand(newGenDocsCommand())
}
