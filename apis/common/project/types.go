package project

// YesNoUnknown is a tri-state answer used by a project's risk profile.
// +kubebuilder:validation:Enum=YES;NO;UNKNOWN
type YesNoUnknown string

// Environment of the resources linked to a project.
// +kubebuilder:validation:Enum=PRODUCTION;STAGING;DEVELOPMENT;TESTING;OTHER
type Environment string

// RiskProfile describes the business context of a project.
type RiskProfile struct {
	// BusinessImpact of the project: LBI (low), MBI (medium) or HBI (high).
	// +kubebuilder:validation:Enum=LBI;MBI;HBI
	// +optional
	BusinessImpact *string `json:"businessImpact,omitempty"`
	// +optional
	IsActivelyDeveloped *YesNoUnknown `json:"isActivelyDeveloped,omitempty"`
	// +optional
	HasAuthentication *YesNoUnknown `json:"hasAuthentication,omitempty"`
	// +optional
	HasExposedAPI *YesNoUnknown `json:"hasExposedAPI,omitempty"`
	// +optional
	IsInternetFacing *YesNoUnknown `json:"isInternetFacing,omitempty"`
	// +optional
	IsCustomerFacing *YesNoUnknown `json:"isCustomerFacing,omitempty"`
	// +optional
	StoresData *YesNoUnknown `json:"storesData,omitempty"`
	// SensitiveDataTypes the project stores, for example PII or PCI.
	// +optional
	// +listType=set
	SensitiveDataTypes []string `json:"sensitiveDataTypes,omitempty"`
	// +optional
	IsRegulated *YesNoUnknown `json:"isRegulated,omitempty"`
	// RegulatoryStandards that apply, for example ISO_27001 or SOC.
	// +optional
	// +listType=set
	RegulatoryStandards []string `json:"regulatoryStandards,omitempty"`
}

// ResourceTag is a cloud resource tag used to scope a link.
type ResourceTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CloudAccountLink scopes a cloud account (subscription) into the project.
// It replaces the separate CloudAccountLink kind of the Terraform-based API.
type CloudAccountLink struct {
	// CloudAccount is the Wiz internal ID of the cloud account.
	CloudAccount string      `json:"cloudAccount"`
	Environment  Environment `json:"environment"`
	// ResourceGroups limits the link to these resource groups.
	// +optional
	ResourceGroups []string `json:"resourceGroups,omitempty"`
	// ResourceTags limits the link to resources carrying these tags.
	// +optional
	ResourceTags []ResourceTag `json:"resourceTags,omitempty"`
	// Shared marks the account as shared with other projects.
	// +optional
	Shared *bool `json:"shared,omitempty"`
}

// CloudOrganizationLink scopes a cloud organization into the project.
type CloudOrganizationLink struct {
	// CloudOrganization is the Wiz internal ID of the cloud organization.
	CloudOrganization string      `json:"cloudOrganization"`
	Environment       Environment `json:"environment"`
	// +optional
	ResourceGroups []string `json:"resourceGroups,omitempty"`
	// +optional
	ResourceTags []ResourceTag `json:"resourceTags,omitempty"`
	// +optional
	Shared *bool `json:"shared,omitempty"`
}

// KubernetesClusterLink scopes a Kubernetes cluster into the project.
type KubernetesClusterLink struct {
	// KubernetesCluster is the Wiz internal ID of the cluster.
	KubernetesCluster string      `json:"kubernetesCluster"`
	Environment       Environment `json:"environment"`
	// Namespaces limits the link to these namespaces.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
	// +optional
	Shared *bool `json:"shared,omitempty"`
}

// Parameters are the configurable fields of a Wiz project. They follow the
// Wiz GraphQL CreateProjectInput.
//
// List fields left unset are not managed; set one to [] to clear it in Wiz.
// They carry no omitempty, so an explicit empty list survives a round trip.
//
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.slug) || has(self.slug)",message="slug cannot be removed once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.isFolder) || has(self.isFolder)",message="isFolder cannot be removed once set"
type Parameters struct {
	// Name of the project. Required unless the resource only observes (see
	// the rules on the Project kinds).
	// +optional
	Name string `json:"name,omitempty"`
	// Slug is a short identifier, unique across all projects including
	// archived ones. Wiz generates one when unset. Immutable once set.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="slug is immutable"
	Slug *string `json:"slug,omitempty"`
	// +optional
	Description *string `json:"description,omitempty"`
	// IsFolder makes this a folder project that holds other projects.
	// Immutable.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="isFolder is immutable"
	IsFolder *bool `json:"isFolder,omitempty"`
	// Archived projects are inactive. Wiz cannot delete projects, so deleting
	// this resource archives the project.
	// +optional
	Archived *bool `json:"archived,omitempty"`
	// +optional
	BusinessUnit *string `json:"businessUnit,omitempty"`
	// Identifiers are free-form external identifiers for the project.
	// +optional
	Identifiers []string `json:"identifiers"`
	// ParentProjectID is the Wiz ID of the folder project this project sits in.
	// +optional
	ParentProjectID *string `json:"parentProjectId,omitempty"`
	// ProjectOwners are Wiz user IDs.
	// +optional
	// +listType=set
	ProjectOwners []string `json:"projectOwners"`
	// SecurityChampions are Wiz user IDs.
	// +optional
	// +listType=set
	SecurityChampions []string `json:"securityChampions"`
	// +optional
	RiskProfile RiskProfile `json:"riskProfile,omitempty"`
	// +optional
	CloudAccountLinks []CloudAccountLink `json:"cloudAccountLinks"`
	// +optional
	CloudOrganizationLinks []CloudOrganizationLink `json:"cloudOrganizationLinks"`
	// +optional
	KubernetesClusterLinks []KubernetesClusterLink `json:"kubernetesClusterLinks"`
}

// Observation is the observed state of a Wiz project.
type Observation struct {
	// ID is the Wiz internal project ID, also the external name.
	ID string `json:"id,omitempty"`
	// Slug as stored by Wiz.
	Slug string `json:"slug,omitempty"`
	// Archived as stored by Wiz.
	Archived bool `json:"archived,omitempty"`
	// AncestorProjectIDs lists the folder projects above this one.
	AncestorProjectIDs []string `json:"ancestorProjectIds,omitempty"`
}
