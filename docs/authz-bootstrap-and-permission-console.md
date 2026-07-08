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

For zone owner/admin bootstrap subjects, IAM also writes control-plane admin relationships for default IAM resources, including:

```text
iam:organization#admin@user:<admin_uuid>
iam:capability#admin@user:<admin_uuid>
iam:resource_type#admin@user:<admin_uuid>
iam:resource#admin@user:<admin_uuid>
iam:resource_binding#admin@user:<admin_uuid>
iam:external_resource_binding#admin@user:<admin_uuid>
iam:role_template#admin@user:<admin_uuid>
iam:grant#admin@user:<admin_uuid>
iam_authz:global#admin@user:<admin_uuid>
```

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
ia m_authz:global#publish_schema
iam_authz:global#view_relationships
iam_authz:global#repair_relationships
```

## 8. Next implementation slices

1. Run `make api` and commit generated files.
2. Run `make deploy` and commit or publish generated Gateway API manifests according to the repo release policy.
3. Replace first-slice UI direct relationship writes with a higher-level Grant Wizard.
4. Add audit events for schema publish, grant, revoke, and repair.
5. Add integration tests for admin bootstrap, user-list authorization, schema validation, and relationship explorer.
