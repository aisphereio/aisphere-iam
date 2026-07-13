# IAM Traceability and Verification Gaps — Cycle C1

## Severity definition

| Priority | Meaning |
|---|---|
| `P0` | Architecture/security contradiction or release-blocking behavior |
| `P1` | Missing business lifecycle, integration evidence or operational control required before release |
| `P2` | Completeness, usability or maintainability improvement |

## Gap register

### GAP-IAM-001 — Main branch contradicts the single-root Organization model

- **Priority:** `P0`
- **Affected requirements:** `REQ-IAM-PROJECT-001`, `002`, `003`, `004`, `REQ-IAM-DEPRECATED-001`
- **Observed:** architecture requires Casdoor Organization → Zone as the only root; main still exposes legacy Organization CRUD and request-controlled Project Organization/owner fields.
- **Risk:** duplicate tenant semantics, cross-domain scope injection, incompatible frontend/API behavior and stale authorization resources.
- **Current remediation:** PR #40 removes the legacy surface and derives Project scope/owner from Principal.
- **Closure evidence:** merged contract, generated routes, service/data cleanup, model guard test, clean reset acceptance and cross-Zone negative tests.

### GAP-IAM-002 — Group mutation is defined twice

- **Priority:** `P0`
- **Affected requirements:** `REQ-IAM-DIR-005`, `006`, `REQ-IAM-DECISION-002`
- **Observed:** `IAMDirectoryService` and `IAMIdentityAdminService` both define Group and membership writes using different routes and authorization-resource conventions.
- **Risk:** duplicate route registration, frontend ambiguity, inconsistent authorization and different Group identifier formats.
- **Closure evidence:** one approved canonical service; duplicate surface removed or formally deprecated; generated route uniqueness test.

### GAP-IAM-003 — Public/authorized raw relationship mutation is unresolved

- **Priority:** `P0`
- **Affected requirements:** `REQ-IAM-AUTHZ-RT-003`, `004`, `REQ-IAM-DECISION-001`, `REQ-IAM-GRANT-002`
- **Observed:** plural runtime relationship APIs are INTERNAL, but singular write/delete APIs are AUTHORIZED; admin APIs separately allow repair operations.
- **Risk:** product clients may bypass Role Template/Grant semantics and manipulate raw tuples or relation names.
- **Closure evidence:** approved exposure model, removal/restriction of unsupported routes and negative Gateway authorization tests.

### GAP-IAM-004 — Contract-only RPCs are externally visible

- **Priority:** `P1`
- **Affected requirements:** `REQ-IAM-PROJECT-005`, `006`, `REQ-IAM-RESOURCE-005`, `006`, `007`
- **Observed unimplemented methods:**
  - `UpdateProject`
  - `ArchiveProject`
  - `MoveResource`
  - `DeleteResource`
  - `UnbindResource`
  - `ListExternalResourceBindings`
- **Risk:** generated documentation and routes imply usable features that always fail at runtime.
- **Closure options:** implement and verify each approved requirement, or remove it from the release contract until scheduled.

### GAP-IAM-005 — No real Casdoor directory verification suite

- **Priority:** `P1`
- **Affected requirements:** all `REQ-IAM-DIR-*`, `REQ-IAM-AUTHN-002`
- **Missing evidence:** real User/Organization/Group reads, Group writes, membership changes, provider errors, mode boundaries and pagination.
- **Closure evidence:** reproducible Casdoor test fixture and automated HTTP/gRPC integration suite.

### GAP-IAM-006 — No real SpiceDB authorization-model verification suite

- **Priority:** `P1`
- **Affected requirements:** all `REQ-IAM-AUTHZ-*`, `REQ-IAM-PROJ-*`, `REQ-IAM-GRANT-*`
- **Missing evidence:** direct and inherited permissions, Group membersets, lookup semantics, consistency tokens, invalid schema/tuple behavior and fail-closed outage handling.
- **Closure evidence:** ephemeral SpiceDB environment loaded with the production schema and table-driven model tests.

### GAP-IAM-007 — Projection durability and concurrency are unproven

- **Priority:** `P1`
- **Affected requirements:** `REQ-IAM-PROJ-004`, `005`, `006`, `007`
- **Missing evidence:** PostgreSQL event persistence, restart recovery, duplicate submission, concurrent retry workers, DTM compensation and permanent failure handling.
- **Risk:** identity writes may succeed while authorization remains stale or is projected multiple times without clear recovery.
- **Closure evidence:** fault-injection integration tests across PostgreSQL, SpiceDB and optional DTM.

### GAP-IAM-008 — Control-plane fact/projection consistency is unproven

- **Priority:** `P1`
- **Affected requirements:** `REQ-IAM-PROJECT-003`, `REQ-IAM-RESOURCE-002`, `REQ-IAM-GRANT-002`, `003`
- **Missing evidence:** behavior when PostgreSQL succeeds and SpiceDB fails, or vice versa; retry/compensation and client-visible result semantics.
- **Closure evidence:** transaction/outbox/DTM decision documented per operation and tested with induced failures.

### GAP-IAM-009 — Gateway trust boundary lacks executable evidence

