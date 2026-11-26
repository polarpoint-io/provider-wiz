package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarpoint-io/provider-wiz/apis/common/user"
	"github.com/polarpoint-io/provider-wiz/apis/namespaced/iam/v1alpha1"
)

const testID = "u-1"

// fakeWiz is an in-memory stand-in for the Wiz GraphQL API that understands
// the user operations.
type fakeWiz struct {
	t     *testing.T
	users map[string]map[string]any
	last  map[string]any // input of the last mutation
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
	in, _ := req.Variables["input"].(map[string]any)
	switch req.OperationName {
	case "GetUser":
		u, ok := f.users[req.Variables["id"].(string)]
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"user":null},"errors":[{"message":"User not found","extensions":{"code":"NOT_FOUND"}}]}`))
			return
		}
		data = map[string]any{"user": u}
	case "CreateUser":
		f.last = in
		u := map[string]any{"id": testID, "name": in["name"], "email": in["email"], "isSuspended": false,
			"effectiveRole": map[string]any{"id": in["role"]}, "effectiveAssignedProjects": projects(in["assignedProjectIds"])}
		f.users[testID] = u
		data = map[string]any{"createUser": map[string]any{"user": u}}
	case "UpdateUser":
		f.last = in
		u := f.users[in["id"].(string)]
		for k, v := range in["patch"].(map[string]any) {
			switch k {
			case "role":
				u["effectiveRole"] = map[string]any{"id": v}
			case "assignedProjectIds":
				u["effectiveAssignedProjects"] = projects(v)
			default:
				u[k] = v
			}
		}
		data = map[string]any{"updateUser": map[string]any{"user": u}}
	case "DeleteUser":
		f.last = in
		delete(f.users, in["id"].(string))
		data = map[string]any{"deleteUser": map[string]any{"_stub": nil}}
	default:
		f.t.Fatalf("unexpected operation %q", req.OperationName)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
		f.t.Error(err)
	}
}

func projects(v any) []any {
	ids, _ := v.([]any)
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"id": id})
	}
	return out
}

func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	fw := &fakeWiz{t: t, users: map[string]map[string]any{}}
	srv := httptest.NewServer(fw)
	defer srv.Close()
	e := &external{wiz: graphql.NewClient(srv.URL, srv.Client())}

	cr := &v1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "jane", Namespace: "default"}}
	cr.Spec.ForProvider.Parameters = user.Parameters{Name: "Jane", Email: "Jane@Example.com", Role: "PROJECT_READER", AssignedProjectIDs: []string{"p2", "p1"}}

	if obs, err := e.Observe(ctx, cr); err != nil || obs.ResourceExists {
		t.Fatalf("Observe before create: exists=%v err=%v", obs.ResourceExists, err)
	}
	if _, err := e.Create(ctx, cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != testID {
		t.Fatalf("external name = %q", got)
	}
	if fw.last["sendEmailInvite"] != true {
		t.Errorf("sendEmailInvite = %v, want true by default", fw.last["sendEmailInvite"])
	}

	// Wiz stores the email lower-cased and the projects in another order.
	fw.users[testID]["email"] = "jane@example.com"
	fw.users[testID]["effectiveAssignedProjects"] = projects([]any{"p1", "p2"})
	obs, err := e.Observe(ctx, cr)
	if err != nil || !obs.ResourceExists || !obs.ResourceUpToDate {
		t.Fatalf("Observe after create: %+v err=%v", obs, err)
	}
	if cr.Status.AtProvider.EffectiveRole != "PROJECT_READER" {
		t.Errorf("observation = %+v", cr.Status.AtProvider)
	}

	// Removing every project assignment with [] is sent, not omitted.
	cr.Spec.ForProvider.AssignedProjectIDs = []string{}
	if obs, _ = e.Observe(ctx, cr); obs.ResourceUpToDate {
		t.Fatal("want not up to date after clearing assignments")
	}
	if _, err := e.Update(ctx, cr); err != nil {
		t.Fatal(err)
	}
	patch := fw.last["patch"].(map[string]any)
	if l, ok := patch["assignedProjectIds"].([]any); !ok || len(l) != 0 {
		t.Errorf("patch assignedProjectIds = %#v, want []", patch["assignedProjectIds"])
	}
	if _, sent := patch["isSuspended"]; sent {
		t.Error("unset isSuspended was sent")
	}
	if obs, _ = e.Observe(ctx, cr); !obs.ResourceUpToDate {
		t.Fatal("want up to date after update")
	}

	// Unset assignments are not managed.
	cr.Spec.ForProvider.AssignedProjectIDs = nil
	fw.users[testID]["effectiveAssignedProjects"] = projects([]any{"p9"})
	if obs, _ = e.Observe(ctx, cr); !obs.ResourceUpToDate {
		t.Fatal("want up to date while assignedProjectIds is unset")
	}
	if _, err := e.Update(ctx, cr); err != nil {
		t.Fatal(err)
	}
	if _, sent := fw.last["patch"].(map[string]any)["assignedProjectIds"]; sent {
		t.Error("unset assignedProjectIds was sent")
	}

	if _, err := e.Delete(ctx, cr); err != nil {
		t.Fatal(err)
	}
	if obs, _ = e.Observe(ctx, cr); obs.ResourceExists {
		t.Fatal("want deleted user to read as gone")
	}
	// Deleting again is not an error.
	if _, err := e.Delete(ctx, cr); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}
