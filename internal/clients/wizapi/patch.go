package wizapi

// UserPatch is the patch of an updateUser mutation. It is written by hand,
// not generated, because a nil field must be left out of the request while an
// empty list must be sent: assignedProjectIds: [] removes every assignment.
type UserPatch struct {
	Name               *string   `json:"name,omitempty"`
	Email              *string   `json:"email,omitempty"`
	Role               *string   `json:"role,omitempty"`
	AssignedProjectIds *[]string `json:"assignedProjectIds,omitempty"`
	IsSuspended        *bool     `json:"isSuspended,omitempty"`
}
