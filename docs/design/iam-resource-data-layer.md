# IAM Resource Control Plane Data Layer

## Goal

Phase 3 adds the persistent data layer for the Aisphere resource control plane.
It deliberately reuses Kernel database capabilities instead of creating another
ORM, migration, transaction, or connection-management stack.

## Kernel capabilities reused

- `kernel/dbx` for database opening, GORM access, query helpers, safe upsert,
  pagination, error normalization, transactions, metrics, and audit hooks.
- `kernel/dbx/postgres` as the registered Postgres driver.
- `kernel/migrationx` for SQL migration loading, validation, and apply modes.
- Existing service bootstrap lifecycle in `internal/data.NewResources`.

The IAM module only owns resource-control-plane schemas, repository methods, and
business semantics.

## Tables

| Table | Purpose |
|---|---|
| `iam_organizations` | Aisphere business tenant, mapped to Casdoor org but not identical to it. |
| `iam_projects` | Project/workspace collaboration and resource boundary. |
| `iam_capabilities` | Built-in platform capabilities such as hub, git, agent, tools, sandbox, runtime. |
| `iam_project_capabilities` | Per-project capability enablement, config, and quota. |
| `iam_resource_types` | Governed resource type registry. |
| `iam_resources` | Resource projections owned by Hub/Git/Agent/Sandbox/Runtime. |
| `iam_resource_bindings` | Cross-resource bindings, such as skill -> backing repo. |
| `iam_external_resource_bindings` | Binding to external systems such as Forgejo/Gitea/GitLab/K8s. |
| `iam_role_templates` | Product-level RBAC role templates mapped to SpiceDB relations. |
| `iam_grants` | Grant source of truth before projection to SpiceDB relationships. |
| `iam_grant_audits` | Grant/revoke audit history. |
| `iam_outbox_events` | Future reconcile/sync events for SpiceDB and external systems. |

## Runtime wiring

`internal/data.Resources` now exposes:

```go
DB           dbx.DB
ControlPlane data.ControlPlaneRepository
```

When `data.database.enabled=true`, IAM opens the DB through `dbx.New`. When
`data.migration.enabled=true`, startup runs `migrationx.Apply` using
`data.migration.config`.

Default production posture is `mode: validate` and `fail_on_pending: true`.
Developers may opt into `dev_apply` locally.

## Repository boundary

`ControlPlaneRepository` is a data-level interface for Phase 4 biz/service code.
It intentionally does not call SpiceDB. It only persists control-plane records.
SpiceDB projection belongs to the Grant/Resource business layer in Phase 4.

Important transaction rules:

- Grant creation and audit insertion happen in one `dbx.InTx` transaction.
- Grant revocation and audit insertion happen in one `dbx.InTx` transaction.
- Future outbox events should be written in the same transaction as the grant or
  resource mutation that produced them.

## Migration file

The first migration is:

```text
migrations/000001_iam_resource_control_plane.sql
```

It is goose-compatible and loaded through `kernel/migrationx`.

## Next phase

Phase 4 will add biz/service implementations:

- `CreateProject` -> DB project + SpiceDB `project#parent` / `project#owner`.
- `UpsertResource` -> DB resource projection + SpiceDB parent relationship.
- `GrantAccess` -> `iam_grants` + `iam_grant_audits` + SpiceDB relationship.
- `BindResource` -> `iam_resource_bindings` + SpiceDB cross-resource relation.
