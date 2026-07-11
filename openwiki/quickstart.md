# Aisphere IAM — OpenWiki

Aisphere IAM is the **identity directory, permission management, and authorization control plane** for the Aisphere platform. It wraps Casdoor (identity directory) and SpiceDB (ReBAC authorization) behind a unified gRPC/HTTP API surface, serving Hub, Runtime, and other Aisphere components.

## Repository at a glance

| Aspect | Summary |
|---|---|
| **Language** | Go 1.25, protobuf-generated gRPC + HTTP |
| **Module** | `github.com/aisphereio/aisphere-iam` |
| **Framework** | `github.com/aisphereio/kernel` (v0.4.x) |
| **Identity** | Casdoor (OIDC provider, user/org/group directory) |
| **Authorization** | SpiceDB (ReBAC — Relationship-Based Access Control) |
| **Database** | PostgreSQL (control-plane records, outbox events) |
| **Gateway** | Envoy Gateway (OIDC authn, header injection, routing) |
| **Deployment** | Kubernetes (Deployment, ConfigMap, Secret, NetworkPolicy, Gateway API) |

## Architecture overview

```
Browser / CLI / Agent
  ↓ HTTPS
Envoy Gateway
  ├── OIDC login (Casdoor)
  ├── JWT verification
  ├── claimToHeaders → x-aisphere-external-*
  ├── header sanitization
  └── route to upstream
       ↓
Aisphere IAM (HTTP :18080 / gRPC :19080)
  ├── Kernel middleware: requestinfo → authn → access
  ├── IAMAuthService        — token verify, GetMe
  ├── IAMDirectoryService   — user/org/group directory queries
  ├── IAMPermissionService  — SpiceDB check/write/read/lookup
  ├── IAMAuthorizationAdmin — schema management, drift repair
  ├── ProjectService        — org/project/capability control plane
  ├── ResourceService       — resource type & instance registry
  ├── GrantService          — role templates & access grants
  └── IAMIdentityAdminService — user/group CRUD (mode-aware)
       ↓
Casdoor (identity source)   SpiceDB (authorization store)   PostgreSQL (control plane)
```

## Key design decisions

1. **Gateway-trusted authentication** — IAM runs in `gateway_trusted` mode. It trusts `x-aisphere-external-*` headers injected by Envoy Gateway after OIDC login. IAM does not perform browser OAuth flows.

2. **File-backed SpiceDB schema** — The SpiceDB schema lives in `configs/spicedb/aisphere.schema.zed`. It is loaded at startup, validated, and written to SpiceDB only when the current schema is empty or missing required definitions. Schema changes are operational/configuration changes.

3. **Outbox-based authorization projection** — Identity mutations (user/group CRUD) write outbox events to PostgreSQL. A DTM-backed saga dispatches these events to SpiceDB, ensuring eventual consistency between the identity directory and the authorization graph.

4. **Proto-first API contract** — All RPCs are defined in `api/iam/v1/iam.proto` and `api/iam/v1/identity_admin.proto`. The `buf.gen.yaml` generates gRPC, HTTP (via grpc-gateway), authz policies, gateway manifests, and OpenAPI specs from these protos.

5. **Identity mode guard** — IAM supports `casdoor_local` (writable local users) and `external_oidc` (read-only identity from external IdP). The mode is configured at startup and enforced at the data layer.

## Services

| Service | Proto | Description |
|---|---|---|
| `IAMAuthService` | `iam.proto` | Token verification, GetMe (current principal) |
| `IAMDirectoryService` | `iam.proto` | Read-only user/org/group directory queries |
| `IAMPermissionService` | `iam.proto` | SpiceDB check, batch check, write/delete/read relationships, lookup resources/subjects |
| `IAMAuthorizationAdminService` | `iam.proto` | Schema read/publish, relationship drift detection, permission explain |
| `IAMIdentityAdminService` | `identity_admin.proto` | User/group CRUD (mode-aware), group membership management |
| `ProjectService` | `iam.proto` | Organization, project, capability registration and management |
| `ResourceService` | `iam.proto` | Resource type registration, resource instance upsert, cross-resource binding |
| `GrantService` | `iam.proto` | Role template registration, access grant/revoke, permission explain |

## Quick start

```bash
# Prerequisites: Go 1.25+, PostgreSQL, Casdoor, SpiceDB

# Install codegen tools
make tools

# Generate API code from protos
make api

# Run proto contract checks
make proto-check

# Run tests
make test

# Run locally (use local config)
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

Default ports: HTTP `0.0.0.0:18080`, gRPC `0.0.0.0:19080`, Metrics `127.0.0.1:19180`.

## Section pages

- [Architecture](architecture/overview.md) — System architecture, component boundaries, data flow
- [Domain](domain/overview.md) — Business domains: identity, authorization, control plane, projection
- [API](api/overview.md) — Proto contracts, access policies, generated code
- [Operations](operations/overview.md) — Deployment, configuration, secrets, Kubernetes manifests
- [Integrations](integrations/overview.md) — Casdoor, SpiceDB, Kernel, Envoy Gateway, DTM
- [Testing](testing/overview.md) — Test strategy, contract tests, identity projection tests
- [Source Map](source-map.md) — Directory layout and key files