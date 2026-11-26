// Package base holds the controller wiring shared by every native kind.
package base

import (
	"context"
	"slices"
	"time"

	"github.com/Khan/genqlient/graphql"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	xpfeature "github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarpoint-io/provider-wiz/internal/clients"
	"github.com/polarpoint-io/provider-wiz/internal/clients/wizapi"
	"github.com/polarpoint-io/provider-wiz/internal/features"
)

const (
	errCredentials = "cannot get Wiz credentials"
	errConfig      = "invalid Wiz credentials"
)

// Kind describes one native managed resource kind.
type Kind struct {
	GVK  schema.GroupVersionKind
	Obj  client.Object
	List resource.ManagedList
	// Connector builds the kind's external client.
	Connector func(kube client.Client) managed.ExternalConnector
}

// Setup adds a managed reconciler for k to the manager.
func Setup(mgr ctrl.Manager, o xpcontroller.Options, k Kind) error {
	name := managed.ControllerName(k.GVK.String())
	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(k.Connector(mgr.GetClient())),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))), //nolint:staticcheck // event.NewAPIRecorder needs the old recorder, as in the Upjet controllers
		managed.WithTimeout(2 * time.Minute),
		// No initializers: the default would set the external name to the
		// Kubernetes name, but it must stay empty until Wiz returns an ID.
		managed.WithInitializers(),
		managed.WithPollInterval(o.PollInterval),
	}
	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}
	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
		if o.MetricOptions.MRStateMetrics != nil {
			rec := statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, k.List, o.MetricOptions.PollStateMetricInterval)
			if err := mgr.Add(rec); err != nil {
				return errors.Wrapf(err, "cannot register MR state metrics recorder for %s", k.GVK.Kind)
			}
		}
	}
	if o.Features.Enabled(xpfeature.EnableAlphaChangeLogs) && o.ChangeLogOptions != nil {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(k.GVK), opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(k.Obj).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// SetupGated adds the reconciler for k once its CRD is installed.
func SetupGated(mgr ctrl.Manager, o xpcontroller.Options, k Kind) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, k); err != nil {
			mgr.GetLogger().Error(err, "unable to setup reconciler", "gvk", k.GVK.String())
		}
	}, k.GVK)
	return nil
}

// Client returns a Wiz API client for the credentials of mg's ProviderConfig.
func Client(ctx context.Context, kube client.Client, mg resource.Managed, newClient func(wizapi.Config) graphql.Client) (graphql.Client, error) {
	creds, err := clients.ExtractCredentials(ctx, kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errCredentials)
	}
	cfg, err := wizapi.ConfigFromCredentials(creds)
	if err != nil {
		return nil, errors.Wrap(err, errConfig)
	}
	return newClient(cfg), nil
}

// SortedSet returns a sorted copy of s, nil when s is empty, for comparing
// lists Wiz treats as sets.
func SortedSet(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := append([]string(nil), s...)
	slices.Sort(out)
	return out
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }
