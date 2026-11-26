package project

import (
	"reflect"
	"slices"
	"sort"

	"github.com/polarpoint-io/provider-wiz/apis/common/project"
	"github.com/polarpoint-io/provider-wiz/internal/clients/wizapi"
)

// fromObserved converts a Wiz project into the parameters that would produce
// it, so it can be compared with, and merged into, the desired state.
func fromObserved(f *wizapi.ProjectFields) project.Parameters {
	p := project.Parameters{
		Name:         f.Name,
		Slug:         ptr(f.Slug),
		Description:  f.Description,
		IsFolder:     ptr(f.IsFolder),
		Archived:     ptr(f.Archived),
		BusinessUnit: f.BusinessUnit,
		Identifiers:  f.Identifiers,
	}
	// Wiz lists the direct parent first among the ancestors.
	if len(f.AncestorProjects) > 0 {
		p.ParentProjectID = ptr(f.AncestorProjects[0].Id)
	}
	for _, u := range f.ProjectOwners {
		p.ProjectOwners = append(p.ProjectOwners, u.Id)
	}
	for _, u := range f.SecurityChampions {
		p.SecurityChampions = append(p.SecurityChampions, u.Id)
	}
	r := f.RiskProfile
	p.RiskProfile = project.RiskProfile{
		BusinessImpact:      (*string)(r.BusinessImpact),
		IsActivelyDeveloped: (*project.YesNoUnknown)(r.IsActivelyDeveloped),
		HasAuthentication:   (*project.YesNoUnknown)(r.HasAuthentication),
		HasExposedAPI:       (*project.YesNoUnknown)(r.HasExposedAPI),
		IsInternetFacing:    (*project.YesNoUnknown)(r.IsInternetFacing),
		IsCustomerFacing:    (*project.YesNoUnknown)(r.IsCustomerFacing),
		StoresData:          (*project.YesNoUnknown)(r.StoresData),
		IsRegulated:         (*project.YesNoUnknown)(r.IsRegulated),
		SensitiveDataTypes:  stringsOf(r.SensitiveDataTypes),
		RegulatoryStandards: stringsOf(r.RegulatoryStandards),
	}
	p.CloudAccountLinks, p.CloudOrganizationLinks, p.KubernetesClusterLinks = observedLinks(f)
	return p
}

func observedLinks(f *wizapi.ProjectFields) ([]project.CloudAccountLink, []project.CloudOrganizationLink, []project.KubernetesClusterLink) {
	ca := make([]project.CloudAccountLink, 0, len(f.CloudAccountLinks))
	for _, l := range f.CloudAccountLinks {
		cl := project.CloudAccountLink{CloudAccount: l.CloudAccount.Id, ResourceGroups: l.ResourceGroups, Shared: l.Shared, Environment: env(l.Environment)}
		for _, t := range l.ResourceTags {
			cl.ResourceTags = append(cl.ResourceTags, project.ResourceTag{Key: t.Key, Value: t.Value})
		}
		ca = append(ca, cl)
	}
	co := make([]project.CloudOrganizationLink, 0, len(f.CloudOrganizationLinks))
	for _, l := range f.CloudOrganizationLinks {
		ol := project.CloudOrganizationLink{CloudOrganization: l.CloudOrganization.Id, ResourceGroups: l.ResourceGroups, Shared: l.Shared, Environment: env(l.Environment)}
		for _, t := range l.ResourceTags {
			ol.ResourceTags = append(ol.ResourceTags, project.ResourceTag{Key: t.Key, Value: t.Value})
		}
		co = append(co, ol)
	}
	kc := make([]project.KubernetesClusterLink, 0, len(f.KubernetesClustersLinks))
	for _, l := range f.KubernetesClustersLinks {
		kc = append(kc, project.KubernetesClusterLink{KubernetesCluster: l.KubernetesCluster.Id, Namespaces: l.Namespaces, Shared: l.Shared, Environment: env(l.Environment)})
	}
	return ca, co, kc
}

func env(e *wizapi.Environment) project.Environment {
	if e == nil {
		return ""
	}
	return project.Environment(*e)
}

