# Release notes

## Unreleased

> **Unofficial provider**, not made, approved or supported by Wiz, Inc. Alpha: every kind is
> `v1alpha1`, and the native controllers have not yet been run against a live Wiz tenant.

### Native Wiz API controllers

Project, User and ServiceAccount now call the Wiz GraphQL API directly instead of going through the
`jschoombee/wiz` Terraform provider. All other kinds are still served by Upjet. Group, kind and
external name are unchanged, so existing objects keep pointing at the same Wiz objects.

### Breaking changes

- **CloudAccountLink removed.** Cloud account links are now `Project.spec.forProvider.cloudAccountLinks`,
  next to `cloudOrganizationLinks` and `kubernetesClusterLinks`.
- **Project delete archives.** Wiz cannot delete projects. Deleting a Project archives it and renames
  it to its slug to free the name.
- **ServiceAccount is immutable.** `name`, `type` and `scopes` cannot be changed, and a change to
  `assignedProjects` reports a sync error until the resource is recreated. `recreateIfRotated` is
  removed. The connection secret gets `client_id` and `client_secret` on create only.
- **Native kinds drop Upjet-only fields.** These are `initProvider` and the Terraform-only status
  fields. The proxy keys of the credentials secret (`proxy`, `proxy_server`, `ca_chain`) are not
  read by native kinds, which use `HTTPS_PROXY` instead.

### New

- User `isSuspended`.
- For Project lists and User `assignedProjectIds`, unset means not managed and `[]` clears the list.
- Native kinds support observe-only imports with an empty `forProvider`.
