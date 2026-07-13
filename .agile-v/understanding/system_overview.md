# IAM System Overview — Gate 0

## Source

- Repository: `aisphereio/aisphere-iam`
- Baseline: `main@46c8785861392c15388b250e6ae6c245efb6bdc9`
- Last updated: `main@653afc0` (PR #40 merged — legacy Organization surface removed)
- Generated at: 2026-07-13
- Knowledge graph: not available
- Method: README/architecture review, Proto inspection, implementation inspection, selected tests, CI and recent PR history

## Summary

Aisphere IAM is a Go service built on `github.com/aisphereio/kernel`. It adapts the Casdoor identity directory, persists IAM control-plane facts in PostgreSQL, projects authorization relationships into SpiceDB, and exposes HTTP/gRPC APIs for directory operations, permission decisions, projects, generic resources and grants.

The intended root model is one Casdoor Organization mapped 1:1 to a SpiceDB `zone`. PR #40 has been merged (`0425275`), removing the legacy second Organization control-plane model. The main branch now derives Project scope and owner from the authenticated Kernel Principal.

## Architectural layers

| Layer | Main locations | Responsibility |
|---|---|---|
| API contract | `api/iam/**.proto` | HTTP/gRPC surface, request validation, access policy and audit metadata |
| Generated transport | `api/iam/**/*_http.pb.go`, `*_grpc.pb.go`, `*_gateway.pb.go`, `*_kernel.pb.go` | Transport and Kernel integration generated from Proto |
| Service adapter | `internal/service/` | Proto conversion, Principal extraction, provider delegation and error mapping |
| Domain/business | `internal/biz/project`, `internal/biz/resource`, `internal/biz/grant` | Control-plane use cases and relationship intent |
| Data/integration | `internal/data/` | Casdoor mode wrapper, PostgreSQL repositories, projection events, SpiceDB interaction |
| Runtime client | `client/authzgrpc/` | Kernel-compatible authorization client for Hub/Runtime and other services |
| Authorization model | `configs/spicedb/aisphere.schema.zed` | ReBAC definitions, relations and permissions |
| Bootstrap catalogue | `configs/resource/defaults.yaml` | Resource types and role templates |
| Runtime wiring | `cmd/aisphere-iam`, `internal/server/` | Dependency construction, service registration and server startup |
| Deployment | `deploy/`, generated Gateway API manifests, Dockerfile | Kubernetes/Gateway deployment and image build |
| Verification | tests, `Makefile`, `.github/workflows/ci.yml` | generation, contract, test, build, drift and container gates |

## External dependencies

| Dependency | Role | Failure implication |
|---|---|---|
| Envoy Gateway | external OIDC/JWT boundary and trusted Principal header injection | IAM must reject requests without an authenticated Kernel Principal where required |
| Casdoor | Organization/User/Group identity-directory fact source | directory reads/writes fail; IAM must not fabricate identity facts |
| SpiceDB | permission calculation and relationship projection | authorization must fail closed; projection failures require retry and visibility |
| PostgreSQL | IAM control-plane fact source and projection event store | control-plane operations and durable projection recovery fail |
| Kernel | authn/authz abstractions, middleware, code generators, transport and policy contracts | runtime and generated code versions must stay aligned |
| DTM | optional distributed projection orchestration | when enabled, projection branches require submit/apply/compensate correctness |

## Main service surfaces

### IAMAuthService

| Operation | Exposure | Observed behavior |
|---|---|---|
| `VerifyToken` | INTERNAL | delegates token verification to the configured Kernel `TokenService` |
| `GetMe` | AUTHENTICATED | returns the normalized Principal from Kernel Context; rejects missing/unauthenticated Principal |

IAM does not own browser session management or OAuth callback state in the target architecture.

### IAMDirectoryService

| Capability | Observed behavior |
|---|---|
| user read/list | checks `zone:<org_id>#view_users`, then delegates to the identity provider |
| Organization metadata read | checks `zone:<org_id>#view_zone`, then reads Casdoor Organization metadata |
| Group read/list | list checks `zone:<org_id>#view_groups`; Group APIs are backed by the identity provider |
| Group mutation | Proto defines create/update/delete and membership operations |
| membership mutation | assign/remove operations exist; projection is expected to update SpiceDB membership relationships |

### IAMIdentityAdminService

Proto defines generated identity management for:

- create/update/disable/delete User;
- create/update/delete Group;
- assign/remove User membership.

The identity mode wrapper supports:

- `casdoor_local`: provider writes are allowed;
- `external_oidc`: upstream user/Organization mutation is blocked while Aisphere-owned Group and membership mutation remains allowed.

There is overlap between `IAMDirectoryService` Group mutations and `IAMIdentityAdminService` Group mutations. This must be resolved as a single product contract.

### IAMDirectoryProjectionService

The API exposes:

- retry pending/failed projection events;
- reconcile the desired directory projection;
- detect projection drift.

The data layer persists projection events in `iam_directory_projection_events`, records state transitions, supports retry, and can submit DTM Saga branches when DTM is enabled.

### IAMPermissionService

Runtime/data-plane authorization capabilities include:

- single permission check;
- batch permission check;
- plural relationship write/delete/read for trusted services;
- legacy singular relationship write/delete surface;
- resource lookup by subject and permission;
- subject lookup by resource and permission.

The `client/authzgrpc` package adapts this service to Kernel authorization interfaces and propagates either the current user Principal or an explicit service Principal as trusted gRPC metadata.

### IAMAuthorizationAdminService

Administrative capabilities include:

- read, validate and publish the SpiceDB schema;
- list, write and delete relationships;
- check and explain authorization decisions;
- calculate a permission map for a subject/resource pair.

The implementation performs an explicit global authorization check against `iam_authz:global` before administrative operations.

### ProjectService

The Proto currently exposes:

- Project create/get/list/update/archive;
- Capability register/list;
- Project Capability enable/disable/list.

Legacy Organization CRUD has been removed by PR #40 (`0425275`). Project scope and owner now derive from the authenticated Kernel Principal via `currentProjectContext`. `UpdateProject` and `ArchiveProject` have been implemented, completing the Project CRUD surface. `GetProject` now validates Zone permission to prevent cross-zone data leakage.

### ResourceService

Implemented or partially implemented capabilities include:

- register/get/list Resource Type;
- upsert/get/list/archive Resource;
- bind/list Resource Binding;
- bind external resource.

Observed main-branch gaps:

- `MoveResource` is unimplemented;
- `DeleteResource` is unimplemented;
- `UnbindResource` is unimplemented;
- `ListExternalResourceBindings` is unimplemented.

### GrantService

Observed capabilities include:

- register/list Role Templates;
- grant access and project a relationship;
- revoke a Grant and delete the graph relationship;
- list Grants;
- explain access through the authorization provider.

## Primary domain flows

### Authenticated profile flow

```text
Client
→ Envoy Gateway validates Casdoor JWT
→ Kernel restores authn.Principal from trusted gateway data
→ IAM GetMe reads Principal from Context
→ response returns normalized Principal
```

### Directory read flow

```text
Authenticated Principal
→ IAM checks zone permission in SpiceDB
→ IAM calls Casdoor directory adapter
→ IAM maps provider object to Proto response
```

### Group/membership projection flow

```text
Authorized Group or membership mutation
→ identity provider mutation
→ desired zone/parent/member relationships calculated
→ durable projection event recorded when DB is configured
→ DTM branch or direct SpiceDB relationship write
→ event marked synced or failed
→ retry/reconcile/drift operations provide repair path
```

### Runtime permission decision flow

```text
Trusted backend service
→ client/authzgrpc propagates user or service Principal metadata
→ IAMPermissionService
→ SpiceDB check/batch/lookup
→ decision and consistency token returned
```

### Control-plane create flow

```text
Authenticated Principal
→ service extracts actor from Kernel Context
→ biz operation persists PostgreSQL fact
→ corresponding structural/ownership relationship projected to SpiceDB
→ response returned with consistency evidence where supported
```

The Project main-branch implementation now satisfies the target scoping invariant: scope and owner derive from the authenticated Kernel Principal, and the legacy Organization model has been removed.

### Grant flow

```text
Authenticated Principal
→ role template resolves to SpiceDB relation
→ Grant fact persisted
→ relationship written to SpiceDB
→ Grant returned with consistency token
```

Revocation should update the Grant fact and remove the projected relationship.

## Existing verification evidence observed

- CI installs the Kernel-matched generators, regenerates API/deploy output, runs contract checks, all Go tests, binary build, generated drift check and Docker build.
- `principal_context_test.go` verifies Kernel Principal extraction and owner fallback helpers.
- `identity_mode_test.go` verifies zone-qualified Group, parent and User membership relationships.
- `model_contract_test.go` prevents the old SpiceDB `organization` root from returning and requires Project → Zone projection.
- `client/authzgrpc/client_test.go` verifies Principal propagation and core runtime client adaptation using a fake gRPC client.
- PR #40 merged at `0425275` with successful CI and Docker workflow results. The legacy Organization model is now removed from main.

## Known constraints and cautions

1. PostgreSQL is the control-plane fact source; SpiceDB is a query projection, not a business metadata source.
2. Permission-provider failures must fail closed.
3. Principal identity must come from Kernel Context, not request-owned actor fields or ordinary external headers.
4. Project must belong to exactly one Casdoor-derived Zone.
5. Group identity is Organization-qualified when projected to SpiceDB.
6. Generated artifacts are committed and CI rejects generation drift.
7. Proto contracts currently include historical/overlapping surfaces; generated existence is not proof of a valid product requirement.
8. Several service methods return `Unimplemented`; the API surface is broader than the completed business surface.
9. Most observed tests are unit/contract tests with fakes or source-string guards. Real Casdoor/SpiceDB/PostgreSQL/Gateway evidence is still required.
10. The platform is not live; architecture documents allow destructive reset instead of compatibility migration for the removed Organization model. PR #40 has removed the legacy model; a clean reset is expected.

## Unknowns requiring later evidence

- complete inventory of all tests and their runtime dependency coverage;
- actual PostgreSQL transaction boundaries for every create/grant/projection operation;
- behavior when PostgreSQL succeeds but SpiceDB projection fails;
- retry idempotency across process restart and multi-replica execution;
- actual Gateway header-cleaning and internal-route isolation behavior;
- audit event persistence and queryability;
- error mapping consistency across HTTP and gRPC;
- pagination correctness and filtering semantics;
- load and latency characteristics of permission checks;
- rollback and schema compatibility behavior.

## Confidence

- Confidence: **Medium**
- Reason: the main API and implementation layers are identifiable and recent PR history is detailed, but no complete repository knowledge graph was available and runtime integration evidence has not yet been collected.
