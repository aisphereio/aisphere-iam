# Domain Overview

Aisphere IAM spans four business domains: **Identity**, **Authorization**, **Control Plane**, and **Projection**.

## 1. Identity Domain

**Purpose:** Manage users, organizations, and groups as the identity directory.

**Backend:** Casdoor (OIDC provider + identity directory)

**Services:**
- `IAMDirectoryService` — Read-only queries: `GetUser`, `ListUsers`, `GetOrganization`, `ListGroups`, `GetGroup`
- `IAMIdentityAdminService` — User/group CRUD: `CreateUser`, `UpdateUser`, `DisableUser`, `DeleteUser`, `CreateGroup`, `UpdateGroup`, `DeleteGroup`, `AssignUserToGroup`, `RemoveUserFromGroup`
- `IAMAuthService` — `VerifyToken`, `GetMe` (current principal)

**Identity modes:**

| Mode | Behavior |
|---|---|
| `casdoor_local` (default) | Full CRUD on users and groups via Casdoor Admin API |
| `external_oidc` | User/org writes rejected; local application groups remain writable |

The mode is configured via `security.authn.identity_mode` and enforced in `internal/data/identity_mode.go`.

**Key source files:**
- `internal/service/iam.go` — `IAMDirectoryService`, `IAMAuthService`
- `internal/server/identity_admin_auto.go` — `IAMIdentityAdminService` wiring
- `internal/data/identity_mode.go` — Mode guard, projection dispatcher
- `api/iam/v1/identity_admin.proto` — Identity admin API contract

## 2. Authorization Domain

**Purpose:** Define, check, and manage ReBAC relationships in SpiceDB.

**Backend:** SpiceDB (authzed/spicedb)

**Services:**

| Service | Key RPCs |
|---|---|
| `IAMPermissionService` | `CheckPermission`, `BatchCheckPermissions`, `WriteRelationships`, `DeleteRelationships`, `ReadRelationships`, `LookupResources`, `LookupSubjects` |
| `IAMAuthorizationAdminService` | `ReadSchema`, `PublishSchema`, `ExplainPermission`, `CheckRelationshipDrift` |

**SpiceDB schema** (`configs/spicedb/aisphere.schema.zed`):
- Definitions: `user`, `service`, `service_account`, `group`, `zone`, `iam_authz`, `iam`
- Groups have relations: `zone`, `parent`, `member`, `owner`, `manager`, `viewer`, `permission_admin`
- Zones have relations: `owner`, `admin`, `user_viewer`, `user_manager`, `group_viewer`, `group_manager`, `permission_admin`, `member`
- Permissions cascade through `parent` and `zone` relations

**Role mapping:**

| Role key | Resource | SpiceDB Relation |
|---|---|---|
| `zone_owner` | zone | `owner` |
| `zone_admin` | zone | `admin` |
| `user_viewer` | zone | `user_viewer` |
| `user_manager` | zone | `user_manager` |
| `group_viewer` | zone | `group_viewer` |
| `group_manager` | zone | `group_manager` |
| `permission_admin` | zone | `permission_admin` |
| `group_owner` | group | `owner` |
| `group_manager` | group | `manager` |
| `group_viewer` | group | `viewer` |
| `group_member` | group | `member` |

**Key source files:**
- `internal/service/iam.go` — `IAMPermissionService`, `IAMAuthorizationAdminService`
- `internal/data/authz_bootstrap.go` — Schema bootstrap
- `configs/spicedb/aisphere.schema.zed` — SpiceDB schema
- `client/authzgrpc/client.go` — gRPC client for other services to call IAM permission checks

## 3. Control Plane Domain

**Purpose:** Manage Aisphere organizations, projects, capabilities, resources, and grants as first-class control-plane records.

**Backend:** PostgreSQL (via GORM)

**Services:**

| Service | Key RPCs | Biz Package |
|---|---|---|
| `ProjectService` | `CreateOrganization`, `CreateProject`, `RegisterCapability`, `SetProjectCapability` | `internal/biz/project/` |
| `ResourceService` | `RegisterResourceType`, `UpsertResource`, `BindResource`, `BindExternalResource` | `internal/biz/resource/` |
| `GrantService` | `RegisterRoleTemplate`, `GrantAccess`, `RevokeAccess`, `ExplainAccess` | `internal/biz/grant/` |

### Organization hierarchy

```
Organization (zone)
  └── Project
       └── Capabilities (enabled/disabled per project)
```

### Resource model

Resources have types (e.g., `zone`, `group`, `repository`, `agent`), can have parent-child relationships, and can be bound to external resources (e.g., a Git repository).

### Grant model

Grants are role-based access assignments:
- **Role templates** define a role key → SpiceDB relation mapping per resource type
- **Grants** assign a subject to a role on a resource, with optional expiration
- Grants are projected to SpiceDB relationships via the outbox mechanism

### Defaults reconciliation

At startup, `configs/resource/defaults.yaml` is reconciled into IAM DB:
- Built-in capabilities: `iam`, `hub`, `git`, `agent`, `tools`, `sandbox`, `runtime`
- Built-in resource types: `zone`, `group`, `project`, `repository`, `agent`, `tool`, `sandbox`, `runtime`
- Built-in role templates

**Key source files:**
- `internal/biz/project/service.go` — Organization/project/capability use cases
- `internal/biz/resource/service.go` — Resource type and instance use cases
- `internal/biz/grant/service.go` — Role template and grant use cases
- `internal/biz/defaults/loader.go` — Defaults file loading and reconciliation
- `configs/resource/defaults.yaml` — Built-in defaults
- `migrations/000001_iam_resource_control_plane.sql` — Control plane tables

## 4. Projection Domain

**Purpose:** Ensure eventual consistency between IAM control-plane records and SpiceDB authorization relationships.

**Mechanism:** Outbox event table + DTM saga

### Outbox event flow

1. Domain service (project, resource, grant, identity) creates/modifies a record
2. The service calls `projection.Manager.NewWriteEvent()` or `NewDeleteEvent()`
3. The event is persisted to `iam_outbox_events` table
4. `projection.Manager.Dispatch()` submits a DTM saga
5. DTM calls the `/internal/dtm/iam/projection/apply` HTTP endpoint
6. The projection manager reads the event payload and writes/deletes SpiceDB relationships
7. Event status is updated to `synced` or `failed`

### Identity projection

Identity projection is a separate path for Casdoor directory mutations:

1. `IdentityProjectionDispatcher` writes events to `iam_directory_projection_events`
2. Events are dispatched via DTM saga
3. `BuildDirectoryProjectionRelationships()` computes the desired SpiceDB state from Casdoor
4. `DetectDirectoryProjectionDrift()` compares desired vs actual state

### Projection operations (HTTP admin endpoints)

| Endpoint | Purpose |
|---|---|
| `POST /internal/dtm/iam/projection/apply` | DTM saga branch: apply a projection event |
| `POST /internal/dtm/iam/projection/compensate` | DTM saga branch: compensate a failed projection |
| `POST /v1/iam/directory-projection/retry` | Retry failed projection events |
| `POST /v1/iam/directory-projection/reconcile` | Reconcile all relationships for an org |
| `GET /v1/iam/directory-projection/drift` | Check for drift between desired and actual relationships |

**Key source files:**
- `internal/biz/projection/manager.go` — Outbox event → SpiceDB projection
- `internal/data/identity_mode.go` — Identity projection dispatcher
- `internal/service/directory_projection.go` — Projection operations service
- `internal/biz/graph/projector.go` — ReBAC relationship projection adapter