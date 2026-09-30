// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// commandReferenceSchema versions the JSON form of `help --all`. Consumers
// such as the agent hook pin it, so a breaking change needs a new value.
const commandReferenceSchema = "gz-git-command-reference/v1"

type helpOptions struct {
	all    bool
	format string
}

// newHelpCommand replaces Cobra's default help command. Without flags it
// behaves like the default; --all walks a whole subtree in one run so a
// reader never has to call --help once per level. help only prints: writing
// man or markdown pages is gen-docs, so help stays read-only under every flag.
func newHelpCommand() *cobra.Command {
	opts := &helpOptions{}
	c := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Simply type gz-git help [path to command] for full details.

--all prints every command under [command] (default: all commands) in one
run, with its usage, flags, and declared effect. Installed man pages are
read with man gz-git-<command>, for example man gz-git-integrate-bootstrap-plan.`,
		Example: `  gz-git help integrate
  gz-git help --all
  gz-git help --all --format json integrate`,
		RunE: func(c *cobra.Command, args []string) error {
			return runHelp(c, args, opts)
		},
	}
	c.Flags().BoolVar(&opts.all, "all", false, "print the whole command tree under [command] in one run")
	c.Flags().StringVar(&opts.format, "format", "text", "output format for --all: text or json")
	return c
}

func runHelp(c *cobra.Command, args []string, opts *helpOptions) error {
	target, _, err := c.Root().Find(args)
	if err != nil || target == nil {
		if !opts.all {
			// Keep Cobra's default answer for a plain unknown topic.
			c.Printf("Unknown help topic %#q\n", args)
			return c.Root().Usage()
		}
		return cliutil.NewExitError(2, fmt.Errorf("unknown help topic %q", strings.Join(args, " ")))
	}

	if !opts.all {
		if c.Flags().Changed("format") {
			return cliutil.NewExitError(2, fmt.Errorf("--format requires --all"))
		}
		target.InitDefaultHelpFlag()
		target.InitDefaultVersionFlag()
		return target.Help()
	}

	switch opts.format {
	case "text":
		return writeReferenceText(c.OutOrStdout(), target)
	case "json":
		return writeReferenceJSON(c.OutOrStdout(), target)
	default:
		return cliutil.NewExitError(2, fmt.Errorf("unsupported --format %q (want text or json)", opts.format))
	}
}

// walkReference visits target and every available command under it,
// depth first, in the order help lists them. The root's help command is
// included the way Cobra's usage template includes it, so a consumer that
// denies undeclared commands still finds help declared.
func walkReference(target *cobra.Command, visit func(*cobra.Command) error) error {
	if err := visit(target); err != nil {
		return err
	}
	for _, child := range target.Commands() {
		isRootHelp := child.Name() == "help" && !target.HasParent()
		if !child.IsAvailableCommand() && !isRootHelp {
			continue
		}
		if err := walkReference(child, visit); err != nil {
			return err
		}
	}
	return nil
}

func writeReferenceText(w io.Writer, target *cobra.Command) error {
	var b strings.Builder
	if global := target.Root().PersistentFlags().FlagUsages(); global != "" {
		b.WriteString("Global Flags:\n")
		b.WriteString(indent(global, "  "))
		b.WriteString("\n")
	}
	_ = walkReference(target, func(c *cobra.Command) error {
		b.WriteString(c.CommandPath())
		b.WriteString("\n")
		if c.Short != "" {
			fmt.Fprintf(&b, "  %s\n", c.Short)
		}
		if !c.Runnable() {
			b.WriteString("  Subcommands only\n\n")
			return nil
		}
		fmt.Fprintf(&b, "  Usage:  %s\n", c.UseLine())
		fmt.Fprintf(&b, "  Effect: %s\n", effectSummary(c))
		if local := c.LocalNonPersistentFlags().FlagUsages(); local != "" {
			b.WriteString("  Flags:\n")
			b.WriteString(indent(local, "  "))
		}
		b.WriteString("\n")
		return nil
	})
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write help reference: %w", err)
	}
	return nil
}

type referenceFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Usage     string `json:"usage"`
}

type referenceCommand struct {
	Path     string          `json:"path"`
	Short    string          `json:"short,omitempty"`
	Usage    string          `json:"usage,omitempty"`
	Runnable bool            `json:"runnable"`
	Effect   string          `json:"effect,omitempty"`
	Mutates  []string        `json:"mutates,omitempty"`
	Flags    []referenceFlag `json:"flags,omitempty"`
}

type commandReference struct {
	Schema      string             `json:"schema"`
	Version     string             `json:"version"`
	GlobalFlags []referenceFlag    `json:"globalFlags"`
	Commands    []referenceCommand `json:"commands"`
}

func writeReferenceJSON(w io.Writer, target *cobra.Command) error {
	ref := commandReference{
		Schema:      commandReferenceSchema,
		Version:     target.Root().Version,
		GlobalFlags: referenceFlags(target.Root().PersistentFlags()),
	}
	_ = walkReference(target, func(c *cobra.Command) error {
		rc := referenceCommand{Path: c.CommandPath(), Short: c.Short, Runnable: c.Runnable()}
		if rc.Runnable {
			rc.Usage = c.UseLine()
			rc.Effect, rc.Mutates, _ = declaredEffect(c)
			rc.Flags = referenceFlags(c.LocalNonPersistentFlags())
		}
		ref.Commands = append(ref.Commands, rc)
		return nil
	})
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ref); err != nil {
		return fmt.Errorf("write help reference: %w", err)
	}
	return nil
}

func referenceFlags(fs *pflag.FlagSet) []referenceFlag {
	flags := []referenceFlag{}
	fs.VisitAll(func(f *pflag.Flag) {
		// -h/--help exists only once Cobra initializes it (Execute, doc
		// generators), so listing it would make the reference depend on
		// what ran earlier in the process.
		if f.Hidden || f.Name == "help" {
			return
		}
		flags = append(flags, referenceFlag{
			Name: f.Name, Shorthand: f.Shorthand, Type: f.Value.Type(), Default: f.DefValue, Usage: f.Usage,
		})
	})
	return flags
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func init() {
	rootCmd.SetHelpCommand(newHelpCommand())
}
