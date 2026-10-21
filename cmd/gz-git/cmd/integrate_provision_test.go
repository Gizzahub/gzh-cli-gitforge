// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

func provisionTestGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture git %v: %v %s", args, err, output)
	}
}

func provisionCommandFixture(t *testing.T, collision bool) (*testutil.WorktreeOrigin, *bytes.Buffer) {
	t.Helper()
	source := testutil.TempWorktreeWithBareOrigin(t)
	owner := testutil.TempWorktreeWithBareOrigin(t)
	remote := "ssh://git@example.test/source.git"
	provisionTestGit(t, source.Clone, "push", "origin", "HEAD:master")
	provisionTestGit(t, source.Clone, "fetch", "origin")
	provisionTestGit(t, source.Clone, "remote", "set-url", "origin", remote)
	provisionTestGit(t, source.Clone, "remote", "set-url", "--push", "origin", remote)
	manifest := fmt.Sprintf("branch:\n  siblingProvision:\n    version: 1\n    siblings:\n      - name: first\n        remote: %q\n        path: ../snapshot\n        revision: source-tip\n        sourceBranch: master\n", remote)
	if collision {
		manifest += fmt.Sprintf("      - name: second\n        remote: %q\n        path: ../origin.git\n        revision: source-tip\n        sourceBranch: master\n", remote)
	}
	if err := os.WriteFile(filepath.Join(owner.Clone, ".gz-git.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(owner.Clone)
	previousSources, previousID := provisionSources, provisionLeaseID
	provisionSources = []string{"first=" + source.Clone}
	if collision {
		provisionSources = append(provisionSources, "second="+source.Clone)
	}
	output := &bytes.Buffer{}
	previousAcquire := integrateProvisionAcquireCmd.OutOrStdout()
	previousInspect := integrateProvisionInspectCmd.OutOrStdout()
	previousRelease := integrateProvisionReleaseCmd.OutOrStdout()
	previousAcquireContext := integrateProvisionAcquireCmd.Context()
	previousInspectContext := integrateProvisionInspectCmd.Context()
	previousReleaseContext := integrateProvisionReleaseCmd.Context()
	integrateProvisionAcquireCmd.SetContext(context.Background())
	integrateProvisionInspectCmd.SetContext(context.Background())
	integrateProvisionReleaseCmd.SetContext(context.Background())
	integrateProvisionAcquireCmd.SetOut(output)
	integrateProvisionInspectCmd.SetOut(output)
	integrateProvisionReleaseCmd.SetOut(output)
	t.Cleanup(func() {
		provisionSources, provisionLeaseID = previousSources, previousID
		integrateProvisionAcquireCmd.SetOut(previousAcquire)
		integrateProvisionInspectCmd.SetOut(previousInspect)
		integrateProvisionReleaseCmd.SetOut(previousRelease)
		integrateProvisionAcquireCmd.SetContext(previousAcquireContext)
		integrateProvisionInspectCmd.SetContext(previousInspectContext)
		integrateProvisionReleaseCmd.SetContext(previousReleaseContext)
	})
	return owner, output
}

// TestProvisionCLIAcquireRejectsUndeclaredSourceName covers ISSUE-49: a
// --source name no declared sibling matches is a usage error (exit 2) that
// names the unknown name and the declared ones, and nothing is acquired.
func TestProvisionCLIAcquireRejectsUndeclaredSourceName(t *testing.T) {
	owner, _ := provisionCommandFixture(t, false)
	provisionSources = append(provisionSources, "ghost="+owner.Clone)

	err := integrateProvisionAcquireCmd.RunE(integrateProvisionAcquireCmd, nil)
	if err == nil {
		t.Fatal("undeclared --source name was accepted")
	}
	var exitErr *cliutil.ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("error = %v, want a usage error with exit code 2", err)
	}
	if !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "first") {
		t.Fatalf("error %q lacks the unknown or the declared source name", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(owner.Clone), "snapshot")); !os.IsNotExist(statErr) {
		t.Fatalf("rejected acquire still provisioned a snapshot: %v", statErr)
	}
	leases, leasesErr := integrate.InspectPersistedSiblingLeases(context.Background(), owner.Clone)
	if leasesErr != nil || len(leases) != 0 {
		t.Fatalf("rejected acquire left leases: %v %v", leases, leasesErr)
	}
}

