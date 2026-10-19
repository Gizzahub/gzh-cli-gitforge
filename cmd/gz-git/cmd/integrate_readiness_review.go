// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

var readinessReviewBranch, readinessReviewTarget, readinessReviewIssuer, readinessReviewExpectSource, readinessReviewStatement string

var integrateReadinessUpdateReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review and apply a readiness update in one interactive transaction",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := config.LoadReadinessApproval()
		if err != nil {
			return cliutil.NewExitError(2, err)
		}
		if !stdinIsInteractive() {
			return cliutil.NewExitError(2, fmt.Errorf("readiness update review requires an interactive terminal"))
		}
		return runReadinessUpdateReview(cmd, false)
	},
}

var integrateReadinessUpdateAutoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Apply a statement-bound readiness update when globally enabled",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mode, err := config.LoadReadinessApproval()
		if err != nil {
			return cliutil.NewExitError(2, err)
		}
		if mode != config.ReadinessApprovalAutomatic {
			return cliutil.NewExitError(2, fmt.Errorf("readiness update auto is disabled by global integration.readinessApproval policy"))
		}
		if readinessReviewExpectSource == "" {
			return cliutil.NewExitError(2, fmt.Errorf("--expect-source is required"))
		}
		if readinessReviewStatement == "" {
			return cliutil.NewExitError(2, fmt.Errorf("--statement is required"))
		}
		return runReadinessUpdateReview(cmd, true)
	},
}

var integrateReadinessUpdateRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Apply a readiness update using the global approval mode",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mode, err := config.LoadReadinessApproval()
		if err != nil {
			return cliutil.NewExitError(2, err)
		}
		automatic := mode == config.ReadinessApprovalAutomatic
		if !automatic && !stdinIsInteractive() {
			return cliutil.NewExitError(2, fmt.Errorf("readiness update run requires an interactive terminal when global integration.readinessApproval is manual"))
		}
		if automatic && readinessReviewExpectSource == "" {
			return cliutil.NewExitError(2, fmt.Errorf("--expect-source is required"))
		}
		if automatic && readinessReviewStatement == "" {
			return cliutil.NewExitError(2, fmt.Errorf("--statement is required"))
		}
		return runReadinessUpdateReview(cmd, automatic)
	},
}

var integrateReadinessUpdatePolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Print the global readiness approval mode",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mode, err := config.LoadReadinessApproval()
		if err != nil {
			return cliutil.NewExitError(2, err)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), mode)
		return err
	},
}

func runReadinessUpdateReview(cmd *cobra.Command, automatic bool) error {
	printIntegrateProvenance(cmd)
	ctx, exec := cmdContext(cmd), gitcmd.NewExecutor()
	issuer, err := resolveIssuer(ctx, exec, ".", readinessReviewIssuer)
	if err != nil {
		return cliutil.NewExitError(2, err)
	}
	plan, err := integrate.ReviewReadinessUpdate(ctx, exec, integrate.ReadinessUpdateReviewOptions{
		RepoPath:     ".",
		Branch:       readinessReviewBranch,
		Target:       readinessReviewTarget,
		Issuer:       issuer,
		ExpectSource: readinessReviewExpectSource,
		Statement:    readinessReviewStatement,
		Automatic:    automatic,
		Notice:       cmd.ErrOrStderr(),
		Review: func(p integrate.ReadinessUpdatePlan, diff string) (bool, error) {
			fmt.Fprintf(cmd.ErrOrStderr(), "repository: %s\nsource: %s\ntarget: %s (%s)\n%s", escapeTerminalControls(p.Repository), escapeTerminalControls(p.SourceSHA), escapeTerminalControls(p.TargetRef), escapeTerminalControls(p.TargetSHA), escapeTerminalControls(diff))
			fmt.Fprint(cmd.ErrOrStderr(), "\nApply this readiness update? [yes/NO]: ")
			return interactiveReadinessApproval(cmd.InOrStdin())
		},
	})
	if err != nil {
		return cliutil.NewExitError(1, err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "READINESS_UPDATED", plan.TargetRef, plan.SourceSHA)
	return nil
}

func interactiveReadinessApproval(input io.Reader) (bool, error) {
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(answer), "yes"), nil
}

// escapeTerminalControls prevents a contract diff from rewriting the reviewer's
// terminal. Tabs and newlines remain formatting; controls, including Unicode
// directional/format controls, and invalid UTF-8 bytes are rendered literally.
func escapeTerminalControls(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	for value != "" {
		r, size := utf8.DecodeRuneInString(value)
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&out, "\\x%02x", value[0])
			value = value[1:]
			continue
		}
		value = value[size:]
		if (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) && r != '\n' && r != '\t' {
			if r <= 0xff {
				fmt.Fprintf(&out, "\\x%02x", r)
			} else {
				fmt.Fprintf(&out, "\\u%04x", r)
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func init() {
	integrateReadinessUpdateCmd.AddCommand(integrateReadinessUpdateReviewCmd, integrateReadinessUpdateAutoCmd, integrateReadinessUpdateRunCmd, integrateReadinessUpdatePolicyCmd)
	for _, cmd := range []*cobra.Command{integrateReadinessUpdateReviewCmd, integrateReadinessUpdateAutoCmd, integrateReadinessUpdateRunCmd} {
		cmd.Flags().StringVar(&readinessReviewBranch, "branch", "", "source branch")
		cmd.Flags().StringVar(&readinessReviewTarget, "target", "", "target ref")
		cmd.Flags().StringVar(&readinessReviewIssuer, "issuer", "", "identity recorded in the update (default: git config gzgit.issuer, then user.email)")
	}
	for _, cmd := range []*cobra.Command{integrateReadinessUpdateAutoCmd, integrateReadinessUpdateRunCmd} {
		cmd.Flags().StringVar(&readinessReviewExpectSource, "expect-source", "", "required full source commit SHA in automatic mode")
		cmd.Flags().StringVar(&readinessReviewStatement, "statement", "", "required completion statement in automatic mode")
	}
}
