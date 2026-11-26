package v1alpha1

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	v2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	projectv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/namespaced/project/v1alpha1"
)

func TestResolveProjectReferences(t *testing.T) {
	s := runtime.NewScheme()
	if err := projectv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	p := &projectv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Namespace: "default"}}
	meta.SetExternalName(p, "wiz-project-id")
	kube := fake.NewClientBuilder().WithScheme(s).WithObjects(p).Build()

	u := &User{ObjectMeta: metav1.ObjectMeta{Name: "jane", Namespace: "default"}}
	u.Spec.ForProvider.AssignedProjectIdsRefs = []v2.NamespacedReference{{Name: "team-a"}}
	if err := u.ResolveReferences(context.Background(), kube); err != nil {
		t.Fatal(err)
	}
	if got := u.Spec.ForProvider.AssignedProjectIDs; len(got) != 1 || got[0] != "wiz-project-id" {
		t.Errorf("user assignedProjectIds = %v", got)
	}

	sa := &ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "ci", Namespace: "default"}}
	sa.Spec.ForProvider.AssignedProjectsRefs = []v2.NamespacedReference{{Name: "team-a"}}
	if err := sa.ResolveReferences(context.Background(), kube); err != nil {
		t.Fatal(err)
	}
	if got := sa.Spec.ForProvider.AssignedProjects; len(got) != 1 || got[0] != "wiz-project-id" {
		t.Errorf("service account assignedProjects = %v", got)
	}

	// Without references the field is left exactly as it is.
	empty := &User{}
	empty.Spec.ForProvider.AssignedProjectIDs = []string{}
	if err := empty.ResolveReferences(context.Background(), kube); err != nil || empty.Spec.ForProvider.AssignedProjectIDs == nil {
		t.Errorf("explicit [] changed to %v (err %v)", empty.Spec.ForProvider.AssignedProjectIDs, err)
	}
}
