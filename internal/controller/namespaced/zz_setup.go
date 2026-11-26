// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/v2/pkg/controller"

	ruleawssns "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/automation/ruleawssns"
	rulejiraaddcomment "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/automation/rulejiraaddcomment"
	rulejiracreateticket "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/automation/rulejiracreateticket"
	rulejiratransitionticket "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/automation/rulejiratransitionticket"
	ruleservicenowcreateticket "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/automation/ruleservicenowcreateticket"
	ruleservicenowupdateticket "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/automation/ruleservicenowupdateticket"
	aws "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/connector/aws"
	gcp "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/connector/gcp"
	samlgroupmapping "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/iam/samlgroupmapping"
	samlidp "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/iam/samlidp"
	serviceaccount "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/iam/serviceaccount"
	user "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/iam/user"
	awssns "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/integration/awssns"
	jira "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/integration/jira"
	servicenow "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/integration/servicenow"
	cicdscanpolicy "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/policy/cicdscanpolicy"
	cloudconfigrule "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/policy/cloudconfigrule"
	cloudconfigruleassociations "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/policy/cloudconfigruleassociations"
	control "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/policy/control"
	controlassociations "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/policy/controlassociations"
	hostconfigruleassociations "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/policy/hostconfigruleassociations"
	securityframework "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/policy/securityframework"
	providerconfig "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/providerconfig"
	graphquery "github.com/polarpoint-io/provider-wiz/internal/controller/namespaced/report/graphquery"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		ruleawssns.Setup,
		rulejiraaddcomment.Setup,
		rulejiracreateticket.Setup,
		rulejiratransitionticket.Setup,
		ruleservicenowcreateticket.Setup,
		ruleservicenowupdateticket.Setup,
		aws.Setup,
		gcp.Setup,
		samlgroupmapping.Setup,
		samlidp.Setup,
		serviceaccount.Setup,
		user.Setup,
		awssns.Setup,
		jira.Setup,
		servicenow.Setup,
		cicdscanpolicy.Setup,
		cloudconfigrule.Setup,
		cloudconfigruleassociations.Setup,
		control.Setup,
		controlassociations.Setup,
		hostconfigruleassociations.Setup,
		securityframework.Setup,
		providerconfig.Setup,
		graphquery.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated creates all controllers with the supplied logger and adds them to
// the supplied manager gated.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		ruleawssns.SetupGated,
		rulejiraaddcomment.SetupGated,
		rulejiracreateticket.SetupGated,
		rulejiratransitionticket.SetupGated,
		ruleservicenowcreateticket.SetupGated,
		ruleservicenowupdateticket.SetupGated,
		aws.SetupGated,
		gcp.SetupGated,
		samlgroupmapping.SetupGated,
		samlidp.SetupGated,
		serviceaccount.SetupGated,
		user.SetupGated,
		awssns.SetupGated,
		jira.SetupGated,
		servicenow.SetupGated,
		cicdscanpolicy.SetupGated,
		cloudconfigrule.SetupGated,
		cloudconfigruleassociations.SetupGated,
		control.SetupGated,
		controlassociations.SetupGated,
		hostconfigruleassociations.SetupGated,
		securityframework.SetupGated,
		providerconfig.SetupGated,
		graphquery.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupWebhookWithManager registers conversion webhooks for all resource kinds in the group.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	for _, setup := range []func(ctrl.Manager) error{
		ruleawssns.SetupWebhookWithManager,
		rulejiraaddcomment.SetupWebhookWithManager,
		rulejiracreateticket.SetupWebhookWithManager,
		rulejiratransitionticket.SetupWebhookWithManager,
		ruleservicenowcreateticket.SetupWebhookWithManager,
		ruleservicenowupdateticket.SetupWebhookWithManager,
		aws.SetupWebhookWithManager,
		gcp.SetupWebhookWithManager,
		samlgroupmapping.SetupWebhookWithManager,
		samlidp.SetupWebhookWithManager,
		serviceaccount.SetupWebhookWithManager,
		user.SetupWebhookWithManager,
		awssns.SetupWebhookWithManager,
		jira.SetupWebhookWithManager,
		servicenow.SetupWebhookWithManager,
		cicdscanpolicy.SetupWebhookWithManager,
		cloudconfigrule.SetupWebhookWithManager,
		cloudconfigruleassociations.SetupWebhookWithManager,
		control.SetupWebhookWithManager,
		controlassociations.SetupWebhookWithManager,
		hostconfigruleassociations.SetupWebhookWithManager,
		securityframework.SetupWebhookWithManager,
		providerconfig.SetupWebhookWithManager,
		graphquery.SetupWebhookWithManager,
	} {
		if err := setup(mgr); err != nil {
			return err
		}
	}
	return nil
}
