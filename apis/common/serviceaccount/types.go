package serviceaccount

// Type of a Wiz service account.
// +kubebuilder:validation:Enum=THIRD_PARTY;SENSOR;KUBERNETES_ADMISSION_CONTROLLER;BROKER;KUBERNETES_CONNECTOR;FIRST_PARTY
type Type string

// Parameters are the configurable fields of a Wiz service account. They
// follow the Wiz GraphQL CreateServiceAccountInput. Wiz cannot update a
// service account, so every field is immutable: delete and recreate the
// resource to change one. A change to assignedProjects is reported as a sync
// error rather than rejected, because references fill that field in after
// the resource is created.
type Parameters struct {
	// Name of the service account.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`
	// Type of the service account. Defaults to THIRD_PARTY, a GraphQL API
	// client. Use BROKER for the Wiz Helm charts.
	// +optional
	// +kubebuilder:default=THIRD_PARTY
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type *Type `json:"type,omitempty"`
	// Scopes granted to a THIRD_PARTY service account, for example
	// read:projects or admin:projects.
	// +optional
	// +listType=set
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="scopes are immutable"
	Scopes []string `json:"scopes,omitempty"`
	// AssignedProjects are the Wiz IDs of the projects a THIRD_PARTY service
	// account is limited to. Left empty, it is not limited to projects.
	// +optional
	// +listType=set
	AssignedProjects []string `json:"assignedProjects,omitempty"`
}

// Observation is the observed state of a Wiz service account.
type Observation struct {
	// ID is the Wiz internal service account ID, also the external name.
	ID string `json:"id,omitempty"`
	// ClientID is the OAuth client ID. The client secret is written to the
	// connection secret, never to the status.
	ClientID string `json:"clientId,omitempty"`
	// CreatedAt as reported by Wiz.
	CreatedAt string `json:"createdAt,omitempty"`
	// LastRotatedAt is when the client secret was last rotated. If it changes
	// after creation, the secret was rotated outside Crossplane and the
	// connection secret no longer holds a valid one.
	LastRotatedAt string `json:"lastRotatedAt,omitempty"`
}
