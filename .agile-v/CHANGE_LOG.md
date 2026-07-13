# Change Log — Aisphere IAM

> Append-only. Never overwrite or delete entries.
> Format: `CR-XXXX | Cycle | Affected REQ | Change | Rationale | Impact | Status`

| CR-ID | Cycle | Affected REQ | Change | Rationale | Impact | Status |
|-------|-------|-------------|--------|-----------|--------|--------|
| CR-0001 | C1 | All | Recover IAM requirements from existing implementation | IAM backend has grown beyond architecture docs; need traceable requirements before further changes | 63 REQs recovered; 17 gaps identified; Gate 1 approved | ✅ IMPLEMENTED_PENDING_GATE_2 |
| CR-0002 | C2 | DIR-005, DECISION-001, DECISION-002 | Converge IAM API boundaries — consolidate Group mutations, make raw relationship API INTERNAL | Architecture requires single canonical Group API; raw tuple mutation bypasses Grant control plane | IAMGroupAdminService created; WriteRelationship/DeleteRelationship changed to INTERNAL | ✅ IMPLEMENTED_PENDING_GATE_2 |
| CR-0003 | C3 | PROJECT-005, PROJECT-006, RESOURCE-005, RESOURCE-006, RESOURCE-007 | Complete Project and Resource lifecycle — implement UpdateProject, ArchiveProject, MoveResource, DeleteResource, UnbindResource, ListExternalResourceBindings | 6 RPCs were returning Unimplemented; full lifecycle required for product readiness | All 6 RPCs implemented; Zone permission check fixed for GetProject | ✅ IMPLEMENTED_PENDING_VERIFICATION |