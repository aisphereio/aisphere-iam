# Architecture

## System boundaries

```
Casdoor        = External identity source / OIDC Provider / User & Organization directory
Envoy Gateway  = Platform's sole external authn boundary (OIDC login, JWT validation, claimToHeaders)
Aisphere IAM   = Casdoor directory adapter + Aisphere identity projection + SpiceDB ReBAC control plane
SpiceDB        = Final authorization fact store (ReBAC relationships)
PostgreSQL     = IAM control-plane records (orgs, projects, capabilities, resources, outbox events)
```

**IAM no longer handles browser OAuth code exchange, refresh, or session management.** These are handled by Envoy Gateway's OIDC SecurityPolicy.

## Authentication flow

1. User authenticates via Envoy Gateway OIDC (Casdoor)
2. Gateway validates JWT, extracts claims, sanitizes headers
3. Gateway injects `x-aisphere-external-*` headers
4. IAM Kernel middleware restores `authn.Principal` from these headers
5. Business code calls `authn.PrincipalFromContext(ctx)` to get the caller

### Authn modes

| Mode | Description |
|---|---|
| `gateway_trusted` (default) | Trust Envoy Gateway injected headers. Internal token disabled. |
| `casdoor_jwt` | Direct Casdoor JWT verification (local dev fallback) |
| `oidc_jwt` | Generic OIDC JWT verification |

## Authorization model

IAM uses **ReBAC (Relationship-Based Access Control)** via SpiceDB:

```
subject + resource + permission → boolean decision
```

Example checks:
```
user:<casdoor-sub> can create_groups on zone:aisphere
user:<casdoor-sub> can manage on group:aisphere/platform
group:aisphere/platform#member can group_manager on zone:aisphere
```

### SpiceDB schema definitions

The schema (`configs/spicedb/aisphere.schema.zed`) defines:

- **user** — Individual user subject
- **service** — Service-to-service subject
- **service_account** — Machine account subject
- **group** — Casdoor group mapped to an availability zone. Has relations: `zone`, `parent`, `member`, `owner`, `manager`, `viewer`, `permission_admin`
- **zone** — Casdoor organization as top-level availability zone. Has relations: `owner`, `admin`, `user_viewer`, `user_manager`, `group_viewer`, `group_manager`, `permission_admin`, `member`
- **iam_authz** — Global IAM authorization control resource
- **iam** — Legacy IAM resource (kept for compatibility)

## Identity projection

Identity mutations (user/group CRUD) are projected to SpiceDB via an outbox pattern:

1. Identity mutation occurs in Casdoor
2. IAM writes an outbox event to `iam_directory_projection_events` table
3. A DTM saga dispatches the event
4. The projection manager applies the relationship write/delete to SpiceDB
5. Event status transitions: `pending` → `submitted` → `projecting` → `synced` (or `failed`)

## Middleware chain

Both HTTP and gRPC servers use the same middleware stack:

```
requestinfo.Server → authn.Server (allow anonymous) → access.Server (accessx.Guard)
```

The `access.Server` uses a skip-policy resolver that:
- Skips all checks for healthz, readyz, metrics, ExternalAuthorize
- Skips authz (keeps authn + audit) for directory read operations (self-checked in service layer)
- Skips authz for IAMPermissionService operations (the service itself is the authorization decision point)
- Skips authz for operations labeled `authz_mode: SELF_CHECK`
- Skips authz for `CreateOrganization` (no org exists yet)
- Applies full authz for all other AUTHORIZED operations

## Deployment architecture

```
User browser → HTTPS :30723 → Envoy Gateway
  ├── / → aisphere-iam-frontend:3000 (SPA)
  └── /v1/iam/* → aisphere-iam:18080 (API)

NetworkPolicy restricts IAM ingress to:
  - Envoy Gateway pods (HTTP + gRPC)
  - Same-namespace pods (DTM callbacks)
  - Prometheus (metrics)
```

## Key source files

| File | Purpose |
|---|---|
| `cmd/aisphere-iam/main.go` | Application entrypoint, dependency wiring |
| `internal/server/access.go` | Middleware chain, skip policy resolver |
| `internal/server/http.go` | HTTP server construction, route registration |
| `internal/server/grpc.go` | gRPC server construction |
| `internal/data/data.go` | Resource initialization (DB, authn, authz, cache, etc.) |
| `internal/data/authz_bootstrap.go` | SpiceDB schema bootstrap at startup |
| `internal/data/identity_mode.go` | Identity mode guard, projection dispatcher |
| `internal/biz/projection/manager.go` | Outbox event → SpiceDB projection manager |
| `internal/biz/graph/projector.go` | ReBAC relationship projection adapter |