func observation(f *wizapi.ProjectFields) project.Observation {
	o := project.Observation{ID: f.Id, Slug: f.Slug, Archived: f.Archived}
	for _, a := range f.AncestorProjects {
		o.AncestorProjectIDs = append(o.AncestorProjectIDs, a.Id)
	}
	return o
}

// lateInitialize fills unset desired fields that Wiz defaults on create. It
// reports whether anything changed.
func lateInitialize(spec *project.Parameters, obs project.Parameters) bool {
	changed := false
	if spec.Slug == nil && obs.Slug != nil {
		spec.Slug = obs.Slug
		changed = true
	}
	if spec.IsFolder == nil && obs.IsFolder != nil {
		spec.IsFolder = obs.IsFolder
		changed = true
	}
	return changed
}

// merged overlays every field set in spec onto the observed state. Fields the
// spec leaves unset keep their current Wiz value, so an update never clears
// something this resource does not manage.
func merged(spec, obs project.Parameters) project.Parameters {
	m := obs
	m.Name = spec.Name
	overlay(&m.Slug, spec.Slug)
	overlay(&m.Description, spec.Description)
	overlay(&m.IsFolder, spec.IsFolder)
	overlay(&m.Archived, spec.Archived)
	overlay(&m.BusinessUnit, spec.BusinessUnit)
	overlay(&m.ParentProjectID, spec.ParentProjectID)
	overlaySlice(&m.Identifiers, spec.Identifiers)
	overlaySlice(&m.ProjectOwners, spec.ProjectOwners)
	overlaySlice(&m.SecurityChampions, spec.SecurityChampions)
	overlaySlice(&m.CloudAccountLinks, spec.CloudAccountLinks)
	overlaySlice(&m.CloudOrganizationLinks, spec.CloudOrganizationLinks)
	overlaySlice(&m.KubernetesClusterLinks, spec.KubernetesClusterLinks)
	r, sr := &m.RiskProfile, spec.RiskProfile
	overlay(&r.BusinessImpact, sr.BusinessImpact)
	overlay(&r.IsActivelyDeveloped, sr.IsActivelyDeveloped)
	overlay(&r.HasAuthentication, sr.HasAuthentication)
	overlay(&r.HasExposedAPI, sr.HasExposedAPI)
	overlay(&r.IsInternetFacing, sr.IsInternetFacing)
	overlay(&r.IsCustomerFacing, sr.IsCustomerFacing)
	overlay(&r.StoresData, sr.StoresData)
	overlay(&r.IsRegulated, sr.IsRegulated)
	overlaySlice(&r.SensitiveDataTypes, sr.SensitiveDataTypes)
	overlaySlice(&r.RegulatoryStandards, sr.RegulatoryStandards)
	return m
}

// upToDate reports whether Wiz already holds every field the spec sets.
// Lists are compared as sets, since Wiz does not promise an order.
func upToDate(spec, obs project.Parameters) bool {
	return equal(normalize(merged(spec, obs)), normalize(obs))
}

func normalize(p project.Parameters) project.Parameters {
	p.Identifiers = sortedStrings(p.Identifiers)
	p.ProjectOwners = sortedStrings(p.ProjectOwners)
	p.SecurityChampions = sortedStrings(p.SecurityChampions)
	p.RiskProfile.SensitiveDataTypes = sortedStrings(p.RiskProfile.SensitiveDataTypes)
	p.RiskProfile.RegulatoryStandards = sortedStrings(p.RiskProfile.RegulatoryStandards)
	p.CloudAccountLinks = slices.Clone(p.CloudAccountLinks)
	for i := range p.CloudAccountLinks {
		l := &p.CloudAccountLinks[i]
		l.ResourceGroups = sortedStrings(l.ResourceGroups)
		l.ResourceTags = sortedTags(l.ResourceTags)
		l.Shared = boolDefault(l.Shared)
	}
	sort.Slice(p.CloudAccountLinks, func(i, j int) bool {
		return p.CloudAccountLinks[i].CloudAccount < p.CloudAccountLinks[j].CloudAccount
	})
	p.CloudOrganizationLinks = slices.Clone(p.CloudOrganizationLinks)
	for i := range p.CloudOrganizationLinks {
		l := &p.CloudOrganizationLinks[i]
		l.ResourceGroups = sortedStrings(l.ResourceGroups)
		l.ResourceTags = sortedTags(l.ResourceTags)
		l.Shared = boolDefault(l.Shared)
	}
	sort.Slice(p.CloudOrganizationLinks, func(i, j int) bool {
		return p.CloudOrganizationLinks[i].CloudOrganization < p.CloudOrganizationLinks[j].CloudOrganization
	})
	p.KubernetesClusterLinks = slices.Clone(p.KubernetesClusterLinks)
	for i := range p.KubernetesClusterLinks {
		l := &p.KubernetesClusterLinks[i]
		l.Namespaces = sortedStrings(l.Namespaces)
		l.Shared = boolDefault(l.Shared)
	}
	sort.Slice(p.KubernetesClusterLinks, func(i, j int) bool {
		return p.KubernetesClusterLinks[i].KubernetesCluster < p.KubernetesClusterLinks[j].KubernetesCluster
	})
	return p
}

