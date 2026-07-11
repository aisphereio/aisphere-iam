# Integrations

## Casdoor

**Role:** External identity source / OIDC Provider / User & Organization directory

**Integration points:**
- `authn/casdoor` (Kernel package) — Login, token verification, user/org/group directory queries
- `security.authn.casdoor` config — Endpoint, client ID/secret, organization name, application name
- `security.authn.oidc` config — OIDC discovery, JWKS URL, audience, allowed algorithms

**M2M Admin access:**
- Used for bootstrap admin resolution and directory management
- Configured via `security.authn.casdoor.admin` (client_id, client_secret)
- Resolves usernames to stable Casdoor UUIDs

**Identity modes:**
- `casdoor_local` — Full CRUD via Casdoor Admin API
- `external_oidc` — Read-only identity from external IdP

**Key config:**
```yaml
security:
  authn:
    provider: casdoor
    casdoor:
      endpoint: http://127.0.0.1:8000
      organization_name: aisphere
      application_name: aisphere
      client_id: CHANGE_ME_CLIENT_ID
```

## SpiceDB

**Role:** ReBAC authorization fact store

**Integration points:**
- `authz/spicedb` (Kernel package) — Relationship read/write/check, schema management
- `security.authz.spicedb` config — Endpoint, preshared key
- Schema file: `configs/spicedb/aisphere.schema.zed`

**Schema bootstrap:**
- IAM loads the schema at startup when `security.authz.install_default_schema: true`
- Startup is idempotent: reads existing schema, skips if required definitions exist
- Schema is validated before writing

**Key source:**
```yaml
security:
  authz:
    enabled: true
    provider: spicedb
    install_default_schema: true
    schema_path: configs/spicedb/aisphere.schema.zed
    spicedb:
      endpoint: 127.0.0.1:50051
      preshared_key: CHANGE_ME
```

**Schema change policy:**
- Additive changes (new definitions, relations, permissions) are usually safe
- Destructive changes (rename/remove) require a migration plan
- For production upgrades, prefer the IAM authorization admin publish API over automatic restart-time overwrite

## Kernel Framework

**Role:** Application framework providing infrastructure abstractions

**Integration points:**

| Kernel Package | IAM Usage |
|---|---|
| `configx` | Config file + env var loading |
| `logx` | Structured logging, access log |
| `metricsx` | Prometheus metrics |
| `dtmx` | Distributed transaction manager (DTM) |
| `authn/casdoor` | Casdoor identity adapter |
| `authz/spicedb` | SpiceDB authorization adapter |
| `accessx` | Combined authn/authz/audit guard |
| `transportx/http` | HTTP server with middleware |
| `transportx/grpc` | gRPC server with middleware |
| `serverx` | Service catalog, runtime providers |
| `middleware/access` | Access control middleware |
| `securityx` | Security runtime (authn modes) |
| `dbx` | Database abstraction (PostgreSQL via pgx) |
| `migrationx` | Database migration (goose) |
| `cachex` | Cache abstraction (Redis) |
| `objectstorex` | Object store abstraction (MinIO) |
| `dtmx` | DTM distributed transaction client |
| `grpcx` | gRPC client configuration |

**Kernel compliance** (`docs/kernel-compliance.md`):
- IAM server assembles `requestinfo → authn → access` middleware on both HTTP and gRPC
- Resource-level authorization works even without Gateway (direct access to IAM's internal ports)
- Future: `protoc-gen-go-kernel` will generate module-scoped fallback resolvers

## Envoy Gateway

**Role:** Platform's sole external authentication boundary

**Integration points:**
- OIDC SecurityPolicy — Casdoor OIDC login and JWT verification
- claimToHeaders — Extracts identity claims into `x-aisphere-external-*` headers
- Header sanitization — Strips client-forged `x-aisphere-*` and `x-internal-*` headers
- HTTPRoute — Routes `/v1/iam/*` to IAM backend

**Gateway configuration:**
- GatewayClass: `aisphere-gateway-class` (controller: `gateway.envoyproxy.io/gatewayclass-controller`)
- Gateway: `aisphere-gateway` (namespace: `aisphere`)
- Listeners: HTTP (port 80), HTTPS (port 443, hostname `*.weagent.cc`)

**IAM trust model:**
- IAM runs in `gateway_trusted` mode
- Kernel middleware restores `authn.Principal` from `x-aisphere-external-*` headers
- Internal token is disabled; trust is provided by NetworkPolicy isolation

## DTM (Distributed Transaction Manager)

**Role:** Saga coordinator for outbox event → SpiceDB projection

**Integration points:**
- `dtmx` (Kernel package) — DTM client for saga submission
- HTTP branch endpoints: `/internal/dtm/iam/projection/apply`, `/internal/dtm/iam/projection/compensate`
- Topic: `iam.authz.projection`

**Flow:**
1. Domain service creates outbox event
2. `projection.Manager.Dispatch()` creates a DTM saga
3. DTM calls the apply branch to write SpiceDB relationships
4. On failure, DTM calls the compensate branch

**Configuration:**
```yaml
dtm:
  enabled: true
  endpoint: http://dtm.aisphere:36789
  branch_secret: CHANGE_ME
```

## PostgreSQL

**Role:** IAM control-plane database

**Integration points:**
- `dbx` (Kernel package) — Database connection via pgx
- GORM — ORM for model queries
- `migrationx` — Goose-based migrations

**Tables:**
- `iam_organizations`, `iam_projects`, `iam_capabilities`, `iam_project_capabilities`
- `iam_resource_types`, `iam_resources`, `iam_resource_bindings`, `iam_external_resource_bindings`
- `iam_role_templates`, `iam_grants`
- `iam_outbox_events`, `iam_directory_projection_events`
- `iam_local_users` (legacy)

## Key source files

| File | Integration |
|---|---|
| `internal/data/data.go` | All integration initialization |
| `internal/data/authz_bootstrap.go` | SpiceDB schema bootstrap |
| `internal/data/identity_mode.go` | Casdoor identity mode guard |
| `internal/biz/projection/manager.go` | DTM saga for outbox projection |
| `client/authzgrpc/client.go` | gRPC client for external services |
| `configs/spicedb/aisphere.schema.zed` | SpiceDB schema |
| `docs/envoy-casdoor-oidc.md` | Envoy Gateway OIDC integration |
| `docs/iam-authz-spicedb-bootstrap.md` | SpiceDB bootstrap model |