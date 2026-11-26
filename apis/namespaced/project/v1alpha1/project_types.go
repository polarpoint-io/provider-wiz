package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/polarpoint-io/provider-wiz/apis/common/project"
)

// ProjectSpec defines the desired state of a Project.
type ProjectSpec struct {
	v2.ManagedResourceSpec `json:",inline"`
	ForProvider            project.Parameters `json:"forProvider"`
}

// ProjectStatus defines the observed state of a Project.
type ProjectStatus struct {
	v2.ManagedResourceStatus `json:",inline"`
	AtProvider               project.Observation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Project is a Wiz project, managed directly through the Wiz GraphQL API.
// Projects group cloud resources by owner or purpose. Wiz cannot delete
// projects, so deleting this resource archives the project.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.name)",message="spec.forProvider.name is a required parameter"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,wiz}
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProjectSpec   `json:"spec"`
	Status            ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Projects
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

// Project type metadata.
var (
	Project_Kind             = "Project"
	Project_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: Project_Kind}.String()
	Project_KindAPIVersion   = Project_Kind + "." + CRDGroupVersion.String()
	Project_GroupVersionKind = CRDGroupVersion.WithKind(Project_Kind)
)

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}

// GetParameters returns the desired Wiz project parameters.
func (mg *Project) GetParameters() *project.Parameters { return &mg.Spec.ForProvider }

// SetObservation records the observed Wiz project state.
func (mg *Project) SetObservation(o project.Observation) { mg.Status.AtProvider = o }
