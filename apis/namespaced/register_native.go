package namespaced

import (
	v1alpha1project "github.com/polarpoint-io/provider-wiz/apis/namespaced/project/v1alpha1"
)

// Native (non-Terraform) APIs are registered here, outside the Upjet
// generated zz_register.go, so regenerating the Upjet APIs keeps them.
func init() {
	AddToSchemes = append(AddToSchemes, v1alpha1project.SchemeBuilder.AddToScheme)
}
