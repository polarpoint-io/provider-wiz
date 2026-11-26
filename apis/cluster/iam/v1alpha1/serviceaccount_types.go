package v1alpha1

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	projectv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/cluster/project/v1alpha1"
	"github.com/polarpoint-io/provider-wiz/apis/common/serviceaccount"
)

// ServiceAccountParameters are the ServiceAccount parameters plus references that fill in
// assignedProjects from Project resources.
type ServiceAccountParameters struct {
	serviceaccount.Parameters `json:",inline"`

	// References to Project resources to fill in assignedProjects.
	// +optional
	AssignedProjectsRefs []v2.Reference `json:"assignedProjectsRefs,omitempty"`
	// Selector for Project resources to fill in assignedProjects.
	// +optional
	AssignedProjectsSelector *v2.Selector `json:"assignedProjectsSelector,omitempty"`
}

// ServiceAccountSpec defines the desired state of a ServiceAccount.
type ServiceAccountSpec struct {
	v2.ClusterManagedResourceSpec `json:",inline"`
	ForProvider                   ServiceAccountParameters `json:"forProvider"`
}

// ServiceAccountStatus defines the observed state of a ServiceAccount.
type ServiceAccountStatus struct {
	v2.ManagedResourceStatus `json:",inline"`
	AtProvider               serviceaccount.Observation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// ServiceAccount is a Wiz service account, managed directly through the
// Wiz GraphQL API. Its client ID and secret are written to the connection
// secret, so it can feed a ProviderConfig. Wiz cannot update a service
// account; change it by deleting and recreating the resource.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,wiz}
type ServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceAccountSpec   `json:"spec"`
	Status            ServiceAccountStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceAccountList contains a list of ServiceAccounts
type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccount `json:"items"`
}

// ServiceAccount type metadata.
var (
	ServiceAccount_Kind             = "ServiceAccount"
	ServiceAccount_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: ServiceAccount_Kind}.String()
	ServiceAccount_KindAPIVersion   = ServiceAccount_Kind + "." + CRDGroupVersion.String()
	ServiceAccount_GroupVersionKind = CRDGroupVersion.WithKind(ServiceAccount_Kind)
)

func init() {
	SchemeBuilder.Register(&ServiceAccount{}, &ServiceAccountList{})
}

// GetParameters returns the desired Wiz serviceaccount parameters.
func (mg *ServiceAccount) GetParameters() *serviceaccount.Parameters {
	return &mg.Spec.ForProvider.Parameters
}

// SetObservation records the observed Wiz serviceaccount state.
func (mg *ServiceAccount) SetObservation(o serviceaccount.Observation) { mg.Status.AtProvider = o }

// ResolveReferences of this ServiceAccount. It is written by hand because the
// referencing field lives in the shared parameters.
func (mg *ServiceAccount) ResolveReferences(ctx context.Context, c client.Reader) error {
	p := &mg.Spec.ForProvider
	if len(p.AssignedProjectsRefs) == 0 && p.AssignedProjectsSelector == nil {
		return nil
	}
	r := reference.NewAPIResolver(c, mg)
	mrsp, err := r.ResolveMultiple(ctx, reference.MultiResolutionRequest{
		CurrentValues: p.AssignedProjects,
		Extract:       reference.ExternalName(),
		References:    p.AssignedProjectsRefs,
		Selector:      p.AssignedProjectsSelector,
		To:            reference.To{List: &projectv1alpha1.ProjectList{}, Managed: &projectv1alpha1.Project{}},
	})
	if err != nil {
		return errors.Wrap(err, "mg.Spec.ForProvider.AssignedProjects")
	}
	p.AssignedProjects = mrsp.ResolvedValues
	p.AssignedProjectsRefs = mrsp.ResolvedReferences
	return nil
}
