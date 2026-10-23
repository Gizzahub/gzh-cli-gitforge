// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package forgebootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// GitLabRemote creates or reuses an empty project in an explicit group.
type GitLabRemote struct{ Client *gitlab.Client }

// NewGitLabRemote keeps authentication outside persisted plans and receipts.
func NewGitLabRemote(baseURL, token string) (*GitLabRemote, error) {
	if token == "" {
		return nil, errors.New("GITLAB_TOKEN is required")
	}
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("GitLab API origin must be HTTPS without credentials, query or path")
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	c, err := gitlab.NewClient(token,
		gitlab.WithBaseURL(strings.TrimRight(baseURL, "/")+"/api/v4/"),
		gitlab.WithHTTPClient(httpClient), gitlab.WithoutRetries())
	if err != nil {
		return nil, errors.New("cannot configure GitLab client")
	}
	return &GitLabRemote{Client: c}, nil
}

// Ensure preserves the adapter's direct empty-project create/reuse interface.
func (g *GitLabRemote) Ensure(ctx context.Context, r Request) error {
	_, err := g.ensureProject(ctx, r, nil)
	return err
}

// EnsureProject provisions only the exact planned state and returns its identity.
// A concurrent project creation or replacement is never adopted by this call.
func (g *GitLabRemote) EnsureProject(ctx context.Context, r Request, expected RemoteState) (RemoteState, error) {
	return g.ensureProject(ctx, r, &expected)
}

func (g *GitLabRemote) ensureProject(ctx context.Context, r Request, expected *RemoteState) (RemoteState, error) {
	p, resp, err := g.Client.Projects.GetProject(r.Namespace+"/"+r.Project, nil, gitlab.WithContext(ctx))
	if err == nil {
		if expected != nil && (!expected.Exists || p == nil || p.ID != expected.ProjectID) {
			return RemoteState{}, errors.New("GitLab project identity changed after planning")
		}
		return projectState(p, r)
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		return RemoteState{}, safeAPIError("inspect project", resp)
	}
	if expected != nil && expected.Exists {
		return RemoteState{}, errors.New("planned GitLab project disappeared")
	}
	group, resp, err := g.Client.Groups.GetGroup(r.Namespace, nil, gitlab.WithContext(ctx))
	if err != nil {
		return RemoteState{}, safeAPIError("inspect namespace", resp)
	}
	if group == nil || group.FullPath != r.Namespace || group.ID <= 0 {
		return RemoteState{}, errors.New("GitLab returned a different namespace")
	}
	name, master, empty, visibility := r.Project, r.DefaultBranch, false, gitlab.PrivateVisibility
	p, resp, err = g.Client.Projects.CreateProject(&gitlab.CreateProjectOptions{
		Name: &name, Path: &name, NamespaceID: &group.ID, DefaultBranch: &master,
		InitializeWithReadme: &empty, Visibility: &visibility,
	}, gitlab.WithContext(ctx))
	if err != nil {
		return RemoteState{}, safeAPIError("create project (outcome must be inspected before retry)", resp)
	}
	return projectState(p, r)
}

func projectState(p *gitlab.Project, r Request) (RemoteState, error) {
	if err := validateProject(p, r); err != nil {
		return RemoteState{}, err
	}
	return RemoteState{ProjectID: p.ID, Exists: true, Empty: true}, nil
}

// Inspect observes project identity without creating or changing resources.
// Populated projects are reported for verified resume; fresh plans reject them.
func (g *GitLabRemote) Inspect(ctx context.Context, r Request) (RemoteState, error) {
	p, resp, err := g.Client.Projects.GetProject(r.Namespace+"/"+r.Project, nil, gitlab.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return RemoteState{}, nil
		}
		return RemoteState{}, safeAPIError("inspect project", resp)
	}
	if err := validateProjectIdentity(p, r); err != nil {
		return RemoteState{}, err
	}
	return RemoteState{ProjectID: p.ID, Exists: true, Empty: p.EmptyRepo}, nil
}

func validateProject(p *gitlab.Project, r Request) error {
	if err := validateProjectIdentity(p, r); err != nil {
		return err
	}
	if !p.EmptyRepo {
		return errors.New("GitLab project contains repository content")
	}
	return nil
}

func validateProjectIdentity(p *gitlab.Project, r Request) error {
	if p == nil || p.ID <= 0 || p.PathWithNamespace != r.Namespace+"/"+r.Project || p.Archived {
		return errors.New("GitLab project identity is incompatible")
	}
	if p.DefaultBranch != "" && p.DefaultBranch != r.DefaultBranch {
		return errors.New("GitLab project default branch differs from plan")
	}
	if r.Remote != p.SSHURLToRepo && r.Remote != p.HTTPURLToRepo {
		return errors.New("GitLab project remote differs from plan")
	}
	return nil
}

// API response bodies and errors can echo credentials; retain only status.
func safeAPIError(operation string, resp *gitlab.Response) error {
	if resp == nil {
		return fmt.Errorf("GitLab %s failed: transport error", operation)
	}
	return fmt.Errorf("GitLab %s failed: HTTP %d", operation, resp.StatusCode)
}
