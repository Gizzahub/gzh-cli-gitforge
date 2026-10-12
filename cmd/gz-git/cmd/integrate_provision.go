// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

var (
	provisionSources []string
	provisionLeaseID string
)

var (
	integrateProvisionCmd         = &cobra.Command{Use: "provision", Short: "Provision declared local sibling dependencies"}
	integrateProvisionSiblingsCmd = &cobra.Command{Use: "siblings", Args: cobra.NoArgs}
	integrateProvisionAcquireCmd  = &cobra.Command{
		Use:   "acquire [--source name=/absolute/primary-checkout] --json",
		Short: "Archive declared siblings into this task worktree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := provisionWorkingRoot()
			if err != nil {
				return err
			}
			decl, present, err := integrate.LoadSiblingProvision(root)
			if err != nil {
				return err
			}
			if !present {
				return fmt.Errorf("branch.siblingProvision is not declared")
			}
			sources, err := parseProvisionSources(provisionSources)
			if err != nil {
				return err
			}
			leases := make([]integrate.SiblingLease, 0, len(decl.Siblings))
			for _, dep := range decl.Siblings {
				source, ok := sources[dep.Name]
				if !ok {
					source, err = integrate.DiscoverSiblingPrimary(cmd.Context(), root, dep)
					if err != nil {
						return errors.Join(err, rollbackProvisionLeases(cmd.Context(), root, leases))
					}
				}
				lease, provisionErr := integrate.ProvisionSibling(cmd.Context(), root, source, dep)
				if provisionErr != nil {
					return errors.Join(provisionErr, rollbackProvisionLeases(cmd.Context(), root, leases))
				}
				lease, provisionErr = integrate.PersistSiblingLease(cmd.Context(), root, lease)
				if provisionErr != nil {
					return errors.Join(provisionErr, integrate.ReleaseSibling(lease), rollbackProvisionLeases(cmd.Context(), root, leases))
				}
				leases = append(leases, lease)
			}
			if err := writeProvisionResponse(cmd, "ACQUIRED", leases); err != nil {
				return errors.Join(err, rollbackProvisionLeases(cmd.Context(), root, leases))
			}
			return nil
		},
	}
)

var integrateProvisionReleaseCmd = &cobra.Command{Use: "release --lease ID --json", Args: cobra.NoArgs, Short: "Release one provisioned sibling lease", RunE: func(cmd *cobra.Command, _ []string) error {
	root, err := provisionWorkingRoot()
	if err != nil {
		return err
	}
	lease, path, err := integrate.LoadPersistedSiblingLease(cmd.Context(), root, provisionLeaseID)
	if err != nil {
		return err
	}
	if lease.Root != filepath.Clean(root) {
		return fmt.Errorf("sibling lease belongs to another task worktree")
	}
	if err := integrate.ReleaseSibling(lease); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	b, err := json.Marshal(struct {
		SchemaVersion int    `json:"schemaVersion"`
		Status        string `json:"status"`
		LeaseID       string `json:"leaseId"`
	}{1, "RELEASED", provisionLeaseID})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return err
}}

var integrateProvisionInspectCmd = &cobra.Command{Use: "inspect --json", Args: cobra.NoArgs, Short: "List resumable sibling leases", RunE: func(cmd *cobra.Command, _ []string) error {
	root, err := provisionWorkingRoot()
	if err != nil {
		return err
	}
	leases, err := integrate.InspectPersistedSiblingLeases(cmd.Context(), root)
	if err != nil {
		return err
	}
	return writeProvisionResponse(cmd, "RESUMABLE", leases)
}}

func writeProvisionResponse(cmd *cobra.Command, status string, leases []integrate.SiblingLease) error {
	type reference struct {
		ID   string `json:"leaseId"`
		Name string `json:"name"`
		OID  string `json:"oid"`
	}
	refs := make([]reference, 0, len(leases))
	for _, lease := range leases {
		refs = append(refs, reference{lease.ID, lease.Name, lease.OID})
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
		SchemaVersion int         `json:"schemaVersion"`
		Status        string      `json:"status"`
		Leases        []reference `json:"leases"`
	}{1, status, refs})
}

func provisionWorkingRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(root)
}

func rollbackProvisionLeases(ctx context.Context, root string, leases []integrate.SiblingLease) error {
	var err error
	for i := len(leases) - 1; i >= 0; i-- {
		lease, path, loadErr := integrate.LoadPersistedSiblingLease(ctx, root, leases[i].ID)
		if loadErr != nil {
			err = errors.Join(err, loadErr)
			continue
		}
		if lease.Root != filepath.Clean(root) {
			err = errors.Join(err, fmt.Errorf("rollback lease belongs to another worktree"))
			continue
		}
		if releaseErr := integrate.ReleaseSibling(lease); releaseErr != nil {
			err = errors.Join(err, releaseErr)
			continue
		}
		err = errors.Join(err, os.Remove(path))
	}
	return err
}

func parseProvisionSources(values []string) (map[string]string, error) {
	out := make(map[string]string, len(values))
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, "=")
		if !ok || name == "" || value == "" || filepath.Clean(value) != value || !filepath.IsAbs(value) {
			return nil, fmt.Errorf("--source must be name=/absolute/primary-checkout")
		}
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("duplicate --source for %s", name)
		}
		out[name] = value
	}
	return out, nil
}

func init() {
	integrateCmd.AddCommand(integrateProvisionCmd)
	integrateProvisionCmd.AddCommand(integrateProvisionSiblingsCmd)
	integrateProvisionSiblingsCmd.AddCommand(integrateProvisionAcquireCmd)
	integrateProvisionSiblingsCmd.AddCommand(integrateProvisionReleaseCmd)
	integrateProvisionSiblingsCmd.AddCommand(integrateProvisionInspectCmd)
	integrateProvisionAcquireCmd.Flags().StringSliceVar(&provisionSources, "source", nil, "declared sibling source as name=/absolute/primary-checkout")
	integrateProvisionAcquireCmd.Flags().Bool("json", true, "print JSON response")
	integrateProvisionReleaseCmd.Flags().StringVar(&provisionLeaseID, "lease", "", "opaque sibling lease ID")
	integrateProvisionReleaseCmd.Flags().Bool("json", true, "print JSON response")
	integrateProvisionInspectCmd.Flags().Bool("json", true, "print JSON response")
	_ = integrateProvisionReleaseCmd.MarkFlagRequired("lease")
}
