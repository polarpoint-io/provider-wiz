package config

import (
	"encoding/json"
	"testing"
)

// TestEveryResourceIsConfigured fails when the Terraform provider schema gains
// a resource that has no API group, kind or external name configured here, or
// when a reference points at a resource that is not generated.
func TestEveryResourceIsConfigured(t *testing.T) {
	var s struct {
		ProviderSchemas map[string]struct {
			ResourceSchemas map[string]json.RawMessage `json:"resource_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal([]byte(providerSchema), &s); err != nil {
		t.Fatalf("cannot parse schema.json: %v", err)
	}
	ps, ok := s.ProviderSchemas["registry.terraform.io/jschoombee/wiz"]
	if !ok {
		t.Fatal("schema.json has no registry.terraform.io/jschoombee/wiz provider")
	}
	for name := range ps.ResourceSchemas {
		if _, ok := resourceGroupKinds[name]; !ok {
			t.Errorf("%s has no group and kind in resourceGroupKinds", name)
		}
		if _, ok := ExternalNameConfigs[name]; !ok {
			t.Errorf("%s has no entry in ExternalNameConfigs", name)
		}
	}
	for name, refs := range resourceReferences {
		for field, target := range refs {
			if _, ok := resourceGroupKinds[target]; !ok {
				t.Errorf("%s.%s references %s, which is not generated", name, field, target)
			}
		}
	}
}

func TestProvidersBuild(t *testing.T) {
	if got := len(GetProvider().Resources); got != len(resourceGroupKinds) {
		t.Errorf("cluster provider has %d resources, want %d", got, len(resourceGroupKinds))
	}
	if got := len(GetProviderNamespaced().Resources); got != len(resourceGroupKinds) {
		t.Errorf("namespaced provider has %d resources, want %d", got, len(resourceGroupKinds))
	}
}
