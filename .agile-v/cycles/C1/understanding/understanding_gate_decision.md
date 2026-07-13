# IAM Understanding Gate Decision

## Gate metadata

- Gate: `Gate 0 — Existing System Understanding`
- Cycle: `C1`
- Baseline: `main@46c8785861392c15388b250e6ae6c245efb6bdc9`
- Last updated: `main@653afc0` (PR #40 merged — legacy Organization surface removed)
- Decision date: 2026-07-13
- Knowledge graph available: **No**
- System overview generated: **Yes**
- Confidence: **Medium**

## Decision

# PASS_WITH_FINDINGS

The system is sufficiently understood to begin **candidate requirement recovery and traceability work**. It is not sufficiently verified to approve production release or to treat every existing API as an accepted product requirement.

## Blocking findings for release readiness

### F-001 — Architecture and main-branch API contract conflict

✅ **CLOSED** — PR #40 merged. See detailed resolution below.

~~The accepted architecture defines Casdoor Organization as the single identity-domain root and forbids a second IAM Organization model. Current main still exposes and implements legacy Organization control-plane CRUD and allows CreateProject scope to be supplied by the request.~~

- **Status:** ✅ **CLOSED** — PR #40 merged at `0425275`
- **Remediation:** `api/iam/project/v1/project.proto` — Organization CRUD RPCs removed; `internal/biz/project/service.go` deleted; `internal/service/control_plane.go` derives Project scope/owner from `authn.Principal` via `currentProjectContext`; `internal/data/memory.go` and `internal/data/resource_repository.go` Organization methods cleaned up; Grant/Resource services reject `organization` type with error; `model_contract_test.go` enforces single-root Zone model.
- **Residual risk:** `internal/data/resource_models.go` User model still contains an `Organization` field (user's org affiliation, not a second Organization model — acceptable).

### F-002 — Contract surface exceeds implemented behavior

The following main-branch RPC implementations explicitly return `Unimplemented`:

- `ProjectService.UpdateProject`
- `ProjectService.ArchiveProject`
- `ResourceService.MoveResource`
- `ResourceService.DeleteResource`
- `ResourceService.UnbindResource`
- `ResourceService.ListExternalResourceBindings`

**Impact:** these operations are `CONTRACT_ONLY` or `PARTIAL_IMPLEMENTATION`, not completed capabilities.

### F-003 — Overlapping Group mutation contracts

Both `IAMDirectoryService` and `IAMIdentityAdminService` define Group mutation and membership behavior, with overlapping routes and different policy/resource conventions.

**Impact:** the canonical Group write service and route contract must be selected before the corresponding requirements can be approved.

### F-004 — Runtime integration evidence is missing

Observed tests primarily use in-memory stores, fake clients or source-contract guards. C1 has not yet collected executable evidence against:

- a real Casdoor instance;
- a real SpiceDB instance;
- PostgreSQL transactions and projection event recovery;
- Envoy Gateway route exposure and trusted identity-header cleaning;
- multi-replica retry/idempotency behavior.

**Impact:** no business domain is currently marked `RELEASE_READY` by Agile V.

### F-005 — Audit and observability evidence is contractual, not operational

Proto policies declare audit events and risk levels, but C1 has not confirmed that events are persisted, queryable and correlated with request/trace/decision identifiers.

**Impact:** high-risk and critical operations require an operational audit verification suite.

## Non-blocking positive findings

- The service has explicit architecture boundaries and strong invariants.
- Proto is used as the contract source for HTTP/gRPC, access policy, audit metadata and Gateway manifests.
- CI performs generation, contract checks, Go tests, binary build, generated drift checks and container build.
- The runtime authorization API exposes check, batch check, relationship operations and graph lookup.
- Durable identity projection events, retries and optional DTM orchestration are implemented.
- Existing tests guard Principal extraction, Group projection structure, root-model invariants and runtime client adaptation.
- PR #40 currently has successful CI and image workflow results.

## Allowed next actions

- generate and review candidate `REQ-IAM-*` requirements;
- build an implementation traceability matrix;
- classify APIs as implemented, partial, contract-only, deprecated or architecture-required;
- design verification cases and integration environments;
- continue read-only analysis.

## Actions not authorized by this gate

- declaring production readiness;
- changing business behavior without Gate 1 requirement approval;
- marking an RPC complete based only on generated transport code.

## Gate 1 prerequisites

Before candidate requirements become approved requirements:

1. select the canonical Group write API;
2. decide whether singular public relationship mutation APIs remain supported or are internal/admin-only;
3. review requirement priorities and release scope;
4. approve the verification standard for each domain.
