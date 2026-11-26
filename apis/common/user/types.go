package user

// Parameters are the configurable fields of a Wiz user. They follow the Wiz
// GraphQL CreateUserInput. Name, email and role are required unless the
// resource only observes (see the rules on the User kinds).
type Parameters struct {
	// Name of the user.
	// +optional
	Name string `json:"name,omitempty"`
	// Email address of the user.
	// +optional
	Email string `json:"email,omitempty"`
	// Role is the ID of the user's Wiz role, for example GLOBAL_ADMIN,
	// GLOBAL_READER or PROJECT_ADMIN.
	// +optional
	Role string `json:"role,omitempty"`
	// AssignedProjectIDs are the Wiz IDs of the projects a project-scoped
	// role applies to. Left unset, assignments are not managed; set it to []
	// to remove every assignment. It carries no omitempty, so an explicit
	// empty list survives a round trip.
	// +optional
	// +listType=set
	AssignedProjectIDs []string `json:"assignedProjectIds"`
	// SendEmailInvite sends the user an invitation email when the user is
	// created. Defaults to true. It has no effect after creation.
	// +optional
	SendEmailInvite *bool `json:"sendEmailInvite,omitempty"`
	// IsSuspended suspends the user's access to Wiz. Left unset, it is not
	// managed.
	// +optional
	IsSuspended *bool `json:"isSuspended,omitempty"`
}

// Observation is the observed state of a Wiz user.
type Observation struct {
	// ID is the Wiz internal user ID, also the external name.
	ID string `json:"id,omitempty"`
	// EffectiveRole is the role Wiz applies, which can come from a SAML
	// group mapping rather than this resource.
	EffectiveRole string `json:"effectiveRole,omitempty"`
	// EffectiveAssignedProjectIDs are the projects Wiz applies the role to.
	EffectiveAssignedProjectIDs []string `json:"effectiveAssignedProjectIds,omitempty"`
	// IsSuspended as stored by Wiz.
	IsSuspended bool `json:"isSuspended,omitempty"`
}
