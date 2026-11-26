package v1alpha1

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/polarpoint-io/provider-wiz/apis/common/user"
	projectv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/namespaced/project/v1alpha1"
)

// UserParameters are the User parameters plus references that fill in
// assignedProjectIds from Project resources.
type UserParameters struct {
	user.Parameters `json:",inline"`

	// References to Project resources to fill in assignedProjectIds.
	// +optional
	AssignedProjectIdsRefs []v2.NamespacedReference `json:"assignedProjectIdsRefs,omitempty"`
	// Selector for Project resources to fill in assignedProjectIds.
	// +optional
	AssignedProjectIdsSelector *v2.NamespacedSelector `json:"assignedProjectIdsSelector,omitempty"`
}

// UserSpec defines the desired state of a User.
type UserSpec struct {
	v2.ManagedResourceSpec `json:",inline"`
	ForProvider            UserParameters `json:"forProvider"`
}

// UserStatus defines the observed state of a User.
type UserStatus struct {
	v2.ManagedResourceStatus `json:",inline"`
	AtProvider               user.Observation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// User is a Wiz user, managed directly through the Wiz GraphQL API.
// Deleting this resource deletes the user from Wiz.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.email)",message="spec.forProvider.email is a required parameter"
// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.name)",message="spec.forProvider.name is a required parameter"
// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.role)",message="spec.forProvider.role is a required parameter"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,wiz}
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserSpec   `json:"spec"`
	Status            UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of Users
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// User type metadata.
var (
	User_Kind             = "User"
	User_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: User_Kind}.String()
	User_KindAPIVersion   = User_Kind + "." + CRDGroupVersion.String()
	User_GroupVersionKind = CRDGroupVersion.WithKind(User_Kind)
)

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}

// GetParameters returns the desired Wiz user parameters.
func (mg *User) GetParameters() *user.Parameters { return &mg.Spec.ForProvider.Parameters }

// SetObservation records the observed Wiz user state.
func (mg *User) SetObservation(o user.Observation) { mg.Status.AtProvider = o }

// ResolveReferences of this User. It is written by hand because the
// referencing field lives in the shared parameters.
func (mg *User) ResolveReferences(ctx context.Context, c client.Reader) error {
	p := &mg.Spec.ForProvider
	if len(p.AssignedProjectIdsRefs) == 0 && p.AssignedProjectIdsSelector == nil {
		return nil
	}
	r := reference.NewAPINamespacedResolver(c, mg)
	mrsp, err := r.ResolveMultiple(ctx, reference.MultiNamespacedResolutionRequest{
		CurrentValues: p.AssignedProjectIDs,
		Namespace:     mg.GetNamespace(),
		Extract:       reference.ExternalName(),
		References:    p.AssignedProjectIdsRefs,
		Selector:      p.AssignedProjectIdsSelector,
		To:            reference.To{List: &projectv1alpha1.ProjectList{}, Managed: &projectv1alpha1.Project{}},
	})
	if err != nil {
		return errors.Wrap(err, "mg.Spec.ForProvider.AssignedProjectIDs")
	}
	p.AssignedProjectIDs = mrsp.ResolvedValues
	p.AssignedProjectIdsRefs = mrsp.ResolvedReferences
	return nil
}
