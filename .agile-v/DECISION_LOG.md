# Decision Log — Aisphere IAM

> Append-only. Never overwrite or delete entries.
> Format: `TIMESTAMP | AGENT_ID | DECISION | RATIONALE | LINKED_REQ`

| TIMESTAMP | AGENT | DECISION | RATIONALE | LINKED_REQ |
|-----------|-------|----------|-----------|------------|
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Start Agile V Cycle C1 — requirements recovery from existing implementation | IAM backend has grown beyond architecture docs; need traceable requirements before further changes | CR-0001 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Gate 0: PASS_WITH_FINDINGS — system understanding complete | System overview, dependency map, and known constraints documented; confidence Medium | REQ-IAM-ENG-008 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Merge PR #40 — remove legacy Organization control-plane model | Architecture requires single Casdoor Organization → Zone root; main branch conflicted | REQ-IAM-PROJECT-001, REQ-IAM-DEPRECATED-001 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Derive Project scope/owner from Kernel Principal | PR #40: currentProjectContext uses authn.Principal.OrgID; request body no longer overrides | REQ-IAM-PROJECT-002, REQ-IAM-PROJECT-003 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Grant/Resource services reject organization type | Legacy type removed; callers must use zone | REQ-IAM-DEPRECATED-001 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Initialize full Agile V framework in .agile-v/ | Add STATE.md, DECISION_LOG.md, APPROVALS.md, CONTROL_MATRIX.yaml, POLICY.yaml, TRACE_LOG.md, EVAL_RESULTS.md, CHECKPOINTS.md, RISK_REGISTER.md, phases/ | REQ-IAM-ENG-008 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Create IAMGroupAdminService — consolidate Group CRUD and membership | Group mutations were split across IAMDirectoryService and IAMIdentityAdminService. New canonical service at api/iam/v1/group_admin.proto with consistent routes and permissions. | REQ-IAM-DIR-005, REQ-IAM-DECISION-002 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Change WriteRelationship/DeleteRelationship to INTERNAL | Singular relationship mutation APIs were AUTHORIZED, allowing product clients to bypass Grant control plane. Changed to INTERNAL to enforce Grant as the high-level access control surface. | REQ-IAM-DECISION-001 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Implement UpdateProject and ArchiveProject | Project lifecycle is partially implemented (Create/Get/List exist). Update and Archive are required to complete the Project CRUD surface. Also fixed GetProject Zone permission check — was missing org_id validation. | REQ-IAM-PROJECT-005, REQ-IAM-PROJECT-006 |
| 2026-07-13T00:00:00Z | yuanyp8 | [C1] Implement 4 Resource RPCs (MoveResource, DeleteResource, UnbindResource, ListExternalResourceBindings) | All 4 were returning Unimplemented. MoveResource updates parent relationship, DeleteResource sets status=DELETED, UnbindResource removes binding, ListExternalResourceBindings queries external bindings. | REQ-IAM-RESOURCE-005, REQ-IAM-RESOURCE-006, REQ-IAM-RESOURCE-007 |