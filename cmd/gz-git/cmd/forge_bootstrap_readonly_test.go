package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/forgebootstrap"
)

// readOnlyBootstrapPlanFixture keeps caller-selected input/output outside the
// graded repository while exercising the authenticated CLI against HTTPS.
func readOnlyBootstrapPlanFixture(t *testing.T) (args []string, verify func()) {
	t.Helper()
	artifacts, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var reads, writes atomic.Int64
	var remote string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes.Add(1)
			http.Error(w, "mutation forbidden", http.StatusMethodNotAllowed)
			return
		}
		reads.Add(1)
		if !strings.HasPrefix(r.URL.Path, "/api/v4/projects/") {
			t.Errorf("unexpected API operation %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("PRIVATE-TOKEN") != "readonly-plan-token" {
			t.Error("authenticated plan request missing fixture token")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"id": 42, "path_with_namespace": "fixture-group/fixture-site", "empty_repo": true, "default_branch": "master", "http_url_to_repo": remote}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	originalTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	t.Setenv("GITLAB_TOKEN", "readonly-plan-token")
	remote = server.URL + "/fixture-group/fixture-site.git"
	checkout := filepath.Join(artifacts, "checkout")
	request := forgebootstrap.Request{Namespace: "fixture-group", Project: "fixture-site", Remote: remote, Checkout: checkout, DefaultBranch: "master", IntegrationBranch: "develop", TemplateURL: server.URL + "/fixture-group/template.git", TemplateSHA: strings.Repeat("a", 40), Site: forgebootstrap.SiteConfig{"siteId": "fixture-site"}}
	input := filepath.Join(artifacts, "request.json")
	output := filepath.Join(artifacts, "plan.json")
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(input, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	savedInput, savedOutput, savedSeed := bootstrapRequestFile, bootstrapPlanFile, bootstrapSeed
	t.Cleanup(func() { bootstrapRequestFile = savedInput; bootstrapPlanFile = savedOutput; bootstrapSeed = savedSeed })
	args = []string{"forge", "bootstrap", "plan", "--input", input, "--output", output}
	verify = func() {
		t.Helper()
		if reads.Load() != 1 || writes.Load() != 0 {
			t.Errorf("plan API requests: reads=%d writes=%d", reads.Load(), writes.Load())
		}
		plan, err := forgebootstrap.ReadPlan(output)
		if err != nil {
			t.Errorf("successful plan output missing: %v", err)
			return
		}
		if plan.RemoteState.ProjectID != 42 || !plan.RemoteState.Empty || plan.Request.GitLabURL != server.URL {
			t.Errorf("plan did not bind observed HTTPS fixture: %+v", plan.RemoteState)
		}
		if _, err = os.Lstat(checkout); !os.IsNotExist(err) {
			t.Errorf("read-only plan created checkout: %v", err)
		}
		bytes, err := os.ReadFile(output)
		if err != nil {
			t.Error(err)
		} else if strings.Contains(string(bytes), "readonly-plan-token") {
			t.Error("authentication token entered plan")
		}
	}
	return args, verify
}
