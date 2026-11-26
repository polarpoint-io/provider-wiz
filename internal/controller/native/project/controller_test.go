package project

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarpoint-io/provider-wiz/apis/common/project"
	"github.com/polarpoint-io/provider-wiz/apis/namespaced/project/v1alpha1"
)

const testID = "p-1"

// fakeWiz is an in-memory stand-in for the Wiz GraphQL API that understands
// the three project operations.
type fakeWiz struct {
	t        *testing.T
	projects map[string]map[string]any
	last     map[string]any // variables of the last mutation
}

func (f *fakeWiz) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		f.t.Fatal(err)
	}
	var data map[string]any
	switch req.OperationName {
	case "GetProject":
		p := f.projects[req.Variables["id"].(string)]
		data = map[string]any{"project": p}
	case "CreateProject":
		f.last = req.Variables
		in := req.Variables["input"].(map[string]any)
		p := map[string]any{"id": testID, "name": in["name"], "slug": "generated-slug", "isFolder": false, "archived": false, "riskProfile": map[string]any{}}
		for _, k := range []string{"description", "projectOwners"} {
			if v, ok := in[k]; ok {
				p[k] = v
			}
		}
		if owners, ok := in["projectOwners"].([]any); ok {
			us := make([]any, 0, len(owners))
			for _, o := range owners {
				us = append(us, map[string]any{"id": o})
			}
			p["projectOwners"] = us
		}
		f.projects[testID] = p
		data = map[string]any{"createProject": map[string]any{"project": p}}
	case "UpdateProject":
		f.last = req.Variables
		in := req.Variables["input"].(map[string]any)
		p := f.projects[in["id"].(string)]
		for _, key := range []string{"override", "patch"} {
			if o, ok := in[key].(map[string]any); ok {
				for k, v := range o {
					switch k {
					case "projectOwners":
					case "cloudAccountLinks":
						p[k] = readLinks(v)
					default:
						p[k] = v
					}
				}
			}
		}
		data = map[string]any{"updateProject": map[string]any{"project": p}}
	default:
		f.t.Fatalf("unexpected operation %q", req.OperationName)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
		f.t.Error(err)
	}
}

// readLinks turns cloudAccountLinks input into the shape a query returns.
func readLinks(v any) []any {
	in := v.([]any)
	out := make([]any, 0, len(in))
	for _, l := range in {
		m := l.(map[string]any)
		out = append(out, map[string]any{"cloudAccount": map[string]any{"id": m["cloudAccount"]}, "environment": m["environment"]})
	}
	return out
}

func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	fw := &fakeWiz{t: t, projects: map[string]map[string]any{}}
	srv := httptest.NewServer(fw)
	defer srv.Close()
	e := &external{wiz: graphql.NewClient(srv.URL, srv.Client())}

	cr := &v1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Namespace: "default"}}
	cr.Spec.ForProvider = project.Parameters{Name: "Team A", Description: ptr("first"), ProjectOwners: []string{"u2", "u1"}}

	// Nothing exists yet.
	obs, err := e.Observe(ctx, cr)
	if err != nil || obs.ResourceExists {
		t.Fatalf("Observe before create: exists=%v err=%v", obs.ResourceExists, err)
	}

	if _, err := e.Create(ctx, cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != testID {
		t.Fatalf("external name = %q", got)
	}
	in := fw.last["input"].(map[string]any)
	if _, sent := in["businessUnit"]; sent {
		t.Error("unset optional field was sent; want it omitted")
	}

	// Exists, up to date, slug late-initialized from Wiz.
	obs, err = e.Observe(ctx, cr)
	if err != nil || !obs.ResourceExists || !obs.ResourceUpToDate || !obs.ResourceLateInitialized {
		t.Fatalf("Observe after create: %+v err=%v", obs, err)
	}
	if cr.Spec.ForProvider.Slug == nil || *cr.Spec.ForProvider.Slug != "generated-slug" {
		t.Errorf("slug not late-initialized: %v", cr.Spec.ForProvider.Slug)
	}
	if cr.Status.AtProvider.ID != testID {
		t.Errorf("observation not set: %+v", cr.Status.AtProvider)
	}

	// Drift: the spec changes the description.
	cr.Spec.ForProvider.Description = ptr("second")
	if obs, _ = e.Observe(ctx, cr); obs.ResourceUpToDate {
		t.Fatal("want not up to date after spec change")
	}
	if _, err := e.Update(ctx, cr); err != nil {
		t.Fatal(err)
	}
	o := fw.last["input"].(map[string]any)["override"].(map[string]any)
	if o["description"] != "second" || o["slug"] != "generated-slug" {
		t.Errorf("override = %v", o)
	}
	if obs, _ = e.Observe(ctx, cr); !obs.ResourceUpToDate {
		t.Fatal("want up to date after update")
	}

	// Delete archives; once archived and deleting, it reads as gone.
	if _, err := e.Delete(ctx, cr); err != nil {
		t.Fatal(err)
	}
	p := fw.last["input"].(map[string]any)["override"].(map[string]any)
	if p["archived"] != true || p["name"] != "generated-slug" || p["description"] != "second" {
		t.Errorf("delete override = %v, want archived, renamed to the slug, other fields kept", p)
	}
	now := metav1.Now()
	cr.SetDeletionTimestamp(&now)
	if obs, _ = e.Observe(ctx, cr); obs.ResourceExists {
		t.Fatal("want archived project to read as deleted")
	}
}

