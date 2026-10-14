package forgebootstrap

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeRemote struct{}

func (fakeRemote) Ensure(context.Context, Request) error { return nil }

func valid() Request {
	return Request{Namespace: "service-hub/singlepageweb", Project: "singlepageweb-template", Remote: "https://gitlab.example/service-hub/singlepageweb-template.git", Checkout: "/tmp/site", DefaultBranch: "master", IntegrationBranch: "develop", TemplateURL: "https://gitlab.example/service-hub/base.git", TemplateSHA: strings.Repeat("a", 40), Site: SiteConfig{"name": "template"}}
}
func TestPlanDigestRoundTrip(t *testing.T) {
	p, e := NewPlan(valid(), time.Unix(10, 0))
	if e != nil {
		t.Fatal(e)
	}
	if p.PlanDigest != Digest(p) {
		t.Fatal("digest mismatch")
	}
	p.Request.Project = "changed"
	if p.PlanDigest == Digest(p) {
		t.Fatal("digest did not bind request")
	}
}
func TestValidateRejectsUnsafeInputs(t *testing.T) {
	r := valid()
	r.Project = "Bad Name"
	if ValidateRequest(r) == nil {
		t.Fatal("accepted project")
	}
	r = valid()
	r.TemplateSHA = strings.Repeat("A", 40)
	if ValidateRequest(r) != nil {
		t.Fatal("uppercase should normalize for validation")
	}
}

func TestApplyRejectsOccupiedCheckoutBeforeRemoteMutation(t *testing.T) {
	r := valid()
	d := t.TempDir()
	r.Checkout = d
	if err := os.WriteFile(d+"/existing", []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := NewPlan(r, time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	called := false
	remote := remoteFunc(func(context.Context, Request) error { called = true; return nil })
	_, err = Apply(context.Background(), p, ApplyOptions{Remote: remote})
	if err == nil || !strings.Contains(err.Error(), "occupied") {
		t.Fatalf("error = %v", err)
	}
	if called {
		t.Fatal("remote was mutated before path validation")
	}
}

type remoteFunc func(context.Context, Request) error

func (f remoteFunc) Ensure(c context.Context, r Request) error { return f(c, r) }
