package forgebootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestGitLabEmptyReuseAndNonemptyRefusal(t *testing.T) {
	for _, empty := range []bool{true, false} {
		t.Run(map[bool]string{true: "empty", false: "populated"}[empty], func(t *testing.T) {
			r := valid()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet {
					t.Error("reuse must never create a project")
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(gitlab.Project{ID: 541, PathWithNamespace: r.Namespace + "/" + r.Project, EmptyRepo: empty, SSHURLToRepo: r.Remote}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := gitlab.NewClient("test-secret", gitlab.WithBaseURL(server.URL+"/api/v4/"))
			if err != nil {
				t.Fatal(err)
			}
			err = (&GitLabRemote{Client: client}).Ensure(context.Background(), r)
			if (err == nil) != empty {
				t.Fatalf("empty=%v error=%v", empty, err)
			}
		})
	}
}

func TestGitLabCreateUsesResolvedGroupAndPrivateVisibility(t *testing.T) {
	r := valid()
	var created atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/api/v4/projects/"):
			w.WriteHeader(http.StatusNotFound)
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/api/v4/groups/"):
			if err := json.NewEncoder(w).Encode(gitlab.Group{ID: 670, FullPath: r.Namespace}); err != nil {
				t.Error(err)
			}
		case req.Method == http.MethodPost && req.URL.Path == "/api/v4/projects":
			var body struct {
				Namespace  int64  `json:"namespace_id"`
				Visibility string `json:"visibility"`
				Initialize bool   `json:"initialize_with_readme"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Namespace != 670 || body.Visibility != "private" || body.Initialize {
				t.Errorf("unexpected creation settings: %+v", body)
			}
			created.Store(true)
			if err := json.NewEncoder(w).Encode(gitlab.Project{ID: 541, PathWithNamespace: r.Namespace + "/" + r.Project, EmptyRepo: true, SSHURLToRepo: r.Remote}); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client, err := gitlab.NewClient("test-secret", gitlab.WithBaseURL(server.URL+"/api/v4/"))
	if err != nil {
		t.Fatal(err)
	}
	if err = (&GitLabRemote{Client: client}).Ensure(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if !created.Load() {
		t.Fatal("project was not created")
	}
}

func TestGitLabAuthorizationFailureNeverCreatesOrLeaksBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Error("authorization failure must not create")
		}
		w.WriteHeader(http.StatusForbidden)
		if _, err := w.Write([]byte("test-secret")); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := gitlab.NewClient("test-secret", gitlab.WithBaseURL(server.URL+"/api/v4/"))
	if err != nil {
		t.Fatal(err)
	}
	err = (&GitLabRemote{Client: client}).Ensure(context.Background(), valid())
	if err == nil || strings.Contains(err.Error(), "test-secret") || !strings.Contains(err.Error(), "403") {
		t.Fatalf("unsafe or absent error: %v", err)
	}
}

func TestGitLabConstructorRejectsUnsafeOrigins(t *testing.T) {
	for _, origin := range []string{"http://gitlab.example", "https://secret@gitlab.example", "https://gitlab.example/?token=secret", "https://gitlab.example/#secret", "https://gitlab.example/path", "://bad", "file:///tmp/server"} {
		t.Run(origin, func(t *testing.T) {
			if _, err := NewGitLabRemote(origin, "test-secret"); err == nil || strings.Contains(err.Error(), "test-secret") {
				t.Fatalf("unsafe constructor error: %v", err)
			}
		})
	}
	if _, err := NewGitLabRemote("https://gitlab.example", ""); err == nil {
		t.Fatal("accepted absent authentication")
	}
	for _, origin := range []string{"", "https://gitlab.example", "https://gitlab.example/"} {
		if client, err := NewGitLabRemote(origin, "test-secret"); err != nil || client.Client == nil {
			t.Fatalf("valid origin %q: %v", origin, err)
		}
	}
}

func TestGitLabInspectSnapshotsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		empty      bool
		mismatch   bool
		wantExists bool
		wantError  bool
	}{
		{name: "absent", status: 404},
		{name: "empty", status: 200, empty: true, wantExists: true},
		{name: "populated for resume", status: 200, wantExists: true},
		{name: "permission denied", status: 403, wantError: true},
		{name: "server error", status: 500, wantError: true},
		{name: "wrong identity", status: 200, mismatch: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls.Add(1)
				if req.Method != http.MethodGet {
					t.Error("Inspect must be read-only")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				projectPath := r.Namespace + "/" + r.Project
				if tc.mismatch {
					projectPath = "different/project"
				}
				if err := json.NewEncoder(w).Encode(gitlab.Project{ID: 541, PathWithNamespace: projectPath, EmptyRepo: tc.empty, SSHURLToRepo: r.Remote}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := gitlab.NewClient("test-secret", gitlab.WithBaseURL(server.URL+"/api/v4/"), gitlab.WithoutRetries())
			if err != nil {
				t.Fatal(err)
			}
			state, err := (&GitLabRemote{Client: client}).Inspect(context.Background(), r)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantError && (state.Exists != tc.wantExists || state.Exists && (state.ProjectID != 541 || state.Empty != tc.empty)) {
				t.Fatalf("incorrect snapshot: %+v", state)
			}
			if calls.Load() != 1 {
				t.Fatalf("Inspect retried unexpectedly: %d calls", calls.Load())
			}
		})
	}
}

func TestGitLabProjectIdentityValidation(t *testing.T) {
	r := valid()
	for _, p := range []*gitlab.Project{
		nil,
		{PathWithNamespace: r.Namespace + "/" + r.Project, Archived: true},
		{PathWithNamespace: r.Namespace + "/" + r.Project, DefaultBranch: "other"},
		{PathWithNamespace: r.Namespace + "/" + r.Project, DefaultBranch: r.DefaultBranch, SSHURLToRepo: "ssh://git@other.example/project.git"},
	} {
		if validateProjectIdentity(p, r) == nil {
			t.Fatalf("accepted mismatched project: %+v", p)
		}
	}
	if err := safeAPIError("inspect", nil); !strings.Contains(err.Error(), "transport error") {
		t.Fatal(err)
	}
}

func TestGitLabProvisionFailuresPreserveSafeErrorAndDoNotRetry(t *testing.T) {
	for _, stage := range []string{"group denied", "group mismatch", "create uncertain"} {
		t.Run(stage, func(t *testing.T) {
			r := valid()
			var posts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodPost:
					posts.Add(1)
					w.WriteHeader(500)
					if _, err := w.Write([]byte(`{"message":"test-secret"}`)); err != nil {
						t.Error(err)
					}
				case strings.HasPrefix(req.URL.Path, "/api/v4/projects/"):
					w.WriteHeader(404)
				case strings.HasPrefix(req.URL.Path, "/api/v4/groups/"):
					if stage == "group denied" {
						w.WriteHeader(403)
						return
					}
					groupPath := r.Namespace
					if stage == "group mismatch" {
						groupPath = "other/group"
					}
					if err := json.NewEncoder(w).Encode(gitlab.Group{ID: 670, FullPath: groupPath}); err != nil {
						t.Error(err)
					}
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
				}
			}))
			defer server.Close()
			client, err := gitlab.NewClient("test-secret", gitlab.WithBaseURL(server.URL+"/api/v4/"), gitlab.WithoutRetries())
			if err != nil {
				t.Fatal(err)
			}
			err = (&GitLabRemote{Client: client}).Ensure(context.Background(), r)
			if err == nil || strings.Contains(err.Error(), "test-secret") {
				t.Fatalf("unsafe or absent failure: %v", err)
			}
			wantPosts := int32(0)
			if stage == "create uncertain" {
				wantPosts = 1
			}
			if posts.Load() != wantPosts {
				t.Fatalf("mutation count %d, expected %d", posts.Load(), wantPosts)
			}
		})
	}
}

func TestGitLabEnsureProjectBindsPlannedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		expected  RemoteState
		status    int
		wantError bool
	}{
		{"same empty project", RemoteState{ProjectID: 541, Exists: true, Empty: true}, 200, false},
		{"replacement", RemoteState{ProjectID: 540, Exists: true, Empty: true}, 200, true},
		{"concurrent creation", RemoteState{}, 200, true},
		{"disappeared", RemoteState{ProjectID: 541, Exists: true, Empty: true}, 404, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls.Add(1)
				if req.Method != http.MethodGet {
					t.Error("identity drift must never create")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if err := json.NewEncoder(w).Encode(gitlab.Project{ID: 541, PathWithNamespace: r.Namespace + "/" + r.Project, EmptyRepo: true, SSHURLToRepo: r.Remote}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := gitlab.NewClient("synthetic-test-only", gitlab.WithBaseURL(server.URL+"/api/v4/"), gitlab.WithoutRetries())
			if err != nil {
				t.Fatal(err)
			}
			state, err := (&GitLabRemote{Client: client}).EnsureProject(context.Background(), r, tc.expected)
			if (err != nil) != tc.wantError {
				t.Fatalf("state=%+v error=%v", state, err)
			}
			if !tc.wantError && state.ProjectID != 541 {
				t.Fatalf("incorrect state %+v", state)
			}
			if calls.Load() != 1 {
				t.Fatalf("unexpected retry/create %d", calls.Load())
			}
		})
	}
}