- **Priority:** `P1`
- **Affected requirements:** `REQ-IAM-AUTHN-001`, `004`, `REQ-IAM-AUTHZ-RT-008`
- **Missing evidence:** external spoofed `x-aisphere-*` headers are removed; only Gateway/IAM-produced identity reaches services; INTERNAL routes are not externally reachable.
- **Closure evidence:** deployed Envoy Gateway E2E suite covering HTTP and gRPC header spoofing and route exposure.

### GAP-IAM-010 — Audit metadata is not linked to durable audit records

- **Priority:** `P1`
- **Affected requirements:** `REQ-IAM-AUTHZ-ADMIN-005`, `REQ-IAM-ENG-006` and all high/critical mutations
- **Observed:** Proto contains audit event and risk metadata.
- **Missing evidence:** actual sink persistence, actor/target/outcome fields, correlation IDs, failure auditing, retention and queryability.
- **Closure evidence:** audit integration tests and an operator query/runbook.

### GAP-IAM-011 — Error semantics are not verified as one contract

- **Priority:** `P1`
- **Affected requirements:** `REQ-IAM-ENG-004`, `005`
- **Missing evidence:** consistent HTTP/gRPC mapping for invalid argument, unauthenticated, denied, not found, conflict, dependency failure and projection failure.
- **Closure evidence:** generated cross-transport error matrix tests.

### GAP-IAM-012 — Authorization-aware list semantics are unclear

- **Priority:** `P1`
- **Affected requirements:** Project, Resource and Grant list requirements
- **Observed:** several list services primarily call repository filters; Cycle C1 has not proven row-level authorization filtering.
- **Risk:** authenticated users may enumerate metadata outside their effective access.
- **Closure evidence:** explicit list policy and cross-Zone/cross-Project negative tests.

### GAP-IAM-013 — Pagination and filter contracts are incomplete

- **Priority:** `P2`
- **Affected requirements:** User, Project, Resource, Grant and authorization lookup lists
- **Missing evidence:** stable ordering, token invalidation, page-size limits, repeated/missing items and filter combinations.
- **Closure evidence:** common pagination contract and table-driven tests.

### GAP-IAM-014 — Grant expiry has representation but no proven enforcement

- **Priority:** `P1`
- **Affected requirement:** `REQ-IAM-GRANT-006`
- **Risk:** an expired Grant may continue to authorize if its SpiceDB relationship remains active.
- **Closure evidence:** approved expiry strategy, worker/query behavior, clock policy and effective-access tests.

### GAP-IAM-015 — Observability and readiness behavior are unverified

- **Priority:** `P1`
- **Affected requirements:** `REQ-IAM-ENG-007`, projection requirements
- **Missing evidence:** metrics for permission latency/denials/backend failures, projection backlog/retries, dependency health, alert thresholds and trace correlation.
- **Closure evidence:** dashboards, alerts, test-environment failure drills and runbooks.

### GAP-IAM-016 — Performance and reliability thresholds are undefined

- **Priority:** `P1`
- **Affected requirements:** runtime authorization and release gate
- **Missing decisions:** permission-check latency SLO, throughput, batch limits, timeout/retry budget, SpiceDB consistency policy and multi-replica capacity.
- **Closure evidence:** approved SLOs and load/soak test report.

### GAP-IAM-017 — Build success is not a release decision

- **Priority:** `P0` process gap
- **Affected requirement:** `REQ-IAM-ENG-008`
- **Observed:** CI provides strong generation/test/build/container consistency checks.
- **Missing evidence:** real dependency stack, security, audit, performance, deployment and rollback validation.
- **Closure evidence:** Gate 2 evidence summary with accepted residual risks.

## Recommended next Agile V cycles

### C2 — Approve and converge the architecture contract

1. review and merge/supersede PR #40;
2. remove legacy Organization control-plane surfaces;
3. select the canonical Group mutation service;
4. decide the raw relationship API exposure;
5. remove unapproved contract-only RPCs from the first release contract or schedule implementation.

### C3 — Build the executable IAM acceptance environment

Use ephemeral or dedicated test instances of:

- PostgreSQL;
- SpiceDB with the production schema;
- Casdoor with deterministic Organization/User/Group fixtures;
- IAM HTTP and gRPC servers;
- Envoy Gateway routes and authentication policy;
- optional DTM when projection Saga mode is enabled.

### C4 — Verify one business slice end-to-end

Recommended first vertical slice:

```text
Casdoor Organization/User/Group
→ authenticated Principal
→ create Group
→ assign User
→ projection event
→ SpiceDB group#member
→ permission check through IAM gRPC client
→ audit and observability evidence
```

This slice exercises the core differentiating IAM architecture without depending on unfinished generic Resource lifecycle operations.

### C5 — Project and Grant business closure

Verify:

```text
Principal.org_id
→ CreateProject
→ Project Zone + Owner projection
→ create/register Resource
→ Grant Role to User/Group
→ Hub-style runtime CheckPermission
→ Revoke Grant
→ permission denied
```

## Gate 2 release condition

The IAM backend remains **NOT RELEASE READY** until all `P0` gaps are closed and every release-scoped `P1` gap is either closed or explicitly accepted with owner, expiry date and mitigation.
