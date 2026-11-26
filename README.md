# provider-wiz

![provider-wiz](docs/images/hero.svg)

> [!IMPORTANT]
> **Unofficial provider.** This is a community project. It is not made, approved, endorsed or
> supported by Wiz, Inc. "Wiz" is a trademark of its owner and is used here only to describe what
> the provider manages. Use it at your own risk, and contact Wiz support only about Wiz itself,
> not about this provider.

`provider-wiz` is a [Crossplane](https://crossplane.io/) provider for [Wiz](https://www.wiz.io).

It is moving, kind by kind, from [Upjet](https://github.com/crossplane/upjet) controllers that wrap
the [`jschoombee/wiz`](https://registry.terraform.io/providers/jschoombee/wiz/latest/docs) Terraform
provider ([source](https://github.com/jschoombee/terraform-provider-wiz), pinned to `v0.0.7`) to
native controllers that call the Wiz GraphQL API directly. Then the provider is no longer limited to
what that Terraform provider covers. Both kinds of controller run in the same provider and share one
`ProviderConfig`. See [docs/native-migration.md](docs/native-migration.md) for how the move works.

> [!WARNING]
> **Alpha.** Every kind is `v1alpha1`. The native controllers are tested against an in-memory fake
> of the Wiz API but have **not yet been run against a live Wiz tenant**. Their GraphQL schema is
> reconstructed from the Terraform provider's source, because Wiz's API reference needs a login.
> Try them on a test tenant before relying on them.

## Kinds

Every kind is served both cluster scoped (`<group>.wiz.polarpoint.io`) and namespaced for
Crossplane v2 (`<group>.wiz.m.polarpoint.io`).

| Group | Kind | Controller |
| --- | --- | --- |
| `project` | Project | **native** |
| `iam` | User, ServiceAccount | **native** |
| `iam` | SAMLIdP, SAMLGroupMapping | Upjet |
| `connector` | AWS, GCP | Upjet |
| `integration` | AWSSNS, Jira, ServiceNow | Upjet |
| `automation` | RuleAWSSNS, RuleJiraAddComment, RuleJiraCreateTicket, RuleJiraTransitionTicket, RuleServiceNowCreateTicket, RuleServiceNowUpdateTicket | Upjet |
| `policy` | CloudConfigRule, CloudConfigRuleAssociations, HostConfigRuleAssociations, Control, ControlAssociations, SecurityFramework, CICDScanPolicy | Upjet |
| `report` | GraphQuery | Upjet |

The old `CloudAccountLink` kind is gone: cloud account links are now part of the Project spec.

### How the native kinds behave

Group, kind and external name (the Wiz ID) are the same as in the Upjet versions, so existing
objects keep pointing at the same Wiz objects. These differences matter:

- **Project**
  - Wiz cannot delete projects. Deleting a Project archives it and renames it to its slug, which
    frees the name for a new project.
  - `cloudAccountLinks`, `cloudOrganizationLinks` and `kubernetesClusterLinks` are part of
    `spec.forProvider`.
  - A top-level list field (`identifiers`, `projectOwners`, `securityChampions` and the three link
    lists) left unset is not managed, and `[]` clears it in Wiz. The lists inside `riskProfile` are
    only sent when they have entries.
  - `slug` and `isFolder` cannot be changed or removed once set.
- **User**
  - Leaving `assignedProjectIds` unset leaves assignments alone, and `[]` removes every
    assignment. You can fill it from Projects with `assignedProjectIdsRefs` or
    `assignedProjectIdsSelector`.
  - `isSuspended` is new.
  - `sendEmailInvite` defaults to `true` and only applies when the user is created.
  - Role and projects are compared with the values Wiz actually applies, which a SAML group mapping
    can override.
- **ServiceAccount**
  - Wiz cannot update a service account. `name`, `type` and `scopes` cannot be changed, and a
    change to `assignedProjects` shows as a sync error until you recreate the resource.
  - `client_id` and `client_secret` go to the connection secret, and Wiz returns the secret
    only once, at creation. Set `writeConnectionSecretToRef` before creating the resource. If the
    secret is lost, the resource shows a sync error, and you need to recreate it to get a new
    secret.
  - The Upjet option `recreateIfRotated` is gone. `status.atProvider.lastRotatedAt` shows when the
    secret was rotated.
  - `type` defaults to `THIRD_PARTY`. `scopes` and `assignedProjects` only apply to that type.

All native kinds also work in observe-only mode (`managementPolicies: [Observe]`). In that mode only
the external-name annotation and an empty `forProvider: {}` are required.

## Install

Packages are published to `ghcr.io/polarpoint-io/provider-wiz` by the manual *Publish Provider
Package* workflow. [examples/install.yaml](examples/install.yaml) installs one. Set its version to
the one you published.

```console
kubectl apply -f examples/install.yaml
```

## Authentication

Create a Wiz service account of type *Custom Integration (GraphQL API)*. Store its credentials as
JSON in a Secret, then point a `ProviderConfig` or `ClusterProviderConfig` at it. Upjet and native
kinds use the same secret.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: wiz-creds
  namespace: crossplane-system
stringData:
  credentials: |
    {
      "wiz_url": "https://api.<region>.app.wiz.io/graphql",
      "wiz_auth_client_id": "<client id>",
      "wiz_auth_client_secret": "<client secret>"
    }
---
apiVersion: wiz.m.polarpoint.io/v1beta1
kind: ClusterProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      name: wiz-creds
      namespace: crossplane-system
      key: credentials
```

| Key | Required | Used by |
| --- | --- | --- |
| `wiz_url` | yes | both |
| `wiz_auth_client_id`, `wiz_auth_client_secret` | yes | both |
| `wiz_auth_url` (default `https://auth.app.wiz.io/oauth/token`) | no | both |
| `wiz_auth_audience` (default `wiz-api`) | no | both |
| `http_client_retry_max` (default `5` for native kinds, `10` for Upjet kinds) | no | both |
| `wiz_auth_grant_type`, `http_client_retry_wait_*` | no | Upjet only |
| `proxy`, `proxy_server`, `ca_chain` | no | Upjet only; native kinds use the `HTTPS_PROXY` environment variable |

A `ServiceAccount` managed resource writes `client_id` and `client_secret` to its connection
secret, so it can feed another ProviderConfig.

## Examples

| Path | What |
| --- | --- |
| [examples/install.yaml](examples/install.yaml) | Install the provider package |
| [examples/cluster/providerconfig](examples/cluster/providerconfig), [examples/namespaced/providerconfig](examples/namespaced/providerconfig) | Credentials secret template and ProviderConfigs |
| [examples/cluster](examples/cluster), [examples/namespaced](examples/namespaced) | Hand-written examples for the native kinds (Project, User, ServiceAccount) |
| [examples-generated](examples-generated) | Examples for the Upjet kinds, generated from the Terraform provider's docs |

The User and ServiceAccount examples reference the Project example by name, so apply the Project
first.

## Developing

Run code generation:

```console
make submodules
make generate
```

`make generate` fetches the Terraform provider from the Terraform Registry to build
`config/schema.json` and regenerates the Upjet kinds and all CRDs. Resource groups, kinds and
references live in `config/resources.go`, external names in `config/external_name.go`, and
credential handling in `internal/clients/wiz.go`.

The native kinds live in `apis/common/<kind>`, `apis/{cluster,namespaced}/<group>/v1alpha1` and
`internal/controller/native/<kind>`. Their GraphQL client is generated from
`internal/clients/wizapi/schema` and `operations` with `go generate ./internal/clients/wizapi`.
[docs/native-migration.md](docs/native-migration.md) lists the steps to move another kind.

Run against a Kubernetes cluster with Crossplane installed:

```console
make run
```

Build, push, and install:

```console
make all
```

Build the binary:

```console
make build
```

Run the unit tests:

```console
go test ./...
```

## Notes

- The Upjet kinds run the Terraform CLI (Upjet's CLI mode). Moving them to Upjet's no-fork mode
  would need the Terraform provider to expose its `internal/provider` package publicly. Moving them
  to native controllers removes that need.

## Report a Bug

Open an [issue](https://github.com/polarpoint-io/provider-wiz/issues).
