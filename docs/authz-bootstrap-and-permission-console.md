# AuthZ Bootstrap and Permission Console

This document records the implementation slice for the IAM authorization model.

## 1. Principal contract

Gateway OIDC restores the authenticated caller into Kernel `authn.Principal`.

The authorization subject must always use the stable Kernel subject id:

```text
user:<principal.subject_id>
```

For Casdoor users, `principal.subject_id` must be the stable Casdoor user UUID. Do not use username, display name, email, or `external_id` as the SpiceDB subject.

Example:

```text
subject_id   = b22888d1-5cd0-4700-8e67-aa4a622fd715
subject_type = user
provider     = casdoor
external_id  = aisphere/use
issuer       = https://casdoor.weagent.cc:30723
org_id       = aisphere
username     = use
groups       = []
```

The correct SpiceDB subject is:

```text
user:b22888d1-5cd0-4700-8e67-aa4a622fd715
```

`groups = []` only means the current OIDC token did not project groups or roles. It does not mean the user has no SpiceDB permissions.

## 2. Directory permission checks

The generated proto policy for directory reads previously used old `iam:org:*` resources. Directory reads now skip generated authz and are checked in `IAMDirectoryService` directly against the `zone` resource.

| API | SpiceDB check |
| --- | --- |
| `GetUser` | `zone:{org_id}#view_users@user:{subject_id}` |
| `ListUsers` | `zone:{org_id}#view_users@user:{subject_id}` |
| `GetOrganization` | `zone:{org_id}#view_zone@user:{subject_id}` |
| `ListGroups` | `zone:{org_id}#view_groups@user:{subject_id}` |

This makes the user-list failure explicit and easier to diagnose:

```text
spicedb check permission failed: zone:aisphere#view_users@user:<uuid>
```

## 3. Casdoor JWKS and certificate contract

Bootstrap admin resolution and JWT verification are separate concerns.

### 3.1 Bootstrap admin resolution

Resolving `aisphere/admin` into a stable Casdoor user UUID is a Casdoor Admin / M2M directory call. It needs:

```yaml
security:
  authn:
    casdoor:
      endpoint: https://casdoor.weagent.cc:30723
      admin:
        enabled: true
        organization_name: aisphere
        application_name: iam-service
        client_id: CHANGE_ME_M2M_CLIENT_ID
        client_secret: CHANGE_ME_M2M_CLIENT_SECRET
```

This lookup does not itself require IAM to verify an inbound JWT signature.

### 3.2 Token verification

IAM still needs OIDC Discovery / JWKS whenever it verifies Casdoor tokens directly, for example:

- `security.authn.mode: casdoor_jwt` or `oidc_jwt`;
- `VerifyToken` / token introspection style APIs;
- future ExternalAuth modes where IAM validates the external bearer token instead of trusting Gateway headers;
- local development or fallback flows that bypass Gateway OIDC.

Production should prefer JWKS over a static `casdoor.pub` file:

```yaml
security:
  authn:
    oidc:
      issuer: https://casdoor.weagent.cc:30723
      discovery_url: https://casdoor.weagent.cc:30723/.well-known/openid-configuration
      jwks_url: https://casdoor.weagent.cc:30723/.well-known/jwks
      audience: [869aff97ab0408cbbd1c]
      allowed_owners: [aisphere]
```

`jwt_certificate_file` remains a local/dev fallback only. In production it should not be the primary certificate rotation mechanism because it cannot automatically track Casdoor key rotation.

## 4. Bootstrap admin

IAM can bootstrap the initial Casdoor admin user by username.

```yaml
control_plane:
  bootstrap_admins:
    enabled: true
    subjects:
      - type: user
        username: admin
        zone_id: aisphere
        casdoor_org: aisphere
        role: zone_owner
        source: bootstrap
        reason: initial Casdoor admin user
```

At startup, IAM uses the configured Casdoor M2M admin client to resolve `aisphere/admin` into the stable user UUID and writes:

```text
zone:aisphere#owner@user:<admin_uuid>
```

Bootstrap now writes one personnel relationship at the scope the administrator owns. Platform administrators are explicit:

```text
platform:global#owner@user:<admin_uuid>
platform:global#admin@user:<admin_uuid>
```

Use `platform_owner` or `platform_admin` only when the subject truly administers every zone. The compatibility aliases `owner` and `admin` remain zone-scoped, so an existing `zone_owner` bootstrap cannot silently become a platform administrator.

