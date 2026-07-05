# IAM Identity Directory and Casdoor Admin Boundary

IAM is the only platform service that may hold an elevated Casdoor management client. Gateway must not hold this credential.

Casdoor is treated as IAM's identity directory when it is the local identity provider. In this mode IAM manages Casdoor users, organizations, applications, and multi-level groups through a dedicated service application.

When the upstream identity source is an external OIDC provider, IAM must treat the identity directory as read-only. IAM may read and normalize users, organizations, and groups, but it must not expose write APIs that attempt to mutate the external provider.

## Responsibility split

```text
External user token
  -> Gateway verifies OIDC/JWKS
  -> Gateway injects trusted principal headers and the internal boundary token
  -> IAM restores the user principal in gateway_trusted mode
  -> IAM performs accessx / SpiceDB authorization
  -> IAM uses its dedicated Casdoor admin M2M client only when the provider mode permits writes
```

## Two identity-provider modes

| Mode | Provider role | IAM user management | IAM organization/group management | Gateway exposure |
| ---- | ------------- | ------------------- | --------------------------------- | ---------------- |
| `casdoor_local` | Casdoor is the local identity directory and token issuer. | Full create/read/update/disable/delete through IAM Identity Admin APIs. | Full Casdoor organization, application, and multi-level group management through IAM Identity Admin APIs. | Admin APIs may be `AUTHORIZED` when guarded by IAM/accessx/SpiceDB. |
| `external_oidc` | Casdoor or another OIDC endpoint is only a federation/login bridge. | Read-only directory/profile projection. No IAM write operations to the upstream identity source. | Read-only organization/group projection. Group membership is consumed for authorization projection, not mutated by IAM. | Directory read APIs may be `AUTHORIZED`; identity write APIs must not be registered to public Gateway. |

## Rules

1. Casdoor is used for authentication, OIDC token issuance, profile/user source, local identity-directory management, and M2M management APIs when it is the local identity provider.
2. Casdoor authorization is not used for business authorization.
3. Business authorization is owned by IAM + Kernel accessx + SpiceDB.
4. Gateway only verifies external tokens and forwards trusted identity. It does not get elevated Casdoor management credentials.
5. IAM must use a dedicated Casdoor service application for management calls such as user, organization, application, and group provisioning.
6. The dedicated admin client must be configured under `security.authn.casdoor.admin` when write-mode identity management is enabled.
7. IAM proto APIs must make the exposure explicit with `aisphere.access.v1.policy`.
8. Hand-written legacy HTTP handlers must not be registered to Gateway unless they are first migrated to proto + generated access policy.

## API shape

Use separate services for read-only directory access and identity administration:

```text
IAMDirectoryService
  GetUser
  ListUsers
  GetOrganization
  ListGroups

IAMIdentityAdminService
  CreateUser
  UpdateUser
  DisableUser
  DeleteUser
  CreateIdentityOrganization
  UpdateIdentityOrganization
  DeleteIdentityOrganization
  CreateGroup
  UpdateGroup
  DeleteGroup
  AssignUserToGroup
  RemoveUserFromGroup
```

Avoid naming identity-side organization mutations as plain `CreateOrganization`. Keep that name for resource/control-plane organization APIs or use a fully-qualified skip rule. Prefer `CreateIdentityOrganization` for Casdoor-side organization management.

## Gateway registration policy

IAM should register only generated proto routes into Gateway. Public Gateway registration must use `gatewayx.PublicRouteFilter()` so `INTERNAL` and `SYSTEM` routes do not enter the public route registry.

```go
serverx.RegisterServiceGatewayRoutesWithFilter(
    ctx,
    routeRegistry,
    gatewayx.PublicRouteFilter(),
    v1.IAMAuthServiceKernelModule(),
    v1.IAMDirectoryServiceKernelModule(),
    // v1.IAMIdentityAdminServiceKernelModule(), // add after proto generation
    projectv1.ProjectServiceKernelModule(),
    resourcev1.ResourceServiceKernelModule(),
    grantv1.GrantServiceKernelModule(),
)
```

## Required IAM configuration shape for local Casdoor management

```yaml
security:
  authn:
    mode: gateway_trusted
    provider: casdoor
    casdoor:
      # Browser/OIDC client used for login, callback, refresh, logout, and token verification config.
      organization_name: aisphere
      application_name: aisphere
      client_id: ${CASDOOR_LOGIN_CLIENT_ID}
      client_secret: ${CASDOOR_LOGIN_CLIENT_SECRET}

      # Elevated service application used only by IAM for Casdoor management APIs.
      admin:
        enabled: true
        organization_name: aisphere
        application_name: iam-service
        client_id: ${CASDOOR_IAM_M2M_CLIENT_ID}
        client_secret: ${CASDOOR_IAM_M2M_CLIENT_SECRET}
```

## Runtime policy

Before IAM performs a Casdoor management action, the caller must already be authenticated and authorized by IAM's own authorization path. The M2M admin credential is an implementation detail of IAM's adapter to Casdoor. It is not the authorization decision source.

```text
caller principal
  -> accessx.Check
  -> SpiceDB permission decision
  -> Casdoor admin M2M call only after allow
```

External OIDC mode stops before the final mutation step:

```text
caller principal
  -> accessx.Check
  -> directory read / profile projection
  -> no upstream identity mutation
```
