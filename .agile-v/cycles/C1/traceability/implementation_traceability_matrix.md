# IAM Implementation Traceability Matrix — Cycle C1

## Interpretation

This matrix links recovered candidate requirements to implementation and currently observed evidence.

- `Verified` means only that the cited evidence verifies the stated narrow behavior.
- `Observed` means implementation was found but has not been executed against the real dependency stack in Cycle C1.
- `Partial` means a business path, lifecycle operation or evidence layer is incomplete.
- `Conflict` means current main differs from the accepted architecture.
- `Contract only` means the API exists but executable behavior is absent or explicitly unimplemented.
- No row is considered production-release evidence unless it includes real integration and operational evidence.

## Authentication and Principal

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-AUTHN-001` | `IAMAuthService.GetMe` | `internal/service/iam.go::GetMe`; Kernel `PrincipalFromContext` | `internal/service/principal_context_test.go` verifies Context extraction and missing Principal helper behavior | `Verified (unit scope)` |
| `REQ-IAM-AUTHN-002` | `IAMAuthService.VerifyToken`; `POST /v1/iam/auth/verify` | `internal/service/iam.go::VerifyToken` delegates to `authn.TokenService`; Proto marks INTERNAL | no real Casdoor token test identified | `Observed` |
| `REQ-IAM-AUTHN-003` | Project/Resource/Grant mutations | `internal/service/control_plane.go` calls `currentProjectContext`, `currentResourceSubject`, `currentGrantSubject` | `principal_context_test.go` verifies actor/owner fallback helpers and rejects missing org_id | `Verified (unit scope)`; PR #40 removed request owner/scope |
| `REQ-IAM-AUTHN-004` | Envoy Gateway OIDC and trusted identity propagation | `README.md`, `docs/architecture-boundaries.md`, Kernel authn mode/config | generated/deployment presence observed; no Gateway spoofing E2E evidence | `Architecture required` |

## Identity directory

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-DIR-001` | `IAMDirectoryService.GetUser` | `internal/service/iam.go::GetUser` and `requireZonePermission(view_users)` | no real Casdoor/SpiceDB test identified | `Observed` |
| `REQ-IAM-DIR-002` | `IAMDirectoryService.ListUsers` | `internal/service/iam.go::ListUsers`; provider filter mapping | no pagination/filter integration evidence identified | `Observed` |
| `REQ-IAM-DIR-003` | `IAMDirectoryService.GetOrganization` | `internal/service/iam.go::GetOrganization`; `view_zone` check | architecture contract documents read-only Casdoor metadata | `Observed`; target meaning is clear |
| `REQ-IAM-DIR-004` | `IAMDirectoryService.ListGroups` | `internal/service/iam.go::ListGroups`; `view_groups` check | no Group-tree provider test identified | `Observed` |
| `REQ-IAM-DIR-005` | Group create/update/delete | `api/iam/v1/iam.proto` and `api/iam/v1/identity_admin.proto` both define mutations | generated contracts exist; no canonical-surface regression test | `Conflict / Partial` |
| `REQ-IAM-DIR-006` | assign/remove User membership | both Proto services expose operations; provider/projection wrapper exists in `internal/data/identity_mode.go` | `identity_mode_test.go` verifies resulting member relationship, not real provider mutation | `Partial` |
| `REQ-IAM-DIR-007` | `security.authn.identity_mode` | `internal/data/identity_mode.go::identityMode`, `identityForMode`, `externalOIDCIdentityAdmin` | selected projection test exists; complete mode matrix not identified | `Observed` |

## Directory projection

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-PROJ-001` | Group create/update projection | `internal/data/identity_mode.go`; Organization-qualified Group relationship construction | `internal/data/identity_mode_test.go` requires `group:aisphere/platform#zone@zone:aisphere` | `Verified (unit scope)` |
| `REQ-IAM-PROJ-002` | child Group projection | same projection wrapper | same test requires `parent@group:aisphere/root` | `Verified (unit scope)` |
| `REQ-IAM-PROJ-003` | membership projection | assign/remove wrapper and projection payload | same test requires `member@user:user-1` | `Verified (unit scope)` |
| `REQ-IAM-PROJ-004` | durable projection event | `IdentityProjectionEventModel`, `EnsureStore`, `Dispatch`, state-marking methods | no PostgreSQL persistence/restart test identified | `Observed` |
| `REQ-IAM-PROJ-005` | retry pending/failed events | `IdentityProjectionDispatcher.RetryOnce`, `StartRetryWorker`, `markFailure` | no multi-replica/idempotency integration evidence identified | `Observed / high-risk gap` |
| `REQ-IAM-PROJ-006` | DTM Saga apply/compensate | `submit`, `ApplyBranch`, `CompensateBranch`; DTM topic and branch URLs | no DTM integration test identified | `Partial` |
| `REQ-IAM-PROJ-007` | retry/reconcile/drift APIs | `IAMDirectoryProjectionService` Proto; projection dispatcher and desired relationship model | no end-to-end reconciliation test identified | `Partial` |

