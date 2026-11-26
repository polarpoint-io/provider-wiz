// Package project is the native controller for Wiz projects. It talks to the
// Wiz GraphQL API directly instead of going through the Terraform provider.
package project

import (
	"context"
	"time"

	"github.com/Khan/genqlient/graphql"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	xpfeature "github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/cluster/project/v1alpha1"
	"github.com/polarpoint-io/provider-wiz/apis/common/project"
	namespacedv1alpha1 "github.com/polarpoint-io/provider-wiz/apis/namespaced/project/v1alpha1"
	"github.com/polarpoint-io/provider-wiz/internal/clients"
	"github.com/polarpoint-io/provider-wiz/internal/clients/wizapi"
	"github.com/polarpoint-io/provider-wiz/internal/features"
)

const (
	errNotProject  = "managed resource is not a Wiz Project"
	errCredentials = "cannot get Wiz credentials"
	errConfig      = "invalid Wiz credentials"
	errGet         = "cannot get Wiz project"
	errCreate      = "cannot create Wiz project"
	errUpdate      = "cannot update Wiz project"
	errArchive     = "cannot archive Wiz project"
	errNoProject   = "Wiz returned no project"
)

// projectResource is implemented by both the cluster-scoped and the
// namespaced Project types.
type projectResource interface {
	resource.Managed
	GetParameters() *project.Parameters
	SetObservation(project.Observation)
}

// SetupCluster adds the cluster-scoped Project controller.
func SetupCluster(mgr ctrl.Manager, o xpcontroller.Options) error {
	return setup(mgr, o, clusterv1alpha1.Project_GroupVersionKind, &clusterv1alpha1.Project{}, &clusterv1alpha1.ProjectList{})
}

// SetupNamespaced adds the namespaced Project controller.
func SetupNamespaced(mgr ctrl.Manager, o xpcontroller.Options) error {
	return setup(mgr, o, namespacedv1alpha1.Project_GroupVersionKind, &namespacedv1alpha1.Project{}, &namespacedv1alpha1.ProjectList{})
}

// SetupGatedCluster adds the cluster-scoped controller once its CRD exists.
func SetupGatedCluster(mgr ctrl.Manager, o xpcontroller.Options) error {
	return gated(mgr, o, clusterv1alpha1.Project_GroupVersionKind, SetupCluster)
}

// SetupGatedNamespaced adds the namespaced controller once its CRD exists.
func SetupGatedNamespaced(mgr ctrl.Manager, o xpcontroller.Options) error {
	return gated(mgr, o, namespacedv1alpha1.Project_GroupVersionKind, SetupNamespaced)
}

func gated(mgr ctrl.Manager, o xpcontroller.Options, gvk schema.GroupVersionKind, setup func(ctrl.Manager, xpcontroller.Options) error) error {
	o.Gate.Register(func() {
		if err := setup(mgr, o); err != nil {
			mgr.GetLogger().Error(err, "unable to setup reconciler", "gvk", gvk.String())
		}
	}, gvk)
	return nil
}

func setup(mgr ctrl.Manager, o xpcontroller.Options, gvk schema.GroupVersionKind, obj client.Object, list resource.ManagedList) error {
	name := managed.ControllerName(gvk.String())
	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{kube: mgr.GetClient(), newClient: wizapi.Cached}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithTimeout(2 * time.Minute),
		managed.WithPollInterval(o.PollInterval),
	}
	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}
	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
		if o.MetricOptions.MRStateMetrics != nil {
			rec := statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, list, o.MetricOptions.PollStateMetricInterval)
			if err := mgr.Add(rec); err != nil {
				return errors.Wrapf(err, "cannot register MR state metrics recorder for %s", gvk.Kind)
			}
		}
	}
	if o.Features.Enabled(xpfeature.EnableAlphaChangeLogs) && o.ChangeLogOptions != nil {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(gvk), opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(obj).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube      client.Client
	newClient func(wizapi.Config) graphql.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(projectResource); !ok {
		return nil, errors.New(errNotProject)
	}
	creds, err := clients.ExtractCredentials(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errCredentials)
	}
	cfg, err := wizapi.ConfigFromCredentials(creds)
	if err != nil {
		return nil, errors.Wrap(err, errConfig)
	}
	return &external{wiz: c.newClient(cfg)}, nil
}

type external struct {
	wiz graphql.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(projectResource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotProject)
	}
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{}, nil
	}
	resp, err := wizapi.GetProject(ctx, e.wiz, id)
	if wizapi.IsNotFound(err) || (err == nil && (resp == nil || resp.Project == nil)) {
		return managed.ExternalObservation{}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}
	f := &resp.Project.ProjectFields

	// Wiz cannot delete projects; an archived project counts as deleted
	// once this resource is being deleted.
	if meta.WasDeleted(cr) && f.Archived {
		return managed.ExternalObservation{}, nil
	}

	cr.SetObservation(observation(f))
	cr.SetConditions(xpv2.Available())

	obs := fromObserved(f)
	spec := cr.GetParameters()
	lateInit := lateInitialize(spec, obs)
	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        upToDate(*spec, obs),
		ResourceLateInitialized: lateInit,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(projectResource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotProject)
	}
	cr.SetConditions(xpv2.Creating())
	resp, err := wizapi.CreateProject(ctx, e.wiz, toCreateInput(*cr.GetParameters()))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}
	if resp == nil || resp.CreateProject == nil || resp.CreateProject.Project == nil {
		return managed.ExternalCreation{}, errors.New(errNoProject)
	}
	meta.SetExternalName(cr, resp.CreateProject.Project.Id)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(projectResource)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotProject)
	}
	id := meta.GetExternalName(cr)
	cur, err := wizapi.GetProject(ctx, e.wiz, id)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGet)
	}
	if cur == nil || cur.Project == nil {
		return managed.ExternalUpdate{}, errors.New(errNoProject)
	}
	// Send the whole project as an override: the observed state with every
	// field this resource sets laid over it. Lists then match the spec
	// exactly, and fields the spec leaves unset are left as they are.
	desired := merged(*cr.GetParameters(), fromObserved(&cur.Project.ProjectFields))
	_, err = wizapi.UpdateProject(ctx, e.wiz, wizapi.UpdateProjectInput{Id: id, Override: toPatch(desired)})
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(projectResource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotProject)
	}
	cr.SetConditions(xpv2.Deleting())
	// Wiz has no deleteProject mutation; archiving is the closest thing.
	_, err := wizapi.UpdateProject(ctx, e.wiz, wizapi.UpdateProjectInput{
		Id:    meta.GetExternalName(cr),
		Patch: &wizapi.UpdateProjectPatch{Archived: ptr(true)},
	})
	return managed.ExternalDelete{}, errors.Wrap(err, errArchive)
}

func (e *external) Disconnect(context.Context) error { return nil }
