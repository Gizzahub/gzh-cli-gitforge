//nolint:gosec,errcheck,gocritic,revive,tagliatelle
package forgebootstrap

import (
	"context"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLabRemote struct{ Client *gitlab.Client }

func NewGitLabRemote(baseURL, token string) (*GitLabRemote, error) {
	if token == "" {
		return nil, fmt.Errorf("GITLAB_TOKEN is required")
	}
	var c *gitlab.Client
	var e error
	if baseURL == "" {
		c, e = gitlab.NewClient(token)
	} else {
		c, e = gitlab.NewClient(token, gitlab.WithBaseURL(strings.TrimRight(baseURL, "/")+"/api/v4/"))
	}
	if e != nil {
		return nil, e
	}
	return &GitLabRemote{c}, nil
}

func (g *GitLabRemote) Ensure(ctx context.Context, r Request) error {
	path := r.Namespace + "/" + r.Project
	if _, resp, e := g.Client.Projects.GetProject(path, nil, gitlab.WithContext(ctx)); e == nil || resp != nil && resp.StatusCode < 300 {
		return fmt.Errorf("project already exists: %s", path)
	}
	name := r.Project
	master := r.DefaultBranch
	falseVal := false
	_, _, e := g.Client.Projects.CreateProject(&gitlab.CreateProjectOptions{Name: &name, Path: &name, DefaultBranch: &master, InitializeWithReadme: &falseVal}, gitlab.WithContext(ctx))
	return e
}