// equal compares two normalized parameter sets, treating nil and empty
// slices and nil and empty strings alike.
func equal(a, b project.Parameters) bool {
	return reflect.DeepEqual(canon(a), canon(b))
}

func canon(p project.Parameters) project.Parameters {
	emptyToNil(&p.Description)
	emptyToNil(&p.BusinessUnit)
	emptyToNil(&p.ParentProjectID)
	nilEmpty(&p.Identifiers)
	nilEmpty(&p.ProjectOwners)
	nilEmpty(&p.SecurityChampions)
	nilEmpty(&p.RiskProfile.SensitiveDataTypes)
	nilEmpty(&p.RiskProfile.RegulatoryStandards)
	nilEmpty(&p.CloudAccountLinks)
	nilEmpty(&p.CloudOrganizationLinks)
	nilEmpty(&p.KubernetesClusterLinks)
	for i := range p.CloudAccountLinks {
		nilEmpty(&p.CloudAccountLinks[i].ResourceGroups)
		nilEmpty(&p.CloudAccountLinks[i].ResourceTags)
	}
	for i := range p.CloudOrganizationLinks {
		nilEmpty(&p.CloudOrganizationLinks[i].ResourceGroups)
		nilEmpty(&p.CloudOrganizationLinks[i].ResourceTags)
	}
	for i := range p.KubernetesClusterLinks {
		nilEmpty(&p.KubernetesClusterLinks[i].Namespaces)
	}
	return p
}

func toCreateInput(p project.Parameters) wizapi.CreateProjectInput {
	in := wizapi.CreateProjectInput{
		Name:              p.Name,
		Slug:              p.Slug,
		Description:       p.Description,
		IsFolder:          p.IsFolder,
		Archived:          p.Archived,
		BusinessUnit:      p.BusinessUnit,
		Identifiers:       p.Identifiers,
		ParentProjectId:   p.ParentProjectID,
		ProjectOwners:     p.ProjectOwners,
		SecurityChampions: p.SecurityChampions,
		RiskProfile:       toRiskProfileInput(p.RiskProfile),
	}
	in.CloudAccountLinks, in.CloudOrganizationLinks, in.KubernetesClusterLinks = toLinkInputs(p)
	return in
}

func toPatch(p project.Parameters) *wizapi.UpdateProjectPatch {
	rp := toRiskProfileInput(p.RiskProfile)
	patch := &wizapi.UpdateProjectPatch{
		Name:              ptr(p.Name),
		Slug:              p.Slug,
		Description:       p.Description,
		Archived:          p.Archived,
		BusinessUnit:      p.BusinessUnit,
		Identifiers:       p.Identifiers,
		ParentProjectId:   p.ParentProjectID,
		ProjectOwners:     p.ProjectOwners,
		SecurityChampions: p.SecurityChampions,
		RiskProfile:       &rp,
	}
	patch.CloudAccountLinks, patch.CloudOrganizationLinks, patch.KubernetesClusterLinks = toLinkInputs(p)
	return patch
}

