package forgebootstrap

import (
	"strings"
	"testing"
	"time"
)

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
