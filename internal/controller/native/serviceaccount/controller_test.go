package serviceaccount

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/cluster/iam/v1alpha1"

	"github.com/polarpoint-io/provider-wiz/apis/common/serviceaccount"
	"github.com/polarpoint-io/provider-wiz/apis/namespaced/iam/v1alpha1"
)

const testID = "sa-1"

type fakeWiz struct {
	t        *testing.T
	accounts map[string]map[string]any
	last     map[string]any
}

func (f *fakeWiz) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OperationName string         `json:"operationName"`
		Query         string         `json:"query"`
		Variables     map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		f.t.Fatal(err)
	}
	var data map[string]any
	in, _ := req.Variables["input"].(map[string]any)
	switch req.OperationName {
	case "GetServiceAccount":
		if strings.Contains(req.Query, "clientSecret") {
			f.t.Error("GetServiceAccount asked for the client secret")
		}
		a, ok := f.accounts[req.Variables["id"].(string)]
		if !ok {
			data = map[string]any{"serviceAccount": nil}
			break
		}
		data = map[string]any{"serviceAccount": a}
	case "CreateServiceAccount":
		f.last = in
		ids, _ := in["assignedProjectIds"].([]any)
		ps := make([]any, 0, len(ids))
		for _, id := range ids {
			ps = append(ps, map[string]any{"id": id})
		}
		a := map[string]any{"id": testID, "name": in["name"], "clientId": "cid", "scopes": in["scopes"], "type": in["type"],
			"createdAt": "2026-11-26T00:00:00Z", "lastRotatedAt": "2026-11-26T00:00:00Z", "assignedProjects": ps}
		f.accounts[testID] = a
		withSecret := map[string]any{"clientSecret": "s3cret"}
		for k, v := range a {
			withSecret[k] = v
		}
		data = map[string]any{"createServiceAccount": map[string]any{"serviceAccount": withSecret}}
	case "DeleteServiceAccount":
		f.last = in
		delete(f.accounts, in["id"].(string))
		data = map[string]any{"deleteServiceAccount": map[string]any{"_stub": nil}}
	default:
		f.t.Fatalf("unexpected operation %q", req.OperationName)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
		f.t.Error(err)
	}
}

func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	fw := &fakeWiz{t: t, accounts: map[string]map[string]any{}}
	srv := httptest.NewServer(fw)
	defer srv.Close()
	e := &external{wiz: graphql.NewClient(srv.URL, srv.Client())}

	cr := &v1alpha1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "ci", Namespace: "default"}}
	cr.Spec.ForProvider.Parameters = serviceaccount.Parameters{Name: "ci", Scopes: []string{"read:projects", "admin:projects"}, AssignedProjects: []string{"p1"}}

	if obs, err := e.Observe(ctx, cr); err != nil || obs.ResourceExists {
		t.Fatalf("Observe before create: exists=%v err=%v", obs.ResourceExists, err)
	}
	c, err := e.Create(ctx, cr)
	if err != nil {
		t.Fatal(err)
	}
	if meta.GetExternalName(cr) != testID {
		t.Fatalf("external name = %q", meta.GetExternalName(cr))
	}
	if string(c.ConnectionDetails[ConnClientID]) != "cid" || string(c.ConnectionDetails[ConnClientSecret]) != "s3cret" {
		t.Errorf("connection details = %v", c.ConnectionDetails)
	}
	if fw.last["type"] != "THIRD_PARTY" {
		t.Errorf("type = %v, want THIRD_PARTY by default", fw.last["type"])
	}

	// Scope order does not matter.
	fw.accounts[testID]["scopes"] = []any{"admin:projects", "read:projects"}
	obs, err := e.Observe(ctx, cr)
	if err != nil || !obs.ResourceExists || !obs.ResourceUpToDate {
		t.Fatalf("Observe after create: %+v err=%v", obs, err)
	}
	if len(obs.ConnectionDetails) != 0 {
		t.Errorf("Observe returned connection details %v; it must not overwrite the secret", obs.ConnectionDetails)
	}
	if cr.Status.AtProvider.ClientID != "cid" || cr.Status.AtProvider.LastRotatedAt == "" {
		t.Errorf("observation = %+v", cr.Status.AtProvider)
	}

	// A change Wiz cannot apply is reported, not silently ignored.
	cr.Spec.ForProvider.AssignedProjects = []string{"p1", "p2"}
	if obs, _ = e.Observe(ctx, cr); obs.ResourceUpToDate {
		t.Fatal("want not up to date after changing assigned projects")
	}
	if _, err := e.Update(ctx, cr); err == nil {
		t.Fatal("want Update to fail: service accounts are immutable")
	}

	if _, err := e.Delete(ctx, cr); err != nil {
		t.Fatal(err)
	}
	if obs, _ = e.Observe(ctx, cr); obs.ResourceExists {
		t.Fatal("want deleted service account to read as gone")
	}
}

func TestOtherTypesSendNoScopes(t *testing.T) {
	broker := serviceaccount.Type("BROKER")
	in := toCreateInput(serviceaccount.Parameters{Name: "b", Type: &broker, Scopes: []string{"read:all"}})
	if in.Scopes != nil || in.AssignedProjectIds != nil || in.Type == nil || *in.Type != "BROKER" {
		t.Errorf("create input = %+v", in)
	}
}

func TestLostSecretIsReported(t *testing.T) {
	ctx := context.Background()
	fw := &fakeWiz{t: t, accounts: map[string]map[string]any{testID: {"id": testID, "name": "ci", "type": "THIRD_PARTY"}}}
	srv := httptest.NewServer(fw)
	defer srv.Close()

	namespaced := &v1alpha1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "ci", Namespace: "team"}}
	namespaced.Spec.ForProvider.Name = "ci"
	namespaced.Spec.WriteConnectionSecretToReference = &xpv2.LocalSecretReference{Name: "creds"}
	meta.SetExternalName(namespaced, testID)

	cluster := &clusterv1alpha1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "ci"}}
	cluster.Spec.ForProvider.Name = "ci"
	cluster.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{Name: "creds", Namespace: "ops"}
	meta.SetExternalName(cluster, testID)

	observeOnly := namespaced.DeepCopy()
	observeOnly.Spec.ManagementPolicies = xpv2.ManagementPolicies{xpv2.ManagementActionObserve}

	cases := map[string]struct {
		cr      serviceAccountResource
		secrets []client.Object
		lost    bool
	}{
		"NamespacedMissingSecret": {cr: namespaced, lost: true},
		"NamespacedWithSecret": {cr: namespaced, secrets: []client.Object{
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "team"}, Data: map[string][]byte{ConnClientSecret: []byte("s")}},
		}},
		"ClusterSecretWithoutKey": {cr: cluster, lost: true, secrets: []client.Object{
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ops"}, Data: map[string][]byte{ConnClientID: []byte("cid")}},
		}},
		"ClusterUpjetSecret": {cr: cluster, secrets: []client.Object{
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ops"}, Data: map[string][]byte{legacyConnClientSecret: []byte("s")}},
		}},
		"ObserveOnly": {cr: observeOnly},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kube := fake.NewClientBuilder().WithObjects(tc.secrets...).Build()
			e := &external{wiz: graphql.NewClient(srv.URL, srv.Client()), kube: kube}
			obs, err := e.Observe(ctx, tc.cr)
			if !obs.ResourceExists {
				t.Fatal("want the account to exist")
			}
			if (err != nil) != tc.lost {
				t.Errorf("Observe error = %v, want lost=%v", err, tc.lost)
			}
		})
	}
}