func TestProvisionCLILeaseJSONAndForeignWorktreeRefusal(t *testing.T) {
	owner, output := provisionCommandFixture(t, false)
	if err := integrateProvisionAcquireCmd.RunE(integrateProvisionAcquireCmd, nil); err != nil {
		t.Fatal(err)
	}
	var acquired struct {
		SchemaVersion int    `json:"schemaVersion"`
		Status        string `json:"status"`
		Leases        []struct {
			ID string `json:"leaseId"`
		} `json:"leases"`
	}
	if err := json.Unmarshal(output.Bytes(), &acquired); err != nil {
		t.Fatal(err)
	}
	if acquired.SchemaVersion != 1 || acquired.Status != "ACQUIRED" || len(acquired.Leases) != 1 || len(acquired.Leases[0].ID) != 32 {
		t.Fatalf("invalid public lease response: %s", output.Bytes())
	}
	provisionLeaseID = acquired.Leases[0].ID
	output.Reset()
	if err := integrateProvisionInspectCmd.RunE(integrateProvisionInspectCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"status":"RESUMABLE"`)) || !bytes.Contains(output.Bytes(), []byte(provisionLeaseID)) {
		t.Fatalf("inspect lost the lease: %s", output.Bytes())
	}
	if err := os.Chdir(owner.Worktree); err != nil {
		t.Fatal(err)
	}
	if err := integrateProvisionReleaseCmd.RunE(integrateProvisionReleaseCmd, nil); err == nil {
		t.Fatal("foreign worktree released a lease")
	}
	if err := os.Chdir(owner.Clone); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := integrateProvisionReleaseCmd.RunE(integrateProvisionReleaseCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"status":"RELEASED"`)) {
		t.Fatalf("release did not acknowledge completion: %s", output.Bytes())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(owner.Clone), "snapshot")); !os.IsNotExist(err) {
		t.Fatalf("released snapshot remains: %v", err)
	}
	for _, command := range []*cobra.Command{integrateProvisionAcquireCmd, integrateProvisionInspectCmd, integrateProvisionReleaseCmd} {
		if command.Flags().Lookup("json") == nil {
			t.Fatalf("%s lacks --json capability", command.Name())
		}
	}
}

func TestProvisionCLIPartialFailureRollsBackOnlyItsSnapshots(t *testing.T) {
	owner, _ := provisionCommandFixture(t, true)
	if err := integrateProvisionAcquireCmd.RunE(integrateProvisionAcquireCmd, nil); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(owner.Clone), "snapshot")); !os.IsNotExist(err) {
		t.Fatalf("partial snapshot remains: %v", err)
	}
	leases, err := integrate.InspectPersistedSiblingLeases(context.Background(), owner.Clone)
	if err != nil || len(leases) != 0 {
		t.Fatalf("partial registry lease remains: %v %v", leases, err)
	}
	provisionTestGit(t, owner.Clone, "--git-dir", owner.Origin, "rev-parse", "HEAD")
}

func TestProvisionCLIInspectRejectsChangedOwnershipAndRevision(t *testing.T) {
	for _, change := range []string{"marker", "incomplete", "source-tip", "dirty-source", "destination"} {
		t.Run(change, func(t *testing.T) {
			owner, output := provisionCommandFixture(t, false)
			if err := integrateProvisionAcquireCmd.RunE(integrateProvisionAcquireCmd, nil); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Leases []struct {
					ID string `json:"leaseId"`
				} `json:"leases"`
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Leases) != 1 {
				t.Fatal("missing acquired lease")
			}
			lease, _, err := integrate.LoadPersistedSiblingLease(context.Background(), owner.Clone, result.Leases[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "marker":
				if err := os.WriteFile(filepath.Join(lease.Destination, ".gz-git-sibling-lease.json"), []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "incomplete", "destination":
				path := filepath.Join(owner.Clone, ".gz-git.yaml")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				text := string(data)
				if change == "incomplete" {
					text += strings.Replace(strings.Split(text, "    siblings:\n")[1], "name: first", "name: second", 1)
				} else {
					text = strings.Replace(text, "../snapshot", "../different", 1)
				}
				if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			case "source-tip":
				provisionTestGit(t, lease.Source, "commit", "--allow-empty", "-m", "new-tip")
				provisionTestGit(t, lease.Source, "update-ref", "refs/remotes/origin/master", "HEAD")
			case "dirty-source":
				if err := os.WriteFile(filepath.Join(lease.Source, "new.txt"), []byte("dirty"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			output.Reset()
			if err := integrateProvisionInspectCmd.RunE(integrateProvisionInspectCmd, nil); err == nil {
				t.Fatal("changed lease reported resumable or empty")
			}
			if _, err := os.Stat(lease.Destination); err != nil {
				t.Fatalf("changed lease removed: %v", err)
			}
		})
	}
}