func TestUpToDateIgnoresOrderAndUnsetFields(t *testing.T) {
	obs := project.Parameters{
		Name: "A", Slug: ptr("a"), BusinessUnit: ptr("bu"),
		ProjectOwners: []string{"u1", "u2"},
		CloudAccountLinks: []project.CloudAccountLink{
			{CloudAccount: "c2", Environment: "PRODUCTION"},
			{CloudAccount: "c1", Environment: "STAGING", Shared: ptr(false)},
		},
	}
	spec := project.Parameters{
		Name:          "A",
		ProjectOwners: []string{"u2", "u1"},
		CloudAccountLinks: []project.CloudAccountLink{
			{CloudAccount: "c1", Environment: "STAGING"},
			{CloudAccount: "c2", Environment: "PRODUCTION"},
		},
	}
	if !upToDate(spec, obs) {
		t.Error("want up to date: only order and unset fields differ")
	}
	spec.CloudAccountLinks = spec.CloudAccountLinks[:1]
	if upToDate(spec, obs) {
		t.Error("want not up to date: a cloud account link was removed")
	}
}

func TestExplicitEmptyListClearsLinks(t *testing.T) {
	ctx := context.Background()
	fw := &fakeWiz{t: t, projects: map[string]map[string]any{
		testID: {"id": testID, "name": "A", "slug": "a", "isFolder": false, "archived": false, "riskProfile": map[string]any{},
			"cloudAccountLinks": []any{map[string]any{"cloudAccount": map[string]any{"id": "c1"}, "environment": "PRODUCTION"}}},
	}}
	srv := httptest.NewServer(fw)
	defer srv.Close()
	e := &external{wiz: graphql.NewClient(srv.URL, srv.Client())}

	cr := &v1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"}}
	meta.SetExternalName(cr, testID)
	cr.Spec.ForProvider = project.Parameters{Name: "A"}

	// Unset: the existing link is left alone.
	if obs, _ := e.Observe(ctx, cr); !obs.ResourceUpToDate {
		t.Fatal("want up to date while cloudAccountLinks is unset")
	}

	// The empty list must survive the API round trip...
	raw, err := json.Marshal(project.Parameters{Name: "A", CloudAccountLinks: []project.CloudAccountLink{}})
	if err != nil {
		t.Fatal(err)
	}
	var back project.Parameters
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.CloudAccountLinks == nil {
		t.Fatalf("explicit empty list lost in round trip: %s", raw)
	}

	// ...and clear the link in Wiz.
	cr.Spec.ForProvider = back
	if obs, _ := e.Observe(ctx, cr); obs.ResourceUpToDate {
		t.Fatal("want not up to date when [] removes the last link")
	}
	if _, err := e.Update(ctx, cr); err != nil {
		t.Fatal(err)
	}
	o := fw.last["input"].(map[string]any)["override"].(map[string]any)
	if l, ok := o["cloudAccountLinks"].([]any); !ok || len(l) != 0 {
		t.Errorf("override cloudAccountLinks = %#v, want []", o["cloudAccountLinks"])
	}
	if obs, _ := e.Observe(ctx, cr); !obs.ResourceUpToDate {
		t.Fatal("want up to date after the link was cleared")
	}
}
