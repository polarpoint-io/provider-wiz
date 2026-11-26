package config

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// groupKind is the Crossplane API short group and kind for a Terraform resource.
type groupKind struct {
	group string
	kind  string
}

// resourceGroupKinds maps every Terraform resource to its API group and kind.
var resourceGroupKinds = map[string]groupKind{
	"wiz_project":                    {"project", "Project"},
	"wiz_project_cloud_account_link": {"project", "CloudAccountLink"},

	"wiz_service_account":    {"iam", "ServiceAccount"},
	"wiz_user":               {"iam", "User"},
	"wiz_saml_idp":           {"iam", "SAMLIdP"},
	"wiz_saml_group_mapping": {"iam", "SAMLGroupMapping"},

	"wiz_connector_aws": {"connector", "AWS"},
	"wiz_connector_gcp": {"connector", "GCP"},

	"wiz_integration_aws_sns":    {"integration", "AWSSNS"},
	"wiz_integration_jira":       {"integration", "Jira"},
	"wiz_integration_servicenow": {"integration", "ServiceNow"},

	"wiz_automation_rule_aws_sns":                  {"automation", "RuleAWSSNS"},
	"wiz_automation_rule_jira_add_comment":         {"automation", "RuleJiraAddComment"},
	"wiz_automation_rule_jira_create_ticket":       {"automation", "RuleJiraCreateTicket"},
	"wiz_automation_rule_jira_transition_ticket":   {"automation", "RuleJiraTransitionTicket"},
	"wiz_automation_rule_servicenow_create_ticket": {"automation", "RuleServiceNowCreateTicket"},
	"wiz_automation_rule_servicenow_update_ticket": {"automation", "RuleServiceNowUpdateTicket"},

	"wiz_cloud_config_rule":              {"policy", "CloudConfigRule"},
	"wiz_cloud_config_rule_associations": {"policy", "CloudConfigRuleAssociations"},
	"wiz_host_config_rule_associations":  {"policy", "HostConfigRuleAssociations"},
	"wiz_control":                        {"policy", "Control"},
	"wiz_control_associations":           {"policy", "ControlAssociations"},
	"wiz_security_framework":             {"policy", "SecurityFramework"},
	"wiz_cicd_scan_policy":               {"policy", "CICDScanPolicy"},

	"wiz_report_graph_query": {"report", "GraphQuery"},
}

// resourceReferences maps a Terraform resource to the fields that hold the ID
// of another Wiz resource managed by this provider.
var resourceReferences = map[string]map[string]string{
	"wiz_project":                    {"parent_project_id": "wiz_project"},
	"wiz_project_cloud_account_link": {"project_id": "wiz_project"},
	"wiz_service_account":            {"assigned_projects": "wiz_project"},
	"wiz_user":                       {"assigned_project_ids": "wiz_project"},
	"wiz_saml_group_mapping":         {"saml_idp_id": "wiz_saml_idp"},

	"wiz_integration_aws_sns":    {"project_id": "wiz_project", "aws_sns_connector_id": "wiz_connector_aws"},
	"wiz_integration_jira":       {"project_id": "wiz_project"},
	"wiz_integration_servicenow": {"project_id": "wiz_project"},

	"wiz_automation_rule_aws_sns":                  {"project_id": "wiz_project", "integration_id": "wiz_integration_aws_sns"},
	"wiz_automation_rule_jira_add_comment":         {"project_id": "wiz_project", "integration_id": "wiz_integration_jira"},
	"wiz_automation_rule_jira_create_ticket":       {"project_id": "wiz_project", "integration_id": "wiz_integration_jira"},
	"wiz_automation_rule_jira_transition_ticket":   {"project_id": "wiz_project", "integration_id": "wiz_integration_jira"},
	"wiz_automation_rule_servicenow_create_ticket": {"project_id": "wiz_project", "integration_id": "wiz_integration_servicenow"},
	"wiz_automation_rule_servicenow_update_ticket": {"project_id": "wiz_project", "integration_id": "wiz_integration_servicenow"},

	"wiz_cloud_config_rule_associations": {"cloud_config_rule_ids": "wiz_cloud_config_rule"},
	"wiz_control":                        {"project_id": "wiz_project"},
	"wiz_control_associations":           {"control_ids": "wiz_control"},
	"wiz_cicd_scan_policy":               {"project_ids": "wiz_project"},

	"wiz_report_graph_query": {"project_id": "wiz_project"},
}

// configureResources registers the group, kind and reference configuration
// for every Wiz resource. The same configuration serves both the cluster
// scoped and the namespaced provider, because references are declared by
// Terraform resource name.
func configureResources(p *ujconfig.Provider) {
	for name, gk := range resourceGroupKinds {
		refs := resourceReferences[name]
		p.AddResourceConfigurator(name, func(r *ujconfig.Resource) {
			r.ShortGroup = gk.group
			r.Kind = gk.kind
			for field, target := range refs {
				r.References[field] = ujconfig.Reference{TerraformName: target}
			}
		})
	}

	// The service account's client credentials are returned as computed,
	// non-sensitive attributes. Mark the secret sensitive so it is published
	// to the connection secret instead of status.atProvider, and publish the
	// client ID next to it so the secret is enough to build a ProviderConfig.
	p.AddResourceConfigurator("wiz_service_account", func(r *ujconfig.Resource) {
		if s, ok := r.TerraformResource.Schema["client_secret"]; ok {
			s.Sensitive = true
		}
		r.Sensitive.AdditionalConnectionDetailsFn = func(attr map[string]any) (map[string][]byte, error) {
			conn := map[string][]byte{}
			if v, ok := attr["client_id"].(string); ok && v != "" {
				conn["client_id"] = []byte(v)
			}
			return conn, nil
		}
	})
}