## Runtime authorization

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-AUTHZ-RT-001` | `CheckPermission` | `internal/service/iam.go::CheckPermission` delegates to `authz.AdminProvider.Check` | fake-client coverage in `client/authzgrpc/client_test.go` | `Observed`; real SpiceDB missing |
| `REQ-IAM-AUTHZ-RT-002` | `BatchCheckPermissions` | service converts and delegates ordered checks | fake-client batch test verifies one decision | `Verified (adapter unit scope)` |
| `REQ-IAM-AUTHZ-RT-003` | `WriteRelationships` | service delegates plural relationship writes; Proto INTERNAL | fake-client test verifies count | `Verified (adapter unit scope)` |
| `REQ-IAM-AUTHZ-RT-004` | `DeleteRelationships` | service delegates filter deletion; Proto INTERNAL | fake client implements operation but no focused assertion identified | `Observed` |
| `REQ-IAM-AUTHZ-RT-005` | `ReadRelationships` | service maps relationship filters and responses | fake-client test verifies owner tuple conversion | `Verified (adapter unit scope)` |
| `REQ-IAM-AUTHZ-RT-006` | `LookupResources` | service delegates resource lookup with cursor/limit | fake client only returns empty result; no semantic test | `Observed` |
| `REQ-IAM-AUTHZ-RT-007` | `LookupSubjects` | service delegates subject lookup with cursor/limit | fake client only returns empty result; no semantic test | `Observed` |
| `REQ-IAM-AUTHZ-RT-008` | `client/authzgrpc` outgoing metadata | `client/authzgrpc/client.go` identity propagation | `client_test.go` verifies user Context and service identity metadata | `Verified (unit scope)` |

## Authorization administration

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-AUTHZ-ADMIN-001` | `GetAuthorizationSchema` | `internal/service/authz_admin.go`; explicit `view_schema` check | no real provider/permission test identified | `Observed` |
| `REQ-IAM-AUTHZ-ADMIN-002` | validate/publish schema | explicit `publish_schema` check, provider validation/write, empty publish guard | Proto contract checks run in CI; no schema rollback/compatibility integration | `Observed` |
| `REQ-IAM-AUTHZ-ADMIN-003` | list/write/delete relationships | explicit global view/repair checks and tuple validation | no broad-delete safety or real SpiceDB test identified | `Observed / high-risk gap` |
| `REQ-IAM-AUTHZ-ADMIN-004` | check/explain/effective permissions | service delegates checks and formats diagnostic steps/map | no focused tests identified | `Observed` |
| `REQ-IAM-AUTHZ-ADMIN-005` | audit admin changes | Proto declares audit events and risk levels | no durable audit-sink evidence identified | `Contract only` |

## Project and Capability

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-PROJECT-001` | target Project/Zone root model | architecture contract and SpiceDB/defaults/project biz model use Zone; PR #40 removed legacy Organization | `model_contract_test.go` rejects SpiceDB/defaults legacy Organization model | `Verified (unit scope)`; PR #40 merged |
| `REQ-IAM-PROJECT-002` | `CreateProject` scope | `internal/service/control_plane.go` uses `currentProjectContext` deriving from `authn.Principal.OrgID` | `principal_context_test.go` verifies org_id from Principal; rejects missing org_id | `Verified (unit scope)`; PR #40 merged |
| `REQ-IAM-PROJECT-003` | creator ownership | project biz projects Zone/owner; service derives actor from Principal | `principal_context_test.go` verifies actor from Principal | `Verified (unit scope)`; PR #40 merged |
| `REQ-IAM-PROJECT-004` | `GetProject`, `ListProjects` scoped to current Zone | repository read/list methods called by main service; `ListProjects` filters by `orgID` from Principal | no cross-Zone authorization integration test identified | `Observed` |
| `REQ-IAM-PROJECT-005` | `UpdateProject` | Proto contract exists | `internal/service/control_plane.go` returns `codes.Unimplemented` | `Contract only` |
| `REQ-IAM-PROJECT-006` | `ArchiveProject` | Proto contract exists | service returns `codes.Unimplemented` | `Contract only` |
| `REQ-IAM-PROJECT-007` | register/list Capability | service and project biz/repository calls exist | no focused integration test identified | `Observed` |
| `REQ-IAM-PROJECT-008` | enable/disable/list Project Capability | service and project biz/repository calls exist | no schema/config or persistence integration evidence identified | `Observed` |
| `REQ-IAM-DEPRECATED-001` | legacy Organization control plane | PR #40 removed Organization CRUD from proto, deleted `internal/biz/project/service.go`, cleaned up data layer | `model_contract_test.go` enforces single-root Zone model; Grant/Resource services reject `organization` type | `Deprecated / removed` |

## Resource control plane

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-RESOURCE-001` | register/get/list Resource Type | `ResourceService` and resource biz/repository calls exist | no focused validation test identified | `Observed` |
| `REQ-IAM-RESOURCE-002` | upsert/get Resource | service derives actor/default owner and calls biz/repository | Principal fallback unit test covers helper only | `Partial`; scope/projection integration missing |
| `REQ-IAM-RESOURCE-003` | list Resources | database filters and pagination response exist | no authorization leakage/pagination test identified | `Observed` |
| `REQ-IAM-RESOURCE-004` | archive Resource | repository archive then get exists | no lifecycle/projection test identified | `Observed` |
| `REQ-IAM-RESOURCE-005` | move/delete Resource | Proto contracts exist | both service methods return `Unimplemented` | `Contract only` |
| `REQ-IAM-RESOURCE-006` | bind/list/unbind Resource | bind and list implementations exist | unbind returns `Unimplemented`; no relation validation test identified | `Partial` |
| `REQ-IAM-RESOURCE-007` | bind/list external Resource | bind implementation exists | list returns `Unimplemented` | `Partial` |

