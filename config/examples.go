package config

import (
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// exampleFixes corrects example manifests scraped from the Terraform
// provider's docs where those docs use arguments the provider schema does not
// have. Each fix edits the first example, which is the one written to
// examples-generated.
var exampleFixes = map[string]func(m map[string]any){
	// The schema block is group_mapping, not group_mappings.
	"wiz_saml_group_mapping": func(m map[string]any) {
		if v, ok := m["group_mappings"]; ok {
			m["group_mapping"] = v
			delete(m, "group_mappings")
		}
	},
	// jira_add_issues_report and jira_project_key are not in the schema; the
	// project argument is jira_project.
	"wiz_automation_rule_jira_transition_ticket": func(m map[string]any) {
		delete(m, "jira_add_issues_report")
		if v, ok := m["jira_project_key"]; ok {
			m["jira_project"] = v
			delete(m, "jira_project_key")
		}
	},
}

// configureExamples applies exampleFixes to the scraped resource metadata.
func configureExamples(p *ujconfig.Provider) {
	for name, fix := range exampleFixes {
		p.AddResourceConfigurator(name, func(r *ujconfig.Resource) {
			if r.MetaResource == nil || len(r.MetaResource.Examples) == 0 {
				return
			}
			ex := &r.MetaResource.Examples[0]
			m := map[string]any{}
			if err := json.Unmarshal([]byte(ex.Manifest), &m); err != nil {
				panic(errorf(name, err))
			}
			fix(m)
			b, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				panic(errorf(name, err))
			}
			ex.Manifest = string(b)
			// Replace, not merge into, the parsed manifest.
			var paved fieldpath.Paved
			if err := paved.UnmarshalJSON(b); err != nil {
				panic(errorf(name, err))
			}
			ex.Paved = paved
		})
	}
}

func errorf(name string, err error) string {
	return "cannot fix example manifest for " + name + ": " + err.Error()
}