func toRiskProfileInput(r project.RiskProfile) wizapi.ProjectRiskProfileInput {
	return wizapi.ProjectRiskProfileInput{
		BusinessImpact:      (*wizapi.BusinessImpact)(r.BusinessImpact),
		IsActivelyDeveloped: (*wizapi.YesNoUnknown)(r.IsActivelyDeveloped),
		HasAuthentication:   (*wizapi.YesNoUnknown)(r.HasAuthentication),
		HasExposedAPI:       (*wizapi.YesNoUnknown)(r.HasExposedAPI),
		IsInternetFacing:    (*wizapi.YesNoUnknown)(r.IsInternetFacing),
		IsCustomerFacing:    (*wizapi.YesNoUnknown)(r.IsCustomerFacing),
		StoresData:          (*wizapi.YesNoUnknown)(r.StoresData),
		IsRegulated:         (*wizapi.YesNoUnknown)(r.IsRegulated),
		SensitiveDataTypes:  typed[wizapi.ProjectDataType](r.SensitiveDataTypes),
		RegulatoryStandards: typed[wizapi.RegulatoryStandard](r.RegulatoryStandards),
	}
}

func toLinkInputs(p project.Parameters) ([]wizapi.ProjectCloudAccountLinkInput, []wizapi.ProjectCloudOrganizationLinkInput, []wizapi.ProjectKubernetesClusterLinkInput) {
	ca := make([]wizapi.ProjectCloudAccountLinkInput, 0, len(p.CloudAccountLinks))
	for _, l := range p.CloudAccountLinks {
		ca = append(ca, wizapi.ProjectCloudAccountLinkInput{
			CloudAccount: l.CloudAccount, Environment: wizapi.Environment(l.Environment),
			ResourceGroups: l.ResourceGroups, ResourceTags: tagInputs(l.ResourceTags), Shared: l.Shared,
		})
	}
	co := make([]wizapi.ProjectCloudOrganizationLinkInput, 0, len(p.CloudOrganizationLinks))
	for _, l := range p.CloudOrganizationLinks {
		co = append(co, wizapi.ProjectCloudOrganizationLinkInput{
			CloudOrganization: l.CloudOrganization, Environment: wizapi.Environment(l.Environment),
			ResourceGroups: l.ResourceGroups, ResourceTags: tagInputs(l.ResourceTags), Shared: l.Shared,
		})
	}
	kc := make([]wizapi.ProjectKubernetesClusterLinkInput, 0, len(p.KubernetesClusterLinks))
	for _, l := range p.KubernetesClusterLinks {
		kc = append(kc, wizapi.ProjectKubernetesClusterLinkInput{
			KubernetesCluster: l.KubernetesCluster, Environment: wizapi.Environment(l.Environment),
			Namespaces: l.Namespaces, Shared: l.Shared,
		})
	}
	return ca, co, kc
}

func tagInputs(tags []project.ResourceTag) []wizapi.ResourceTagInput {
	if len(tags) == 0 {
		return nil
	}
	out := make([]wizapi.ResourceTagInput, 0, len(tags))
	for _, t := range tags {
		out = append(out, wizapi.ResourceTagInput{Key: t.Key, Value: t.Value})
	}
	return out
}

func ptr[T any](v T) *T { return &v }

func overlay[T any](dst **T, src *T) {
	if src != nil {
		*dst = src
	}
}

func overlaySlice[T any](dst *[]T, src []T) {
	if src != nil {
		*dst = src
	}
}

func emptyToNil(s **string) {
	if *s != nil && **s == "" {
		*s = nil
	}
}

func nilEmpty[T any](s *[]T) {
	if len(*s) == 0 {
		*s = nil
	}
}

func boolDefault(b *bool) *bool {
	if b == nil {
		return ptr(false)
	}
	return b
}

func sortedStrings(s []string) []string {
	if s == nil {
		return nil
	}
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

func sortedTags(t []project.ResourceTag) []project.ResourceTag {
	if t == nil {
		return nil
	}
	c := slices.Clone(t)
	sort.Slice(c, func(i, j int) bool {
		if c[i].Key != c[j].Key {
			return c[i].Key < c[j].Key
		}
		return c[i].Value < c[j].Value
	})
	return c
}

func stringsOf[T ~string](in []T) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func typed[T ~string](in []string) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	for i, v := range in {
		out[i] = T(v)
	}
	return out
}
