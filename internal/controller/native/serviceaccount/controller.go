// Package serviceaccount is the native controller for Wiz service accounts.
// It talks to the Wiz GraphQL API directly instead of going through the
// Terraform provider.
package serviceaccount

import (
	"context"
	"slices"

	"github.com/Khan/genqlient/graphql"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/cluster/iam/v1alpha1"
	"github.com/polarpoint-io/provider-wiz/apis/common/serviceaccount"
	namespacedv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/namespaced/iam/v1alpha1"
	"github.com/polarpoint-io/provider-wiz/internal/clients/wizapi"
	"github.com/polarpoint-io/provider-wiz/internal/controller/native/base"
)

// Connection secret keys. With the ProviderConfig's wiz_url, they are
// enough to authenticate as this service account.
const (
	ConnClientID     = "client_id"
	ConnClientSecret = "client_secret"
)

const (
	errNotServiceAccount = "managed resource is not a Wiz ServiceAccount"
	errGet               = "cannot get Wiz service account"
	errCreate            = "cannot create Wiz service account"
	errDelete            = "cannot delete Wiz service account"
	errNoServiceAccount  = "Wiz returned no service account"
	errImmutable         = "Wiz cannot update a service account: the spec no longer matches it, so delete and recreate this resource to apply the change"
	errGetSecret         = "cannot get connection secret"
	errLostSecret        = "the connection secret has no client secret, and Wiz returns it only when the service account is created; delete and recreate this resource to get a new one"
)

// legacyConnClientSecret is where the Upjet controller wrote the secret.
const legacyConnClientSecret = "attribute.client_secret"

// serviceAccountResource is implemented by both the cluster-scoped and the
// namespaced ServiceAccount types.
type serviceAccountResource interface {
	resource.Managed
	GetParameters() *serviceaccount.Parameters
	SetObservation(serviceaccount.Observation)
}

// SetupCluster adds the cluster-scoped ServiceAccount controller.
func SetupCluster(mgr ctrl.Manager, o xpcontroller.Options) error {
	return base.Setup(mgr, o, clusterKind)
}

// SetupNamespaced adds the namespaced ServiceAccount controller.
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
		GVK: clusterv1alpha1.ServiceAccount_GroupVersionKind, Obj: &clusterv1alpha1.ServiceAccount{}, List: &clusterv1alpha1.ServiceAccountList{},
		Connector: newConnector,
	}
	namespacedKind = base.Kind{
		GVK: namespacedv1alpha1.ServiceAccount_GroupVersionKind, Obj: &namespacedv1alpha1.ServiceAccount{}, List: &namespacedv1alpha1.ServiceAccountList{},
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
	if _, ok := mg.(serviceAccountResource); !ok {
		return nil, errors.New(errNotServiceAccount)
	}
	wiz, err := base.Client(ctx, c.kube, mg, c.newClient)
	if err != nil {
		return nil, err
	}
	return &external{wiz: wiz, kube: c.kube}, nil
}

type external struct {
	wiz  graphql.Client
	kube client.Reader
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(serviceAccountResource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotServiceAccount)
	}
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{}, nil
	}
	resp, err := wizapi.GetServiceAccount(ctx, e.wiz, id)
	if wizapi.IsNotFound(err) || (err == nil && (resp == nil || resp.ServiceAccount == nil)) {
		return managed.ExternalObservation{}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}
	f := &resp.ServiceAccount.ServiceAccountFields
	cr.SetObservation(observation(f))
	// If publishing the secret failed after Create, the next reconcile finds
	// the account and nothing would create it again. Say so instead of
	// reporting a ready resource with unusable credentials.
	if lost, err := e.secretLost(ctx, cr); err != nil || lost {
		if err == nil {
			err = errors.New(errLostSecret)
		}
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, err
	}
	cr.SetConditions(xpv2.Available())
	// No connection details here: Wiz returns the client secret only on
	// create, and the secret keeps what Create published.
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate(*cr.GetParameters(), f),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(serviceAccountResource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotServiceAccount)
	}
	cr.SetConditions(xpv2.Creating())
	resp, err := wizapi.CreateServiceAccount(ctx, e.wiz, toCreateInput(*cr.GetParameters()))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}
	if resp == nil || resp.CreateServiceAccount == nil || resp.CreateServiceAccount.ServiceAccount == nil {
		return managed.ExternalCreation{}, errors.New(errNoServiceAccount)
	}
	sa := resp.CreateServiceAccount.ServiceAccount
	meta.SetExternalName(cr, sa.Id)
	cr.SetObservation(observation(&sa.ServiceAccountFields))
	conn := managed.ConnectionDetails{}
	if sa.ClientId != nil {
		conn[ConnClientID] = []byte(*sa.ClientId)
	}
	if sa.ClientSecret != nil {
		conn[ConnClientSecret] = []byte(*sa.ClientSecret)
	}
	return managed.ExternalCreation{ConnectionDetails: conn}, nil
}