## Grant control plane

| Requirement | API / entry point | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|---|
| `REQ-IAM-GRANT-001` | register/list Role Template | Grant service delegates to biz/repository | defaults provide built-in templates; no validation integration test identified | `Observed` |
| `REQ-IAM-GRANT-002` | `GrantAccess` | current actor extracted; biz Grant creation returns write result/consistency token | no PostgreSQL + SpiceDB transaction test identified | `Observed / high-risk gap` |
| `REQ-IAM-GRANT-003` | `RevokeAccess` | current actor extracted; biz called with relationship deletion enabled | no partial-failure recovery test identified | `Observed / high-risk gap` |
| `REQ-IAM-GRANT-004` | `ListGrants` | repository filters and pagination response exist | no authorization/pagination test identified | `Observed` |
| `REQ-IAM-GRANT-005` | `ExplainAccess` | Grant biz authorization explanation delegated | no real inherited/Group test identified | `Observed` |
| `REQ-IAM-GRANT-006` | Grant expiry | API/model contains `expires_at` | no executor, cleanup or effective-access expiry test identified in C1 | `Partial / unresolved` |

## Engineering and release controls

| Requirement | Implementation evidence | Test / CI evidence | C1 result |
|---|---|---|---|
| `REQ-IAM-ENG-001` | Proto plus generated HTTP/gRPC/access/Gateway/Kernel artifacts; Makefile generation | CI regenerates API/deploy and rejects git drift | `Verified (CI design)` |
| `REQ-IAM-ENG-002` | `go.mod` and Makefile use Kernel `v0.4.3` | CI checks runtime module version and installs matching generators | `Verified (CI design)` |
| `REQ-IAM-ENG-003` | `make proto-check` runs Buf and Aisphere contract checker | CI executes contract checks | `Verified (CI design)` |
| `REQ-IAM-ENG-004` | service methods reject nil providers and propagate provider errors; architecture requires fail-closed | no fault-injection test identified | `Partial` |
| `REQ-IAM-ENG-005` | architecture requires stable error classes; services use Kernel errors and some gRPC status errors | no complete HTTP/gRPC error matrix identified | `Architecture required` |
| `REQ-IAM-ENG-006` | Proto audit metadata exists | no operational audit sink/query evidence identified | `Contract only` |
| `REQ-IAM-ENG-007` | README/deployment expose HTTP, gRPC and metrics ports; Kernel provides observability foundation | no dashboard/alert/readiness dependency test identified | `Partial` |
| `REQ-IAM-ENG-008` | CI runs generation, checks, Go tests, binary build, drift check and Docker build | PR #40 head has successful CI and image workflows | `Build-ready evidence only`; not release-ready |

## Unresolved contract decisions

| Decision | Evidence | Risk | Required Gate 1 outcome |
|---|---|---|---|
| `REQ-IAM-DECISION-001` — singular relationship mutation APIs | `WriteRelationship` / `DeleteRelationship` are AUTHORIZED while plural runtime APIs are INTERNAL and admin APIs use global repair permissions | arbitrary tuple/relation exposure can bypass Grant abstractions | keep, restrict or remove; one regression test enforces decision |
| `REQ-IAM-DECISION-002` — canonical Group write API | overlapping methods/routes in `IAMDirectoryService` and `IAMIdentityAdminService` | duplicate registration, inconsistent permissions and identifier forms | select one canonical contract and remove/deprecate the other |

## Coverage summary

| Evidence level | C1 assessment |
|---|---|
| Architecture and Proto inventory | strong enough to proceed |
| Service implementation inventory | substantial, not yet exhaustive at every symbol |
| Unit/contract test linkage | partial |
| PostgreSQL integration | not demonstrated |
| Casdoor integration | not demonstrated |
| SpiceDB integration | not demonstrated |
| Gateway HTTP/gRPC end-to-end | not demonstrated |
| Audit/observability | not demonstrated |
| Performance/reliability | not demonstrated |
| Release readiness | **not established** |
