# IAM Resource Control Plane Phase 4: Biz/Service Layer

## Status

Phase 4 adds domain services on top of the Phase 3 repository layer. The first
implementation is intentionally pure Go and does not depend on generated
`project/resource/grant` protobuf code. After `make api` is available, the gRPC
service layer only needs DTO conversion from generated request messages into
these domain requests.

## Packages

```text
internal/biz/project
  Organization / Project / Capability use cases.

internal/biz/resource
  ResourceType / Resource projection / ResourceBinding / ExternalBinding use cases.

internal/biz/grant
  RoleTemplate / GrantAccess / RevokeAccess / ExplainAccess use cases.

internal/biz/graph
  Thin adapter from domain events to kernel/authz RelationshipWriter.

internal/biz/defaults
  Idempotent loader for configs/resource/defaults.yaml.
```

## Kernel Reuse Boundary

The biz layer still follows the earlier boundary decision:

```text
Casdoor  -> identity source only
kernel   -> provider-neutral authz/db/config contracts
IAM DB   -> resource-control-plane facts
SpiceDB  -> ReBAC graph projection
```

The biz layer imports `kernel/authz` only through provider-neutral interfaces:

```go
type RelationshipWriter interface {
    WriteRelationships(ctx context.Context, relationships ...Relationship) (WriteResult, error)
    DeleteRelationships(ctx context.Context, filter RelationshipFilter) (WriteResult, error)
}
```

No biz package imports `authz/spicedb` directly.

## Main Workflows

### CreateOrganization

```text
project.Service.CreateOrganization
  -> iam_organizations insert
  -> optional SpiceDB: organization:<id>#owner@subject
```

### CreateProject

```text
project.Service.CreateProject
  -> iam_projects insert
  -> SpiceDB: project:<id>#parent@organization:<org_id>
  -> optional SpiceDB: project:<id>#owner@subject
```

### UpsertResource

```text
resource.Service.UpsertResource
  -> validate resource_type exists
  -> iam_resources upsert
  -> if parent exists: <resource>#parent@<parent>
  -> optional owner relation
```

`resource_type.spicedb_type` is used when projecting object types. This keeps
product-facing resource type names decoupled from the SpiceDB schema names.

### BindResource

```text
resource.Service.BindResource
  -> iam_resource_bindings upsert
  -> SpiceDB relationship projection
```

The first special cross-domain alias is implemented explicitly:

```text
skill:s1 --backing_repo--> git_repository:r1
  -> git_repository:r1#backing_skill@skill:s1
```

Default behavior for other bindings is:

```text
source#relation@target
```

### GrantAccess

```text
grant.Service.GrantAccess
  -> find enabled role_template(resource_type, role_key)
  -> iam_grants insert
  -> iam_grant_audits insert
  -> SpiceDB: resource#relation@subject
```

This preserves the product-level RBAC surface while storing and computing with
ReBAC relationships.

### RevokeAccess

```text
grant.Service.RevokeAccess
  -> load grant
  -> set revoked_at
  -> iam_grant_audits insert
  -> optional SpiceDB DeleteRelationships
```

The delete operation is explicit because some revoke flows may want to preserve
historical graph relationships until a reconciler runs.

## Defaults Reconciliation

`internal/biz/defaults` loads `configs/resource/defaults.yaml` and upserts:

- capabilities
- resource types
- role templates

It is controlled by:

```yaml
control_plane:
  defaults:
    enabled: false
    path: configs/resource/defaults.yaml
```

Default reconciliation is disabled by default. In dev you can enable it after
migrations are applied and the database is configured.

## What is intentionally not done yet

- No generated pb service implementation for `ProjectService`, `ResourceService`,
  or `GrantService` yet.
- No full outbox/reconciler loop yet.
- No default organization/project bootstrap yet.
- No admin UI yet.

Those are Phase 5/6 tasks.
