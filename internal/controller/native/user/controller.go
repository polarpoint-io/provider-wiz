// Package user is the native controller for Wiz users. It talks to the Wiz
// GraphQL API directly instead of going through the Terraform provider.
package user

import (
	"context"
	"slices"
	"strings"

	"github.com/Khan/genqlient/graphql"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/cluster/iam/v1alpha1"
	"github.com/polarpoint-io/provider-wiz/apis/common/user"
	namespacedv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/namespaced/iam/v1alpha1"
	"github.com/polarpoint-io/provider-wiz/internal/clients/wizapi"
	"github.com/polarpoint-io/provider-wiz/internal/controller/native/base"
)

const (
	errNotUser = "managed resource is not a Wiz User"
	errGet     = "cannot get Wiz user"
	errCreate  = "cannot create Wiz user"
	errUpdate  = "cannot update Wiz user"
	errDelete  = "cannot delete Wiz user"
	errNoUser  = "Wiz returned no user"
)

// userResource is implemented by both the cluster-scoped and the namespaced
// User types.
type userResource interface {
	resource.Managed
	GetParameters() *user.Parameters
	SetObservation(user.Observation)
}

// SetupCluster adds the cluster-scoped User controller.
func SetupCluster(mgr ctrl.Manager, o xpcontroller.Options) error {
	return base.Setup(mgr, o, clusterKind)
}

// SetupNamespaced adds the namespaced User controller.
func SetupNamespaced(mgr ctrl.Manager, o xpcontroller.Options) error {
	return base.Setup(mgr, o, namespacedKind)
}

// SetupGatedCluster adds the cluster-scoped controller once its CRD exists.
func SetupGatedCluster(mgr ctrl.Manager, o xpcontroller.Options) error {
	return base.SetupGated(mgr, o, clusterKind)
}

// SetupGatedNamespaced adds the namespaced controller once its CRD exists.
func SetupGatedNamespaced(mgr ctrl.Manager, o xpcontroller.Options) error {
	return base.SetupGated(mgr, o, namespacedKind)
}

var (
	clusterKind = base.Kind{
		GVK: clusterv1alpha1.User_GroupVersionKind, Obj: &clusterv1alpha1.User{}, List: &clusterv1alpha1.UserList{},
		Connector: newConnector,
	}
	namespacedKind = base.Kind{
		GVK: namespacedv1alpha1.User_GroupVersionKind, Obj: &namespacedv1alpha1.User{}, List: &namespacedv1alpha1.UserList{},
		Connector: newConnector,
	}
)

func newConnector(kube client.Client) managed.ExternalConnector {
	return &connector{kube: kube, newClient: wizapi.Cached}
}

type connector struct {
	kube      client.Client
	newClient func(wizapi.Config) graphql.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(userResource); !ok {
		return nil, errors.New(errNotUser)
	}
	wiz, err := base.Client(ctx, c.kube, mg, c.newClient)
	if err != nil {
		return nil, err
	}
	return &external{wiz: wiz}, nil
}

type external struct {
	wiz graphql.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(userResource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotUser)
	}
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{}, nil
	}
	resp, err := wizapi.GetUser(ctx, e.wiz, id)
	if wizapi.IsNotFound(err) || (err == nil && (resp == nil || resp.User == nil)) {
		return managed.ExternalObservation{}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}
	obs := observation(&resp.User.UserFields)
	cr.SetObservation(obs)
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate(*cr.GetParameters(), &resp.User.UserFields, obs),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(userResource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotUser)
	}
	cr.SetConditions(xpv2.Creating())
	p := cr.GetParameters()
	in := wizapi.CreateUserInput{
		Name:               p.Name,
		Email:              p.Email,
		Role:               p.Role,
		AssignedProjectIds: p.AssignedProjectIDs,
		SendEmailInvite:    p.SendEmailInvite == nil || *p.SendEmailInvite,
	}
	resp, err := wizapi.CreateUser(ctx, e.wiz, in)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}
	if resp == nil || resp.CreateUser == nil || resp.CreateUser.User == nil {
		return managed.ExternalCreation{}, errors.New(errNoUser)
	}
	// A suspended user is created active; Observe then reports the drift and
	// Update suspends it.
	meta.SetExternalName(cr, resp.CreateUser.User.Id)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(userResource)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotUser)
	}
	_, err := wizapi.UpdateUser(ctx, e.wiz, wizapi.UpdateUserInput{Id: meta.GetExternalName(cr), Patch: toPatch(*cr.GetParameters())})
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(userResource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotUser)
	}
	cr.SetConditions(xpv2.Deleting())
	_, err := wizapi.DeleteUser(ctx, e.wiz, wizapi.DeleteUserInput{Id: meta.GetExternalName(cr)})
	if wizapi.IsNotFound(err) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
}

func (e *external) Disconnect(context.Context) error { return nil }

func observation(f *wizapi.UserFields) user.Observation {
	o := user.Observation{ID: f.Id, IsSuspended: f.IsSuspended != nil && *f.IsSuspended}
	if f.EffectiveRole != nil {
		o.EffectiveRole = f.EffectiveRole.Id
	}
	for _, p := range f.EffectiveAssignedProjects {
		o.EffectiveAssignedProjectIDs = append(o.EffectiveAssignedProjectIDs, p.Id)
	}
	return o
}

// upToDate compares the fields this resource manages. Lists left unset in
// the spec are not managed, and email addresses compare case-insensitively.
func upToDate(spec user.Parameters, f *wizapi.UserFields, obs user.Observation) bool {
	switch {
	case f.Name == nil || *f.Name != spec.Name:
		return false
	case f.Email == nil || !strings.EqualFold(*f.Email, spec.Email):
		return false
	case obs.EffectiveRole != spec.Role:
		return false
	case spec.AssignedProjectIDs != nil && !slices.Equal(base.SortedSet(spec.AssignedProjectIDs), base.SortedSet(obs.EffectiveAssignedProjectIDs)):
		return false
	case spec.IsSuspended != nil && *spec.IsSuspended != obs.IsSuspended:
		return false
	}
	return true
}

// toPatch sends every managed field. assignedProjectIds is sent only when the
// spec sets it, so that [] removes every assignment and unset leaves them.
func toPatch(p user.Parameters) wizapi.UserPatch {
	patch := wizapi.UserPatch{
		Name:        base.Ptr(p.Name),
		Email:       base.Ptr(p.Email),
		Role:        base.Ptr(p.Role),
		IsSuspended: p.IsSuspended,
	}
	if p.AssignedProjectIDs != nil {
		ids := append([]string{}, p.AssignedProjectIDs...)
		patch.AssignedProjectIds = &ids
	}
	return patch
}
