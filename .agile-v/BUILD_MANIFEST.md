# Build Manifest — Aisphere IAM

> Cycle: C1 | Generated: 2026-07-13 | Status: COMPLETE

## Summary

| Dimension | Count |
|-----------|:-----:|
| Total REQs | 63 |
| Total ART entries | 62 |
| Code without REQ | **0** ✅ |

## Artifact Index

### Authentication and Principal (P0)

| ART-ID | REQ-ID | Path | Notes |
|--------|--------|------|-------|
| ART-0001 | REQ-IAM-AUTHN-001 | `internal/service/iam.go::GetMe` | Returns normalized Principal from Kernel Context |
| ART-0002 | REQ-IAM-AUTHN-002 | `internal/service/iam.go::VerifyToken` | Delegates to authn.TokenService; INTERNAL only |
| ART-0003 | REQ-IAM-AUTHN-003 | `internal/service/control_plane.go::currentProjectContext` | Derives actor/owner from Kernel Principal |
| ART-0004 | REQ-IAM-AUTHN-004 | `internal/server/access.go::iamSkipPolicyResolver` | Gateway trust boundary; PUBLIC routes skip authz |

### Identity Directory (P0)

| ART-ID | REQ-ID | Path | Notes |
|--------|--------|------|-------|
| ART-0005 | DIR-001 | `internal/service/iam.go::GetUser` | Zone permission check + Casdoor delegation |
| ART-0006 | DIR-002 | `internal/service/iam.go::ListUsers` | Zone permission check + provider filter |
| ART-0007 | DIR-003 | `internal/service/iam.go::GetOrganization` | Zone permission check + Casdoor metadata |
| ART-0008 | DIR-004 | `internal/service/iam.go::ListGroups` | Zone permission check + Casdoor Group tree |
| ART-0009 | DIR-005 | `api/iam/v1/group_admin.proto` | Canonical Group management service |
| ART-0010 | DIR-005 | `internal/service/group_admin.go` | Group CRUD implementation |
| ART-0011 | DIR-006 | `internal/service/group_admin.go::AssignUserToGroup` | Membership assignment |
| ART-0012 | DIR-006 | `internal/service/group_admin.go::RemoveUserFromGroup` | Membership removal |
| ART-0013 | DIR-007 | `internal/data/identity_mode.go::externalOIDCIdentityAdmin` | Identity mode boundary enforcement |

### Directory Projection (P1)

| ART-ID | REQ | Path | Notes |
|--------|-----|------|-------|
| ART-0014 | PROJ-001 | `internal/data/identity_mode.go::groupTopologyRelationships` | Group identity projection |
| ART-0015 | PROJ-002 | `internal/data/identity_mode.go::groupTopologyRelationships` | Multi-level Group parent projection |
| ART-0016 | PROJ-003 | `internal/data/identity_mode.go::groupMemberRelationship` | User membership projection |
| ART-0017 | PROJ-004 | `internal/data/identity_mode.go::IdentityProjectionEventModel` | Projection event persistence |
| ART-0018 | PROJ-005 | `internal/data/identity_mode.go::RetryOnce` | Retry failed projection |
| ART-0019 | PROJ-006 | `internal/data/identity_mode.go::submit` | DTM Saga support |
| ART-0020 | PROJ-007 | `internal/data/identity_mode.go::DetectDirectoryProjectionDrift` | Drift detection/reconciliation |

### Runtime Authorization (P0)

| ART-ID | REQ | Description | Path |
|--------|-----|-------------|------|
| ART-0021 | AUTHZ-RT-001 | Single permission check | `internal/service/iam.go::CheckPermission` |
| ART-0022 | AUTHZ-RT-002 | Batch permission check | `internal/service/iam.go::BatchCheckPermissions` |
| ART-0023 | AUTHZ-RT-003 | Batch relationship write | `internal/service/iam.go::WriteRelationships` |
| ART-0024 | AUTHZ-RT-004 | Batch relationship delete | `internal/service/iam.go::DeleteRelationships` |
| ART-0025 | AUTHZ-RT-005 | Relationship read | `internal/service/iam.go::ReadRelationships` |
| ART-0026 | AUTHZ-RT-006 | Resource lookup | `internal/service/iam.go::LookupResources` |
| ART-0027 | AUTHZ-RT-007 | Subject lookup | `internal/service/iam.go::LookupSubjects` |
| ART-0028 | AUTHZ-RT-008 | gRPC identity propagation | `client/authzgrpc/client.go` |

### Authorization Administration (P1)

