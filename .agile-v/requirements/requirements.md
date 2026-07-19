# IAM Candidate Requirements — Cycle C1

<!-- Revision: C1 | Date: 2026-07-13 | Last updated: 2026-07-13 (PR #40 merged — legacy Organization removed) | Human Gate 1: Pending -->

## 1. Purpose and interpretation

These requirements were recovered from the current implementation, Proto contracts, accepted architecture documents, selected tests, CI configuration and recent change history.

They are **Candidate requirements** until Human Gate 1 approval. Existing behavior can be accidental, obsolete or inconsistent with the accepted architecture. Therefore:

- implementation evidence does not automatically make a requirement valid;
- a generated RPC is not considered implemented when its service returns `Unimplemented`;
- unit evidence is not equivalent to integration evidence;
- architecture-required behavior is recorded separately from conflicting main-branch behavior;
- no requirement is `RELEASE_READY` in Cycle C1.

## 2. Requirement status legend

| Status | Meaning |
|---|---|
| `OBSERVED_IMPLEMENTED` | Executable implementation was found. |
| `PARTIAL_IMPLEMENTATION` | Only part of the business path is implemented or verified. |
| `CONTRACT_ONLY` | Proto/API contract exists but executable implementation is absent or incomplete. |
| `ARCHITECTURE_REQUIRED` | Required by the accepted architecture but current main does not fully comply. |
| `DEPRECATED` | Existing/historical behavior must not remain part of the target product contract. |

## 2b. Priority definition

| Priority | Meaning | Release criteria | Examples |
|:--------:|---------|-----------------|----------|
| **P0** | **核心能力** — 系统必须有的功能，缺失则产品不可用 | 必须通过 Gate 2 才能发布 | 认证、授权、身份目录、Project/Resource CRUD |
| **P1** | **重要能力** — 产品需要有的功能，缺失则体验不完整 | 建议通过 Gate 2，可带已知问题发布 | 投影、授权管理、Grant 过期、审计持久化 |
| **P2** | **完善性** — 锦上添花的功能，缺失不影响核心使用 | 可推迟到后续 Cycle | 性能 SLO、错误矩阵、告警规则 |

### P0 判定标准（满足任一即 P0）

1. **安全边界** — 认证、授权、鉴权相关，缺失会导致安全漏洞
2. **核心业务** — 用户/组织/Project/Resource 的 CRUD，缺失则产品不可用
3. **数据一致性** — 关键数据同步和投影，缺失会导致数据不一致
4. **架构契约** — 已决策的架构模型，违反会导致技术债务

### P1 判定标准（满足任一即 P1）

1. **管理能力** — 管理员操作（Schema 管理、关系修复、审计）
2. **生命周期完善** — 非核心但必要的生命周期操作（Grant 过期、投影修复）
3. **可靠性** — 重试、补偿、漂移检测等可靠性机制
4. **合规性** — 审计日志、操作记录等合规要求

### P2 判定标准（满足任一即 P2）

1. **工程优化** — 性能 SLO、错误矩阵、告警规则
2. **可观测性** — 监控、日志、追踪的完善
3. **非功能性** — 代码质量、文档、测试覆盖率的提升

Evidence qualifiers:

- `UNIT_EVIDENCE`
- `CONTRACT_EVIDENCE`
- `CI_EVIDENCE`
- `INTEGRATION_EVIDENCE`

## 3. Requirement summary

| Domain and Capability | Candidate requirements | Priority | Main finding |
|---|---:|---:|---|
| Authentication and Principal | 4 | P0 | Kernel Context is authoritative, but Gateway runtime evidence is missing |
| Identity directory | 7 | P0 | reads are well-defined; Group write surface consolidated |
| Directory projection | 7 | P1 | durable retry model exists; real failure/recovery proof is missing |
| Runtime authorization | 8 | P0 | core data-plane API exists; singular tuple APIs moved to INTERNAL |
| Authorization administration | 5 | P1 | administration surface exists; audit evidence is contractual only |
| Project and Capability | 8 | P0 | full CRUD implemented; scope/owner from Principal |
| Resource control plane | 7 | P0 | full lifecycle implemented |
| Grant control plane | 6 | P1 | Grant lifecycle exists; real SpiceDB evidence is missing |
| Engineering and release controls | 8 | P2 | CI is strong for build consistency, not release readiness |

---

# 4. Authentication and Principal requirements (P0)

## REQ-IAM-AUTHN-001 — Return the authenticated current Principal
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** `GetMe` shall return the normalized authenticated Principal restored by Kernel middleware from request Context.
- **Constraint:** IAM business code shall not reconstruct the current Principal from ordinary request headers or request-body identity fields.
- **Verification criteria:**
  1. authenticated Context returns the expected subject, Organization and authentication metadata;
  2. missing or unauthenticated Principal returns an unauthenticated/missing-credential error;
  3. user-controlled headers cannot replace the Kernel Principal.
- **Done criteria:** unit test, HTTP test behind Gateway, gRPC test and header-spoofing negative test pass.

## REQ-IAM-AUTHN-002 — Verify a token only for trusted platform services
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall expose token verification only as an INTERNAL platform-service operation and delegate verification to the configured token provider.
- **Constraint:** IAM shall return a dependency/backend failure when the token provider is unavailable and shall not fabricate a Principal.
- **Verification criteria:** valid, expired, malformed, wrong-issuer and wrong-audience tokens produce the expected result; external Gateway routes do not expose the operation.
- **Done criteria:** Casdoor-backed integration tests and Gateway route-isolation tests pass.

## REQ-IAM-AUTHN-003 — Derive actors from Kernel Context
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** every control-plane mutation shall derive `created_by`, `actor` and default owner from the authenticated Kernel Principal.
- **Constraint:** a client-provided actor or owner shall not override the authenticated caller unless an explicitly approved delegated-administration requirement exists.
- **Verification criteria:** Project, Resource and Grant mutation requests with forged actor/owner values cannot change the recorded actor; missing Principal is rejected.
- **Done criteria:** all mutation services use one shared Principal extraction contract and have negative tests.

## REQ-IAM-AUTHN-004 — Keep browser authentication outside IAM
- **Priority:** P0
- **Status:** `ARCHITECTURE_REQUIRED`
- **Requirement:** Envoy Gateway shall remain the external OIDC/JWT authentication boundary; IAM shall not own browser Session, authorization-code callback state or refresh-token lifecycle.
- **Constraint:** IAM may adapt or verify identities for trusted services but shall not become a browser-facing session server.
- **Verification criteria:** generated public routes contain no browser login/callback/session endpoints; deployment tests prove Gateway performs OIDC/JWT validation and trusted identity propagation.
- **Done criteria:** Gateway integration evidence and architecture contract test exist.

---

# 5. Identity directory requirements (P0)

## REQ-IAM-DIR-001 — Read a User only with Zone permission
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** reading a User from an Organization shall require `zone:<org_id>#view_users` for the current Principal before calling the identity provider.
- **Verification criteria:** allowed, denied, missing Principal and SpiceDB unavailable scenarios are tested.
- **Done criteria:** Casdoor + SpiceDB integration tests pass and denial is fail-closed.

## REQ-IAM-DIR-002 — List Users only with Zone permission
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** listing Users shall require `zone:<org_id>#view_users` and shall pass supported filters to the identity provider.
- **Verification criteria:** Organization, Group, Role and page-size filters are validated; cross-Organization access is rejected.
- **Done criteria:** filter, pagination and authorization integration tests pass.

## REQ-IAM-DIR-003 — Read Casdoor Organization metadata as the identity-domain root
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall expose read-only metadata for the selected Casdoor Organization after `zone:<org_id>#view_zone` authorization.
- **Constraint:** this operation shall not imply that IAM owns a second Organization lifecycle.
- **Verification criteria:** metadata is sourced from Casdoor; unauthorized and cross-domain reads are rejected.
- **Done criteria:** Casdoor integration and architecture-boundary tests pass.

## REQ-IAM-DIR-004 — List Groups only with Zone permission
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** listing the multi-level Group tree shall require `zone:<org_id>#view_groups` and support parent/type/user filtering.
- **Verification criteria:** root, child, user-membership and unauthorized listing scenarios pass; membership filtering resolves both stable IDs and persisted machine-name aliases.
- **Done criteria:** real Casdoor Group-tree tests and pagination/empty-tree tests pass.

## REQ-IAM-DIR-005 — Manage Groups through one canonical API
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall provide one canonical contract for creating, updating and deleting Casdoor-backed Groups.
- **Constraint:** the contract shall define stable Group identifiers, parent semantics, recursive deletion behavior, idempotency and required permissions; IAM shall durably preserve the user-facing machine name that Casdoor cannot store separately from its primary key.
- **Implementation:** `IAMGroupAdminService` (`api/iam/v1/group_admin.proto`) is the canonical Group management surface. Routes: `/v1/iam/groups/...`. Permissions: `zone:*` for create, `group:*` for manage. Group writes removed from `IAMDirectoryService` and `IAMIdentityAdminService`.
- **Verification criteria:** only the approved surface is externally registered; duplicate routes do not exist; permission and audit behavior is consistent; create/read/update preserve the stable-ID-to-machine-name mapping.
- **Done criteria:** contract tests enforce the canonical service.

## REQ-IAM-DIR-006 — Assign and remove User membership
- **Priority:** P0
- **Status:** `PARTIAL_IMPLEMENTATION`, `UNIT_EVIDENCE`
- **Requirement:** an authorized caller shall be able to assign a User to a Group and remove that membership through IAM, without the frontend calling Casdoor directly.
- **Constraint:** the Group and User must belong to the selected Casdoor Organization; projection to SpiceDB must use the stable User subject identifier.
- **Verification criteria:** assign, duplicate assign, remove, duplicate remove, cross-Organization and missing-object cases are tested.
- **Done criteria:** Casdoor mutation and SpiceDB projection are verified end-to-end.

## REQ-IAM-DIR-007 — Enforce identity-provider mode boundaries
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** in `casdoor_local` mode IAM may perform configured identity-provider writes; in `external_oidc` mode IAM shall reject upstream User/Organization mutation while preserving approved Aisphere Group and membership operations.
- **Verification criteria:** every write method is tested in both modes; rejected calls do not reach the upstream provider.
- **Done criteria:** a complete mode matrix test exists and configuration rejects unknown mode values.

---

# 6. Directory projection requirements

## REQ-IAM-PROJ-001 — Project Group identity with Organization qualification
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** a Casdoor Group shall be projected as `group:<org_id>/<group_id>` and linked to `zone:<org_id>`.
- **Verification criteria:** collisions between identical Group IDs in different Organizations cannot occur.
- **Done criteria:** unit and real SpiceDB tests confirm the relationship shape.

## REQ-IAM-PROJ-002 — Project the multi-level Group parent relation
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** a child Group shall have a `parent` relationship to its Organization-qualified parent Group.
- **Verification criteria:** create, move/update, recursive delete and orphan-parent scenarios are covered.
- **Done criteria:** Group-tree permission inheritance is verified in SpiceDB.

## REQ-IAM-PROJ-003 — Project User membership as `group#member`
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** User membership shall be projected as `group:<qualified-id>#member@user:<stable-user-id>`.
- **Verification criteria:** assign and remove operations change effective authorization as expected.
- **Done criteria:** Casdoor membership and SpiceDB effective permission are verified in one E2E test.

## REQ-IAM-PROJ-004 — Persist projection work and state
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** when a projection store is configured, IAM shall persist a projection event containing aggregate identity, operation, payload, status, retry count, error and next-run time.
- **Verification criteria:** pending, submitted, projecting, synced, failed and archived transitions are tested.
- **Done criteria:** PostgreSQL integration tests verify durable state across process restart.

## REQ-IAM-PROJ-005 — Retry failed or pending projection work
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall retry eligible pending, submitted or failed projection events with bounded batch size and observable failure state.
- **Constraint:** retries must be idempotent and safe under multiple IAM replicas.
- **Verification criteria:** transient SpiceDB failure, permanent invalid relationship, restart and concurrent worker scenarios are tested.
- **Done criteria:** no duplicate effective relationship or lost event occurs in integration tests.

## REQ-IAM-PROJ-006 — Support DTM apply and compensation
- **Priority:** P1
- **Status:** `PARTIAL_IMPLEMENTATION`
- **Requirement:** when DTM is enabled, IAM shall submit projection work as a Saga with explicit apply and compensate branches and preserve the event state.
- **Verification criteria:** apply success, apply failure, compensation success, compensation failure and DTM unavailable cases are covered.
- **Done criteria:** DTM integration tests and recovery runbook exist.

## REQ-IAM-PROJ-007 — Detect and repair directory projection drift
- **Priority:** P1
- **Status:** `PARTIAL_IMPLEMENTATION`
- **Requirement:** authorized administrators shall be able to detect missing desired relationships, reconcile a selected Organization and retry failed projection events.
- **Verification criteria:** drift output is deterministic; repair is idempotent; unexpected extra relationships have an explicit policy.
- **Done criteria:** real Casdoor-to-SpiceDB reconciliation tests pass with audit evidence.

---

# 7. Runtime authorization requirements

## REQ-IAM-AUTHZ-RT-001 — Check one permission
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** a trusted platform service shall be able to check a subject, resource and permission and receive allow/deny effect, reason and consistency token.
- **Verification criteria:** direct, inherited, Group-derived, public/read-only, denied and dependency-error decisions are covered.
- **Done criteria:** SpiceDB integration tests prove the supported resource models.

## REQ-IAM-AUTHZ-RT-002 — Batch permission checks
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** a trusted service shall be able to submit one or more permission checks and receive decisions in the same order.
- **Verification criteria:** empty input is rejected; mixed allow/deny and partial backend failure semantics are defined and tested.
- **Done criteria:** batch correctness and latency tests pass.

## REQ-IAM-AUTHZ-RT-003 — Write relationship projections in batches
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** trusted backend services shall be able to write one or more approved authorization relationships through the INTERNAL IAM runtime API.
- **Constraint:** schema administration is not part of the runtime client.
- **Verification criteria:** valid, duplicate, invalid-schema and unauthorized caller cases are tested.
- **Done criteria:** internal service authentication and SpiceDB integration tests pass.

## REQ-IAM-AUTHZ-RT-004 — Delete relationship projections by filter
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** trusted backend services shall be able to delete projected relationships by an explicit filter and receive count and consistency evidence.
- **Constraint:** broad filters require an approved safety rule to prevent accidental global deletion.
- **Verification criteria:** exact, scoped, empty and dangerous filter cases are tested.
- **Done criteria:** deletion guardrails and audit evidence exist.

## REQ-IAM-AUTHZ-RT-005 — Read relationships for trusted services
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** trusted services shall be able to read relationships using resource and subject filters.
- **Verification criteria:** all filters, empty result and pagination/limit policy are covered.
- **Done criteria:** real SpiceDB read tests pass.

## REQ-IAM-AUTHZ-RT-006 — Lookup accessible resources
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall return resources of a requested type for which a subject has a requested permission, with cursor and consistency token support.
- **Verification criteria:** direct, inherited, Group and no-access cases are tested.
- **Done criteria:** pagination and authorization correctness tests pass against SpiceDB.

## REQ-IAM-AUTHZ-RT-007 — Lookup authorized subjects
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall return subjects of a requested type that have a requested permission on a resource, with cursor and consistency support.
- **Verification criteria:** User, Group memberset and service subjects are covered.
- **Done criteria:** real lookup-subject tests pass.

## REQ-IAM-AUTHZ-RT-008 — Propagate stable authorization identity over gRPC
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** the IAM runtime client shall propagate the current Principal's stable subject, subject type, provider, organization and project identifiers; background work shall use an explicitly supplied service Principal.
- **Constraint:** profile attributes such as name, username, email and phone must not enter authorization gRPC metadata, and unauthenticated callers must not be silently promoted to a privileged identity.
- **Verification criteria:** user UUID and service identities propagate while Unicode profile attributes cannot produce non-printable gRPC metadata.
- **Done criteria:** stable-identity propagation and metadata-safety tests pass.

---

# 8. Authorization administration requirements

## REQ-IAM-AUTHZ-ADMIN-001 — Read the active authorization schema
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** only a Principal with `iam_authz:global#view_schema` shall read the current schema text and version.
- **Verification criteria:** allowed, denied and provider-unavailable cases are tested.
- **Done criteria:** real SpiceDB and audit tests pass.

## REQ-IAM-AUTHZ-ADMIN-002 — Validate and publish a schema
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** only a Principal with `iam_authz:global#publish_schema` shall validate or publish non-empty schema text.
- **Constraint:** publishing is a critical operation and must be auditable and protected from incompatible schema changes.
- **Verification criteria:** valid, invalid, empty, incompatible and unauthorized cases are tested.
- **Done criteria:** rollback/compatibility policy and audit evidence exist.

## REQ-IAM-AUTHZ-ADMIN-003 — Inspect and repair relationships
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** relationship inspection shall require `view_relationships`; administrative write/delete shall require `repair_relationships`.
- **Verification criteria:** invalid tuples and overly broad deletion filters are rejected.
- **Done criteria:** permissions, audit and safety tests pass.

## REQ-IAM-AUTHZ-ADMIN-004 — Diagnose authorization decisions
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** an authorized administrator shall be able to check, explain and enumerate selected effective permissions for a subject/resource pair.
- **Constraint:** explanation shall not claim a graph path that the provider did not return; current implementation only provides a summarized diagnostic sequence.
- **Verification criteria:** allow, deny, backend error and multiple-permission cases are tested.
- **Done criteria:** UI/API semantics clearly distinguish provider evidence from locally formatted diagnostics.

## REQ-IAM-AUTHZ-ADMIN-005 — Audit all administrative authorization changes
- **Priority:** P1
- **Status:** `CONTRACT_ONLY`
- **Requirement:** schema publication, relationship repair/delete and projection repair shall produce durable audit records with actor, target, reason, request ID, trace ID, decision ID and outcome.
- **Verification criteria:** audit records are queryable and correlated to HTTP/gRPC requests.
- **Done criteria:** operational audit integration tests pass.

---

# 9. Project and Capability requirements

## REQ-IAM-PROJECT-001 — Use Casdoor Organization as the single root
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** IAM shall map one Casdoor Organization to one read-only `zone` root and shall not maintain a second Platform Organization entity.
- **Implementation:** PR #40 (`0425275`) removed legacy Organization CRUD from proto, deleted `internal/biz/project/service.go`, cleaned up data layer. Grant/Resource services reject `organization` type.
- **Verification criteria:** Proto, generated routes, database models, repository methods, defaults and SpiceDB schema contain no legacy Organization control plane.
- **Done criteria:** contract tests pass and the legacy surface is absent.

## REQ-IAM-PROJECT-002 — Derive Project Zone from the authenticated Principal
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** `CreateProject` shall derive the Project `org_id`/Zone from `Principal.org_id`; request bodies and paths shall not override the identity domain.
- **Implementation:** `internal/service/control_plane.go::currentProjectContext` extracts org_id from `authn.Principal.OrgID`; `CreateProjectRequest.ZoneID` is set by the service, not the request.
- **Verification criteria:** forged Organization input is impossible or ignored; missing Principal Organization is rejected.
- **Done criteria:** `principal_context_test.go` verifies org_id from Principal and rejects missing org_id.

## REQ-IAM-PROJECT-003 — Make the creator the Project owner
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`, `UNIT_EVIDENCE`
- **Requirement:** Project creation shall atomically persist the Project fact and project `project:<id>#owner@<creator>`.
- **Constraint:** an owner shall not nominate another owner during creation.
- **Implementation:** `currentProjectContext` derives actor from `authn.Principal`; `CreateProjectRequest.Owner` is set to the same actor.
- **Verification criteria:** creator ownership grants expected permissions immediately; projection failure is visible and recoverable.
- **Done criteria:** `principal_context_test.go` verifies actor from Principal.

## REQ-IAM-PROJECT-004 — Read and list Projects within the current Zone
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** Project reads and lists shall be authorization-aware and default to the authenticated Principal's Zone.
- **Constraint:** a list filter shall not expose another Zone without an explicit cross-domain administrative requirement.
- **Implementation:** `ListProjects` filters by `orgID` from `currentProjectContext`; `GetProject` reads by ID.
- **Verification criteria:** own Zone, other Zone, joined-only and pagination scenarios are tested.
- **Done criteria:** cross-Zone authorization integration test is still needed.

## REQ-IAM-PROJECT-005 — Update a Project
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** an authorized Project manager shall be able to update approved mutable Project fields.
- **Implementation:** `internal/service/control_plane.go::UpdateProject` uses `UpsertProject` to update display_name, description, visibility, labels, annotations. Zone permission checked via `currentProjectContext`.
- **Verification criteria:** immutable ID/Zone/creator fields cannot change; optimistic concurrency policy is defined.
- **Done criteria:** implementation, persistence, audit and integration tests exist.

## REQ-IAM-PROJECT-006 — Archive a Project
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** an authorized manager shall archive a Project without silently deleting its audit/control-plane history.
- **Implementation:** `internal/service/control_plane.go::ArchiveProject` sets status to ARCHIVED via `ArchiveProject` in repository. Zone permission checked via `currentProjectContext`.
- **Verification criteria:** archived Projects reject disallowed mutations and have an explicit resource/projection cleanup policy.
- **Done criteria:** lifecycle and restoration/deletion policy are approved and tested.

## REQ-IAM-PROJECT-007 — Register and list Capabilities
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall register Capability metadata and list Capabilities by supported status filters.
- **Verification criteria:** duplicate ID/name, invalid owner service and schema validation are tested.
- **Done criteria:** persistence and authorization integration tests pass.

## REQ-IAM-PROJECT-008 — Enable, disable and list Project Capabilities
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** an authorized Project manager shall enable or disable a registered Capability with config/quota and list the resulting Project Capability state.
- **Verification criteria:** unknown Capability, invalid config, repeated enable/disable and archived Project scenarios are covered.
- **Done criteria:** schema validation and transactional persistence tests pass.

---

# 10. Resource control-plane requirements

## REQ-IAM-RESOURCE-001 — Register and discover Resource Types
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall register, get and list Resource Types including capability ownership, parent types, grantability, auditability, SpiceDB type, relations and permissions.
- **Verification criteria:** invalid relation/permission names, incompatible parent type and duplicate type are rejected.
- **Done criteria:** defaults and runtime registration obey one validation contract.

## REQ-IAM-RESOURCE-002 — Upsert and read a Resource
- **Priority:** P0
- **Status:** `PARTIAL_IMPLEMENTATION`
- **Requirement:** an authorized service shall upsert a Resource fact and retrieve it by typed reference.
- **Constraint:** actor/default owner come from Kernel Context; Organization/Project scope must be validated rather than trusted from input.
- **Verification criteria:** create, update, idempotent replay, invalid parent, cross-Zone and missing actor scenarios are tested.
- **Done criteria:** PostgreSQL and SpiceDB structural/ownership projection tests pass.

## REQ-IAM-RESOURCE-003 — List Resources with scoped filters
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall list Resources using approved type, Zone, Project, status and pagination filters.
- **Constraint:** results must be authorization-scoped, not only database-filtered.
- **Verification criteria:** cross-Project/Zone leakage and pagination stability are tested.
- **Done criteria:** authorization-aware list behavior is proven.

## REQ-IAM-RESOURCE-004 — Archive a Resource
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall archive a Resource and return its updated state.
- **Verification criteria:** already archived, missing, dependent-child and authorization cases are tested.
- **Done criteria:** relationship cleanup/retention behavior is defined and verified.

## REQ-IAM-RESOURCE-005 — Move and delete a Resource
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** approved Resource types may support move and delete under explicit lifecycle and dependency rules.
- **Implementation:** `MoveResource` updates parent relationship via `UpsertResource`. `DeleteResource` sets status=DELETED via `DeleteResource` in repository.
- **Verification criteria:** cycles, invalid parent, dependent resources, soft delete and relationship cleanup are covered.
- **Done criteria:** product requirements are approved before implementation.

## REQ-IAM-RESOURCE-006 — Unbind and unbind internal Resources
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall create, list and remove typed Resource bindings.
- **Implementation:** `UnbindResource` deletes the binding record via `UnbindResource` in repository. Bind and list were already implemented.
- **Verification criteria:** duplicate binding, invalid relation, cross-scope binding and unbind idempotency are tested.
- **Done criteria:** the full binding lifecycle is implemented and verified.

## REQ-IAM-RESOURCE-007 — Bind and list external Resources
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall bind an internal Resource to an external provider identity/path/URL and list those bindings.
- **Implementation:** `ListExternalResourceBindings` queries external bindings by resource, provider, and sync status. Bind was already implemented.
- **Verification criteria:** provider uniqueness, sync mode/status and stale external identity are covered.
- **Done criteria:** read lifecycle and synchronization policy are implemented.

## REQ-IAM-RESOURCE-008 — Use Skill as the canonical Git authorization resource
- **Priority:** P0
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall use `skill:<name>` as the sole authorization resource for Skill metadata and its native Git/LFS repository; repository name and Skill name are identical.
- **Constraint:** no `git_namespace`, `git_repository`, `backing_repo` or `backing_skill` authorization model may coexist. Ordinary branch writes require `edit`; formal publication to `main` or release tags requires `publish`.
- **Verification criteria:** committed schema, permission manifest, identity cleanup filters and default role templates expose the canonical Skill model and reject the removed Git resource model.
- **Done criteria:** contract and identity-mode tests pass and Hub can check all Git operations through IAM using only the Skill reference.

---

# 11. Grant control-plane requirements

## REQ-IAM-GRANT-001 — Register and list Role Templates
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall register and list Role Templates that map a product-facing role key to a SpiceDB relation for a Resource Type.
- **Verification criteria:** unknown Resource Type, unknown relation, duplicate role and disabled role are tested.
- **Done criteria:** Role Template validation is tied to Resource Type and schema metadata.

## REQ-IAM-GRANT-002 — Grant access as a high-level control-plane operation
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** an authorized actor shall grant a Role Template to a User, Group memberset or Service on a Resource; IAM shall persist the Grant fact and write the corresponding relationship.
- **Constraint:** ordinary product users shall not construct arbitrary tuples or raw relation keys.
- **Verification criteria:** direct User, Group `#member`, Service, duplicate, expiry and invalid role cases are tested.
- **Done criteria:** PostgreSQL + SpiceDB integration proves fact/projection consistency.

## REQ-IAM-GRANT-003 — Revoke access and remove its projection
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** an authorized actor shall revoke an active Grant, record actor/reason/time and remove the graph relationship.
- **Verification criteria:** repeated revoke, missing Grant, projection failure and partial-state recovery are tested.
- **Done criteria:** durable recovery and audit evidence exist.

## REQ-IAM-GRANT-004 — List Grants
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall list Grants by Resource and/or Subject with stable pagination.
- **Verification criteria:** active, expired, revoked and no-filter safety behavior are defined.
- **Done criteria:** authorization scope and pagination tests pass.

## REQ-IAM-GRANT-005 — Explain effective access
- **Priority:** P1
- **Status:** `OBSERVED_IMPLEMENTED`
- **Requirement:** IAM shall explain whether a Subject has a requested permission on a Resource using the authorization provider decision.
- **Constraint:** the response must distinguish a current decision from a complete historical grant explanation.
- **Verification criteria:** direct role, inherited role, Group membership, deny and backend error are tested.
- **Done criteria:** semantics are documented and verified against SpiceDB.

## REQ-IAM-GRANT-006 — Enforce Grant expiry
- **Priority:** P1
- **Status:** `PARTIAL_IMPLEMENTATION`
- **Requirement:** a Grant with `expires_at` shall cease to provide effective access at the defined time and shall have an observable lifecycle state.
- **Current evidence:** expiry is represented in the model/API, but Cycle C1 has not found execution evidence that removes or excludes the relationship.
- **Verification criteria:** before expiry, after expiry, clock skew, retry and cleanup cases are covered.
- **Done criteria:** expiry worker/query strategy and integration tests exist.

## REQ-IAM-GRANT-007 — Support scoped administrators and reusable custom roles
- **Priority:** P1
- **Status:** `PARTIAL_IMPLEMENTATION`, `UNIT_EVIDENCE`
- **Requirement:** IAM shall assign administrators once at platform, Zone or Group scope and derive child-resource access through SpiceDB inheritance; IAM shall also support manifest-validated custom role capability bundles for fine-grained User and Group grants.
- **Constraint:** existing `owner` and `admin` bootstrap aliases must remain Zone-scoped; platform roles require explicit names and legacy expanded relationships may only be removed through an explicit migration switch.
- **Verification criteria:** relationship cardinality, platform/Zone/Group scope boundaries, safe legacy cleanup, custom role lifecycle, binding projection, impact preview and resource-level sharing are tested.
- **Done criteria:** backend projection and API tests, frontend role/assignment flows, migration documentation and cross-surface verification pass.

---

# 12. Engineering, security and release requirements

## REQ-IAM-ENG-001 — Use Proto as the API source of truth
- **Priority:** P2
- **Status:** `OBSERVED_IMPLEMENTED`, `CI_EVIDENCE`
- **Requirement:** supported HTTP/gRPC routes, validation metadata, access policy, audit metadata, Kernel registration and Gateway manifests shall be generated from Proto contracts.
- **Constraint:** hand-written product routes require an explicit exception and equivalent policy/audit coverage.
- **Verification criteria:** regeneration is deterministic and CI detects drift.
- **Done criteria:** generated drift check remains mandatory.

## REQ-IAM-ENG-002 — Keep Kernel runtime and generators aligned
- **Priority:** P2
- **Status:** `OBSERVED_IMPLEMENTED`, `CI_EVIDENCE`
- **Requirement:** the Kernel runtime module and all Kernel code generators shall use the same released version.
- **Verification criteria:** CI fails on mismatch.
- **Done criteria:** local, CI and Docker generation use the same version source.

## REQ-IAM-ENG-003 — Enforce API and authorization contract checks
- **Priority:** P2
- **Status:** `OBSERVED_IMPLEMENTED`, `CI_EVIDENCE`
- **Requirement:** every change to Proto or authorization contracts shall pass Buf lint/build and Aisphere contract checks before merge.
- **Verification criteria:** intentionally invalid exposure, missing reason, invalid policy or schema/root-model change causes a failing test/check.
- **Done criteria:** negative fixtures cover all critical rules.

## REQ-IAM-ENG-004 — Fail closed on identity or authorization dependency failure
- **Priority:** P2
- **Status:** `PARTIAL_IMPLEMENTATION`
- **Requirement:** missing/unavailable identity or authorization providers shall never result in an allow decision or fabricated identity.
- **Verification criteria:** timeout, unavailable, malformed response and partial outage cases are tested at HTTP and gRPC boundaries.
- **Done criteria:** chaos/fault-injection integration suite passes.

## REQ-IAM-ENG-005 — Return stable error classes
- **Priority:** P2
- **Status:** `ARCHITECTURE_REQUIRED`
- **Requirement:** APIs shall distinguish invalid input, unauthenticated, permission denied, not found, conflict, dependency failure and projection failure consistently over HTTP and gRPC.
- **Verification criteria:** a cross-service error matrix proves status/code/body consistency.
- **Done criteria:** generated/open API documentation and tests encode the matrix.

## REQ-IAM-ENG-006 — Produce durable audit evidence
- **Priority:** P2
- **Status:** `CONTRACT_ONLY`
- **Requirement:** operations marked for audit shall produce durable records containing actor, action, target, risk, outcome and correlation identifiers.
- **Verification criteria:** successful and failed critical operations are queryable.
- **Done criteria:** audit sink and retention policy are verified.

## REQ-IAM-ENG-007 — Expose health, metrics, logs and traces
- **Priority:** P2
- **Status:** `PARTIAL_IMPLEMENTATION`
- **Requirement:** IAM shall expose readiness/health, Prometheus metrics, structured logs and tracing sufficient to diagnose Casdoor, PostgreSQL, SpiceDB and projection failures.
- **Verification criteria:** dependency failure changes readiness/metrics according to the approved policy and preserves request/trace/decision correlation.
- **Done criteria:** dashboards, alerts and runbooks are validated in the test environment.

## REQ-IAM-ENG-008 — Require evidence before release
- **Priority:** P2
- **Status:** `ARCHITECTURE_REQUIRED`
- **Requirement:** an IAM release shall not be marked ready solely because generation, unit tests and compilation pass.
- **Required evidence:**
  1. Proto/contract checks;
  2. unit tests;
  3. PostgreSQL integration;
  4. Casdoor integration;
  5. SpiceDB authorization and projection integration;
  6. HTTP/gRPC/Gateway end-to-end tests;
  7. authorization negative/security tests;
  8. audit/observability verification;
  9. performance and reliability thresholds;
  10. deployment and rollback validation.
- **Done criteria:** Gate 2 evidence summary has no unaccepted P0/P1 findings.

## REQ-IAM-ENG-009 — Keep permission bootstrap deterministic and additive-only
- **Priority:** P1
- **Status:** `IMPLEMENTED`, `UNIT_EVIDENCE`, `CI_EVIDENCE`
- **Requirement:** IAM shall keep its permission manifest aligned with the SpiceDB schema, load bootstrap role policy from that manifest, and automatically publish only missing schema declarations.
- **Constraint:** changed or removed active relations and permissions shall fail closed and require an explicit schema migration.
- **Verification criteria:** committed manifest/schema drift fails CI; identical schema skips writes; strict additions publish; changed or active-only declarations are rejected.
- **Done criteria:** manifest validation, bootstrap regression tests, Make/CI gate, and operator documentation are present.

---

# 13. Explicitly deprecated or unresolved surfaces

## REQ-IAM-DEPRECATED-001 — Remove the second Platform Organization control plane
- **Priority:** P0
- **Status:** `DEPRECATED`
- **Behavior to remove:** Organization CRUD/archive under `ProjectService`, Organization persistence/repository state and `organization:*` authorization resources.
- **Reason:** conflicts with the accepted single-root architecture.
- **Removal evidence required:** no Proto method, generated route, service method, database model/migration, defaults entry or SpiceDB definition remains.

## REQ-IAM-DECISION-001 — Decide the singular relationship mutation surface
- **Priority:** P0
- **Status:** `DECIDED`
- **Decision:** `WriteRelationship` and `DeleteRelationship` changed from `AUTHORIZED` to `INTERNAL`. GrantAccess/RevokeAccess remain the only product-facing access control operations.
- **Implementation:** `api/iam/v1/iam.proto` exposure changed; `reason` field added explaining restriction.
- **Risk:** raw tuple mutation bypasses the high-level Grant control plane and exposes relation keys to product clients.
- **Gate 1 requirement:** confirm the decision and add a regression test preventing surface drift.

## REQ-IAM-DECISION-002 — Decide the canonical Group mutation surface
- **Priority:** P0
- **Status:** `DECIDED`
- **Decision:** Group writes consolidated into `IAMGroupAdminService` (`api/iam/v1/group_admin.proto`). Routes: `/v1/iam/groups/...`. Permissions: `zone:*` for create, `group:*` for manage. Group writes removed from `IAMDirectoryService` and `IAMIdentityAdminService`.
- **Implementation:** `internal/service/group_admin.go` created; `internal/server/modules.go` and `wiring.go` updated; `internal/server/access.go` `isManualGroupManagementOperation` hack removed.

---

# 14. Human Gate 1 checklist

The requirement catalogue can move from `Candidate` to `Approved [C1]` only after these decisions:

- [x] approve the single Casdoor Organization → Zone root model;
- [x] approve removal of legacy Organization control-plane APIs;
- [x] approve Principal-derived Project scope and owner rules;
- [x] select the canonical Group mutation API — **IAMGroupAdminService**;
- [x] decide the raw/singular relationship mutation surface — **INTERNAL**;
- [x] approve which unimplemented RPCs belong in the first release — **全部已实现（UpdateProject, ArchiveProject, MoveResource, DeleteResource, UnbindResource, ListExternalResourceBindings）**;
- [x] approve the required integration environment — **aisphere-dev (36.137.200.194) K8s 集群**;
- [x] assign priorities (`P0`, `P1`, `P2`) and release milestone to approved requirements — **P0: 认证/目录/授权/Project/Resource (34个); P1: 投影/授权管理/Grant (18个); P2: 工程 (8个)**;