// Update is only called when the spec has drifted from Wiz, which a service
// account cannot be updated to match.
func (e *external) Update(context.Context, resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, errors.New(errImmutable)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(serviceAccountResource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotServiceAccount)
	}
	cr.SetConditions(xpv2.Deleting())
	_, err := wizapi.DeleteServiceAccount(ctx, e.wiz, wizapi.DeleteServiceAccountInput{Id: meta.GetExternalName(cr)})
	if wizapi.IsNotFound(err) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
}

func (e *external) Disconnect(context.Context) error { return nil }

// secretLost reports whether this resource asks for a connection secret,
// may have created the account, and the secret holds no client secret.
// Resources that only observe an account never get its secret.
func (e *external) secretLost(ctx context.Context, cr serviceAccountResource) (bool, error) {
	if e.kube == nil || !mayCreate(cr.GetManagementPolicies()) {
		return false, nil
	}
	var key types.NamespacedName
	switch o := cr.(type) {
	case interface{ GetWriteConnectionSecretToReference() *xpv2.SecretReference }:
		ref := o.GetWriteConnectionSecretToReference()
		if ref == nil {
			return false, nil
		}
		key = types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	case interface {
		GetWriteConnectionSecretToReference() *xpv2.LocalSecretReference
	}:
		ref := o.GetWriteConnectionSecretToReference()
		if ref == nil {
			return false, nil
		}
		key = types.NamespacedName{Namespace: cr.GetNamespace(), Name: ref.Name}
	default:
		return false, nil
	}
	s := &corev1.Secret{}
	if err := e.kube.Get(ctx, key, s); err != nil {
		if kerrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.Wrap(err, errGetSecret)
	}
	return len(s.Data[ConnClientSecret]) == 0 && len(s.Data[legacyConnClientSecret]) == 0, nil
}

func mayCreate(p xpv2.ManagementPolicies) bool {
	return len(p) == 0 || slices.Contains(p, xpv2.ManagementActionAll) || slices.Contains(p, xpv2.ManagementActionCreate)
}

func observation(f *wizapi.ServiceAccountFields) serviceaccount.Observation {
	return serviceaccount.Observation{
		ID:            f.Id,
		ClientID:      deref(f.ClientId),
		CreatedAt:     deref(f.CreatedAt),
		LastRotatedAt: deref(f.LastRotatedAt),
	}
}

// upToDate compares the spec with Wiz. Scopes and projects only apply to,
// and are only sent for, THIRD_PARTY service accounts, as in the Terraform
// provider; Wiz may report its own scopes for the other types.
func upToDate(spec serviceaccount.Parameters, f *wizapi.ServiceAccountFields) bool {
	t := typeOf(spec)
	if f.Name != spec.Name || (f.Type != nil && *f.Type != t) {
		return false
	}
	if t != wizapi.ServiceAccountTypeThirdParty {
		return true
	}
	projects := make([]string, 0, len(f.AssignedProjects))
	for _, p := range f.AssignedProjects {
		projects = append(projects, p.Id)
	}
	return slices.Equal(base.SortedSet(f.Scopes), base.SortedSet(spec.Scopes)) &&
		slices.Equal(base.SortedSet(projects), base.SortedSet(spec.AssignedProjects))
}

func toCreateInput(p serviceaccount.Parameters) wizapi.CreateServiceAccountInput {
	t := typeOf(p)
	in := wizapi.CreateServiceAccountInput{Name: p.Name, Type: &t}
	if t == wizapi.ServiceAccountTypeThirdParty {
		in.Scopes = p.Scopes
		in.AssignedProjectIds = p.AssignedProjects
	}
	return in
}

// typeOf returns the spec's type, THIRD_PARTY when unset.
func typeOf(p serviceaccount.Parameters) wizapi.ServiceAccountType {
	if p.Type == nil {
		return wizapi.ServiceAccountTypeThirdParty
	}
	return wizapi.ServiceAccountType(*p.Type)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