| ART-ID | REQ | Description | Path |
|--------|-----|-------------|------|
| ART-0029 | AUTHZ-ADMIN-001 | Read schema | `internal/service/authz_admin.go::GetAuthorizationSchema` |
| ART-0030 | AUTHZ-ADMIN-002 | Validate/publish schema | `internal/service/authz_admin.go::Validate/PublishAuthorizationSchema` |
| ART-0031 | AUTHZ-ADMIN-003 | Inspect/repair relationships | `internal/service/authz_admin.go::List/Write/DeleteRelationships` |
| ART-0032 | AUTHZ-ADMIN-004 | Diagnose authorization | `internal/service/authz_admin.go::Check/Explain/GetEffectivePermissions` |
| ART-0033 | AUTHZ-ADMIN-005 | Audit admin changes | `internal/data/data.go::auditx.NewPostgresStore` (PostgreSQL durable sink) |

### Project and Capability (P0)

| ART-ID | REQ | Description | Path |
|--------|-----|-------------|------|
| ART-0034 | PROJECT-001 | Casdoor Organization → Zone root | `internal/biz/project/project.go` |
| ART-0035 | PROJECT-002 | CreateProject from Principal | `internal/service/control_plane.go::CreateProject` |
| ART-0036 | PROJECT-003 | Creator ownership | `internal/biz/project/project.go::CreateProject` |
| ART-0037 | PROJECT-004 | Get/List Projects scoped to Zone | `internal/service/control_plane.go::GetProject/ListProjects` |
| ART-0038 | PROJECT-005 | UpdateProject | `internal/service/control_plane.go::UpdateProject` |
| ART-0039 | PROJECT-006 | ArchiveProject | `internal/service/control_plane.go::ArchiveProject` |
| ART-0040 | PROJECT-007 | Register/list Capabilities | `internal/service/control_plane.go::RegisterCapability/ListCapabilities` |
| ART-0041 | PROJECT-008 | Enable/disable/list Project Capabilities | `internal/service/control_plane.go::Enable/Disable/ListProjectCapabilities` |

### Resource Control Plane (P0)

| ART-ID | REQ | Description | Path |
|--------|-----|-------------|------|
| ART-0042 | RESOURCE-001 | Resource Type CRUD | `internal/service/control_plane.go::Register/Get/ListResourceType` |
| ART-0043 | RESOURCE-002 | Resource upsert/get | `internal/service/control_plane.go::UpsertResource/GetResource` |
| ART-0044 | RESOURCE-003 | List Resources | `internal/service/control_plane.go::ListResources` |
| ART-0045 | RESOURCE-004 | Archive Resource | `internal/service/control_plane.go::ArchiveResource` |
| ART-0046 | RESOURCE-005 | Move/Delete Resource | `internal/service/control_plane.go::MoveResource/DeleteResource` |
| ART-0047 | RESOURCE-006 | Bind/Unbind Resource | `internal/service/control_plane.go::BindResource/UnbindResource` |
| ART-0048 | RESOURCE-007 | External Resource bindings | `internal/service/control_plane.go::BindExternalResource/ListExternalResourceBindings` |

### Grant Control Plane (P1)

| ART-ID | REQ | Description | Path |
|--------|-----|-------------|------|
| ART-0049 | GRANT-001 | Role Template CRUD | `internal/service/control_plane.go::RegisterRoleTemplate/ListRoleTemplates` |
| ART-0050 | GRANT-002 | GrantAccess | `internal/service/control_plane.go::GrantAccess` |
| ART-0051 | GRANT-003 | RevokeAccess | `internal/service/control_plane.go::RevokeAccess` |
| ART-0052 | GRANT-004 | ListGrants | `internal/service/control_plane.go::ListGrants` |
| ART-0053 | GRANT-005 | ExplainAccess | `internal/service/control_plane.go::ExplainAccess` |
| ART-0054 | GRANT-006 | Grant expiry executor | `internal/biz/grant/service.go::ExpireDueGrants` + `cmd/aisphere-iam/main.go` (Dapr Job) |

### Engineering and Release (P2)

| ART-ID | REQ | Description | Path |
|--------|-----|-------------|------|
| ART-0055 | ENG-001 | Proto as source of truth | `api/iam/**/*.proto` |
| ART-0056 | ENG-002 | Kernel version alignment | `go.mod`, `Makefile` |
| ART-0057 | ENG-003 | Contract checks | `Makefile::proto-check` |
| ART-0058 | ENG-004 | Fail-closed behavior | `internal/service/*.go` (nil provider checks) |
| ART-0059 | ENG-005 | Stable error classes | `errorx` usage across service layer |
| ART-0060 | ENG-006 | Audit evidence | `internal/data/data.go::auditx.NewPostgresStore` (PostgreSQL durable sink) |
| ART-0061 | ENG-007 | Observability | `internal/server/http.go::NewHTTPServer`, `internal/server/grpc.go::NewGRPCServer` |
| ART-0062 | ENG-008 | Release evidence | CI workflow, `.agile-v/EVAL_RESULTS.md` |