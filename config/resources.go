package config

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// Terraform resource names, API groups and fields used more than once.
const (
	tfProject               = "wiz_project"
	tfIntegrationJira       = "wiz_integration_jira"
	tfIntegrationServiceNow = "wiz_integration_servicenow"

	groupAutomation = "automation"
	groupPolicy     = "policy"

	fieldProjectID     = "project_id"
	fieldIntegrationID = "integration_id"
)

// nativeResources are Terraform resources this provider no longer generates
// with Upjet. They are served by native controllers that call the Wiz GraphQL
// API directly (internal/controller/native), or folded into another native
// kind. The value is the native Go type references should point to, relative
// to apis/<scope>/, or "" when the resource has no native kind of its own.
var nativeResources = map[string]string{
	tfProject:                        "project/v1alpha1.Project",
	"wiz_project_cloud_account_link": "", // folded into Project.spec.forProvider.cloudAccountLinks
}

// groupKind is the Crossplane API short group and kind for a Terraform resource.
type groupKind struct {
	group string
	kind  string
}

// resourceGroupKinds maps every Terraform resource to its API group and kind.
var resourceGroupKinds = map[string]groupKind{
	"wiz_service_account":    {"iam", "ServiceAccount"},
	"wiz_user":               {"iam", "User"},
	"wiz_saml_idp":           {"iam", "SAMLIdP"},
	"wiz_saml_group_mapping": {"iam", "SAMLGroupMapping"},

	"wiz_connector_aws": {"connector", "AWS"},
	"wiz_connector_gcp": {"connector", "GCP"},

	"wiz_integration_aws_sns": {"integration", "AWSSNS"},
	tfIntegrationJira:         {"integration", "Jira"},
	tfIntegrationServiceNow:   {"integration", "ServiceNow"},

	"wiz_automation_rule_aws_sns":                  {groupAutomation, "RuleAWSSNS"},
	"wiz_automation_rule_jira_add_comment":         {groupAutomation, "RuleJiraAddComment"},
	"wiz_automation_rule_jira_create_ticket":       {groupAutomation, "RuleJiraCreateTicket"},
	"wiz_automation_rule_jira_transition_ticket":   {groupAutomation, "RuleJiraTransitionTicket"},
	"wiz_automation_rule_servicenow_create_ticket": {groupAutomation, "RuleServiceNowCreateTicket"},
	"wiz_automation_rule_servicenow_update_ticket": {groupAutomation, "RuleServiceNowUpdateTicket"},

	"wiz_cloud_config_rule":              {groupPolicy, "CloudConfigRule"},
	"wiz_cloud_config_rule_associations": {groupPolicy, "CloudConfigRuleAssociations"},
	"wiz_host_config_rule_associations":  {groupPolicy, "HostConfigRuleAssociations"},
	"wiz_control":                        {groupPolicy, "Control"},
	"wiz_control_associations":           {groupPolicy, "ControlAssociations"},
	"wiz_security_framework":             {groupPolicy, "SecurityFramework"},
	"wiz_cicd_scan_policy":               {groupPolicy, "CICDScanPolicy"},

	"wiz_report_graph_query": {"report", "GraphQuery"},
}

// resourceReferences maps a Terraform resource to the fields that hold the ID
// of another Wiz resource managed by this provider.
var resourceReferences = map[string]map[string]string{
	"wiz_service_account":    {"assigned_projects": tfProject},
	"wiz_user":               {"assigned_project_ids": tfProject},
	"wiz_saml_group_mapping": {"saml_idp_id": "wiz_saml_idp"},

	"wiz_integration_aws_sns": {fieldProjectID: tfProject, "aws_sns_connector_id": "wiz_connector_aws"},
	tfIntegrationJira:         {fieldProjectID: tfProject},
	tfIntegrationServiceNow:   {fieldProjectID: tfProject},

	"wiz_automation_rule_aws_sns":                  {fieldProjectID: tfProject, fieldIntegrationID: "wiz_integration_aws_sns"},
	"wiz_automation_rule_jira_add_comment":         {fieldProjectID: tfProject, fieldIntegrationID: tfIntegrationJira},
	"wiz_automation_rule_jira_create_ticket":       {fieldProjectID: tfProject, fieldIntegrationID: tfIntegrationJira},
	"wiz_automation_rule_jira_transition_ticket":   {fieldProjectID: tfProject, fieldIntegrationID: tfIntegrationJira},
	"wiz_automation_rule_servicenow_create_ticket": {fieldProjectID: tfProject, fieldIntegrationID: tfIntegrationServiceNow},
	"wiz_automation_rule_servicenow_update_ticket": {fieldProjectID: tfProject, fieldIntegrationID: tfIntegrationServiceNow},

	"wiz_cloud_config_rule_associations": {"cloud_config_rule_ids": "wiz_cloud_config_rule"},
	"wiz_control":                        {fieldProjectID: tfProject},
	"wiz_control_associations":           {"control_ids": "wiz_control"},
	"wiz_cicd_scan_policy":               {"project_ids": tfProject},

	"wiz_report_graph_query": {fieldProjectID: tfProject},
}

// reference builds the reference to a Terraform resource. References to a
// resource that is now native point at the native Go type, since Upjet cannot
// resolve a Terraform name it does not generate.
func reference(scope, target string) ujconfig.Reference {
	if t, ok := nativeResources[target]; ok && t != "" {
		//nolint:staticcheck // Type is the only way to reference a non-Upjet kind.
		return ujconfig.Reference{Type: modulePath + "/apis/" + scope + "/" + t}
	}
	return ujconfig.Reference{TerraformName: target}
}

// configureResources registers the group, kind and reference configuration
// for every Wiz resource. The same configuration serves both the cluster
// scoped and the namespaced provider, because references are declared by
// Terraform resource name.
func configureResources(p *ujconfig.Provider, scope string) {
	for name, gk := range resourceGroupKinds {
		refs := resourceReferences[name]
		p.AddResourceConfigurator(name, func(r *ujconfig.Resource) {
			r.ShortGroup = gk.group
			r.Kind = gk.kind
			for field, target := range refs {
				r.References[field] = reference(scope, target)
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
