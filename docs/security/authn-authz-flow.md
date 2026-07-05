# IAM AuthN and AuthZ Flow

This document describes the two supported security flows for IAM and downstream services.

## Flow 1: Gateway-authenticated request, backend AuthZ check

This is the normal browser/API path.

```text
Client
  -> Gateway verifies external OIDC/JWT
  -> Gateway strips client-controlled trusted headers
  -> Gateway injects X-Aisphere-* principal headers
  -> Gateway injects internal service token
  -> IAM/backend runs gateway_trusted AuthN middleware
  -> IAM/backend restores Principal into context
  -> accessx resolves proto policy into an AuthZ check
  -> authz provider checks SpiceDB/ReBAC relationship graph
  -> handler runs only after AuthN + AuthZ pass
```

Key points:

- External clients must not be allowed to set `X-Aisphere-*` identity headers directly.
- Gateway is responsible for removing any inbound trusted identity headers before injecting its own verified principal.
- Backend services in `gateway_trusted` mode trust those headers only after the internal service token is valid.
- Public operations use `security.access.public_operations` and skip both AuthN and AuthZ.
- Self-service/bootstrap operations use `security.access.skip_operations` and skip AuthZ only; they still require AuthN and audit.
- Normal protected operations use generated proto access metadata and are checked through AuthZ.

## Flow 2: IAM identity administration also requires AuthZ

Directory and identity-administration operations must not bypass authorization just because they are AuthN-related.

```text
Principal restored from Gateway
  -> request hits IAM directory / identity admin API
  -> accessx checks permission on the IAM control-plane resource
  -> IAM mutates or reads the identity backend only after AuthZ allows it
```

Examples:

| Operation kind | Expected protection |
| --- | --- |
| Directory read, such as list users/groups | Generated proto policy with `AUTHORIZED` exposure and AuthZ check. |
| Relationship write/delete | Generated proto policy with `AUTHORIZED` exposure and AuthZ check. |
| Legacy local user CRUD under `/v1/users` | Explicit handler-level `accessx` check because the route is not generated from proto. |
| Local application group membership changes | Identity operation runs first, then membership is projected into AuthZ relationships. |

## Local user management

The legacy `/v1/users` API is still a plain HTTP handler for compatibility with the existing frontend. Because it is not a generated proto operation, it does not have a generated access resolver. The handler therefore performs explicit checks:

```text
GET /v1/users
  -> check iam:local_user#list

POST /v1/users
  -> check iam:local_user#upsert

DELETE /v1/users/{username}
  -> check iam:local_user#delete
```

Bootstrap admins receive `admin` on `iam:local_user` by default, and the IAM schema exposes `list`, `upsert`, and `delete` permissions on `definition iam`.

Long term, this legacy API should be migrated into a generated proto service so Gateway registration, request info, access resolver, and audit metadata are all generated uniformly.

## Group and membership projection

IAM uses groups as local application authorization constructs in both `casdoor_local` and `external_oidc` identity modes.

Projected AuthZ relationships:

```text
AssignUserToGroup(user_id, group_id)
  -> group:<group_id>#member@user:<user_id>

CreateGroup(parent_id, group_id)
UpdateGroup(parent_id, group_id)
  -> group:<parent_id>#member@group:<group_id>#member

RemoveUserFromGroup(user_id, group_id)
  -> delete group:<group_id>#member@user:<user_id>

DeleteGroup(group_id)
  -> delete relationships where resource is group:<group_id>
  -> delete relationships where subject is group:<group_id>#member
```

This lets resource policies grant permissions to `group#member` while users can come from either local Casdoor or an external OIDC directory.

## Identity mode boundary

`casdoor_local`:

```text
IAM can manage local users, identity organizations, applications,
groups, and group membership.
```

`external_oidc`:

```text
IAM reads upstream users and identity organizations.
IAM must not mutate upstream users or upstream identity organizations.
IAM can still manage local application groups and group membership.
```

## Current implementation status

- Gateway-trusted AuthN is implemented through `securityx` and `authn.TrustedHeaderAuthenticator`.
- Generated proto APIs are protected through `iamServerMiddlewares`, generated request-info resolvers, and generated access resolvers.
- Local user legacy APIs are now explicitly protected by handler-level `accessx` checks.
- Local group changes are projected into AuthZ through `BindIdentityAuthZ`.
- The remaining recommended cleanup is to replace legacy `/v1/users` with a generated proto identity-admin service.
