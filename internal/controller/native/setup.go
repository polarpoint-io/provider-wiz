// Package native wires up the controllers that call the Wiz GraphQL API
// directly. Kinds move here from the Upjet (Terraform) controllers one at a
// time; see docs/native-migration.md.
package native

import (
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/polarpoint-io/provider-wiz/internal/controller/native/project"
	"github.com/polarpoint-io/provider-wiz/internal/controller/native/serviceaccount"
	"github.com/polarpoint-io/provider-wiz/internal/controller/native/user"
)

// Setup adds every native controller to the manager.
func Setup(mgr ctrl.Manager, cluster, namespaced xpcontroller.Options) error {
	for _, s := range []struct {
		fn func(ctrl.Manager, xpcontroller.Options) error
		o  xpcontroller.Options
	}{
		{project.SetupCluster, cluster},
		{project.SetupNamespaced, namespaced},
		{user.SetupCluster, cluster},
		{user.SetupNamespaced, namespaced},
		{serviceaccount.SetupCluster, cluster},
		{serviceaccount.SetupNamespaced, namespaced},
	} {
		if err := s.fn(mgr, s.o); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated adds every native controller once its CRD is installed.
func SetupGated(mgr ctrl.Manager, cluster, namespaced xpcontroller.Options) error {
	for _, s := range []struct {
		fn func(ctrl.Manager, xpcontroller.Options) error
		o  xpcontroller.Options
	}{
		{project.SetupGatedCluster, cluster},
		{project.SetupGatedNamespaced, namespaced},
		{user.SetupGatedCluster, cluster},
		{user.SetupGatedNamespaced, namespaced},
		{serviceaccount.SetupGatedCluster, cluster},
		{serviceaccount.SetupGatedNamespaced, namespaced},
	} {
		if err := s.fn(mgr, s.o); err != nil {
			return err
		}
	}
	return nil
}
