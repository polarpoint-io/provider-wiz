// Package v1alpha1 contains the native (non-Terraform) Wiz Project API.
// +kubebuilder:object:generate=true
// +groupName=project.wiz.m.polarpoint.io
// +versionName=v1alpha1
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	CRDGroup   = "project.wiz.m.polarpoint.io"
	CRDVersion = "v1alpha1"
)

var (
	// CRDGroupVersion is the API Group Version used to register the objects
	CRDGroupVersion = schema.GroupVersion{Group: CRDGroup, Version: CRDVersion}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: CRDGroupVersion} //nolint:staticcheck // same builder as the generated Upjet APIs

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