Structural links connect `platform:global` to each zone and global IAM resource. Organization administrators keep one direct `zone:<id>#owner|admin` relationship and inherit group management through the zone hierarchy; IAM does not copy them into every child group as a direct manager.

The scoped roles and structural resource list are loaded from the `bootstrap` section of `configs/resource/defaults.yaml`. Bootstrap subjects remain in environment-specific service configuration, while permission policy remains in the shared manifest.

Before startup writes relationships, IAM validates the manifest against `configs/spicedb/aisphere.schema.zed`. The same check is available offline:

```text
make permission-manifest-check
```

Schema bootstrap automatically publishes missing definitions, relations, and permissions. Existing permission expressions change only when `security.authz.allow_permission_migrations` is explicitly enabled. Removed active declarations still fail closed.

## 5. Permission Console resource

The SpiceDB schema now includes a global IAM authorization-control resource:

```zed
definition iam_authz {
  relation owner: user | service | service_account | group#member
  relation admin: user | service | service_account | group#member
  relation schema_admin: user | service | service_account | group#member
  relation auditor: user | service | service_account | group#member

  permission view_schema = owner + admin + schema_admin + auditor
  permission publish_schema = owner + admin + schema_admin
  permission view_relationships = owner + admin + schema_admin + auditor
  permission repair_relationships = owner + admin + schema_admin
  permission manage = owner + admin + schema_admin
}
```

This backs the UI module for:

- schema view / validate / publish / diff;
- relationship explorer;
- grant and revoke operations;
- permission explain;
- drift detection and repair.

## 6. Proto-first development workflow

AuthZ Admin API must be developed from `api/iam/v1/iam.proto` first. Do not add final product APIs as hand-written HTTP routes.

The workflow is:

```bash
make tools
make api
make deploy
make proto-check
go test ./...
```

`make api` regenerates:

- protobuf Go types;
- gRPC service bindings;
- Kernel HTTP bindings;
- grpc-gateway bindings;
- Kernel service catalog / access metadata;
- OpenAPI docs.

`make deploy` regenerates Gateway API manifests under:

```text
deploy/generated
```

The Docker build and GitHub Actions workflow both run `make api` and `make deploy` before building the binary.

## 7. AuthZ Admin API contract

The Permission Console API is now represented by `IAMAuthorizationAdminService` in proto:

```text
GET  /v1/iam/authz/schema
POST /v1/iam/authz/schema:validate
POST /v1/iam/authz/schema:publish
GET  /v1/iam/authz/relationships
POST /v1/iam/authz/relationships
POST /v1/iam/authz/relationships:delete
POST /v1/iam/authz/permissions:check
POST /v1/iam/authz/permissions:explain
GET  /v1/iam/authz/effective-permissions
```

Generated access metadata uses `SELF_CHECK`, and `IAMAuthorizationAdminService` performs concrete SpiceDB checks against:

```text
iam_authz:global#view_schema
iam_authz:global#publish_schema
iam_authz:global#view_relationships
iam_authz:global#repair_relationships
```

## 8. Role-first grants

Built-in roles continue to map to native resource relations. Custom roles store an ordered permission set in PostgreSQL and project capabilities to `custom_role` objects. A custom grant creates a stable `role_binding` between one role, one user or `group#member`, and one concrete resource.

This supports fine-grained sharing such as “share `skill:skill-a` with `user:alice` as reviewer” without inventing a global RBAC role or duplicating a relationship for every capability. Updating a custom role is version-checked, audited, impact-previewed, and projected through the outbox.

## 9. Safe rollout and rollback

Use this order for an existing environment:

1. Deploy the additive schema containing `platform`, hierarchy links, `custom_role`, and `role_binding`.
2. For the reviewed schema rollout only, set `security.authz.allow_permission_migrations: true`; start IAM and verify representative platform, zone, group, and resource checks.
3. Return `allow_permission_migrations` to `false` immediately after the schema is installed.
4. Keep `control_plane.bootstrap_admins.cleanup_legacy_expansions: false` while confirming each administrator has the expected single platform or zone relationship.
5. Enable `cleanup_legacy_expansions: true` for one controlled restart. Review planned/deleted relationship counts and representative access checks, then return it to `false`.
6. Deploy the role-first frontend only after the new GrantService API is reachable.

Rollback the service and bootstrap configuration while leaving additive schema definitions in place. Do not remove `custom_role`, `role_binding`, `custom_binding`, or their permission branches while custom roles or grants exist. Leaving unused additive schema in place is safe; deleting types that still have bindings is not.
