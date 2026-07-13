# CAPA Log — Aisphere IAM

> Corrective and Preventive Action log. Append-only.
> Format: `CAPA-XXXX | Cycle | Trigger | Nonconformity | Root Cause | Corrective Action | Preventive Action | Effectiveness Verification | Status | Owner`

| CAPA-ID | Cycle | Trigger | Nonconformity | Root Cause | Corrective Action | Preventive Action | Effectiveness Verification | Status | Owner |
|---------|-------|---------|---------------|------------|-------------------|-------------------|---------------------------|--------|-------|
| CAPA-0001 | C1 | NC-001 | GetProject missing Zone permission check — cross-Zone read was possible | Legacy Organization model left a code path that bypassed org_id validation | Added `currentProjectContext` org_id validation to GetProject | PR #40 removed legacy Organization model entirely; contract test enforces single-root | ✅ Verified: GetProject rejects cross-Zone access | ✅ CLOSED | yuanyp8 |
| CAPA-0002 | C1 | NC-002 | `isManualGroupManagementOperation` hack in access.go allowed inconsistent Group mutation paths | Group mutations were split across IAMDirectoryService and IAMIdentityAdminService | Created IAMGroupAdminService as canonical Group management surface; removed duplicate routes | Proto contract test enforces single canonical Group service | ✅ Verified: only IAMGroupAdminService exposes Group write routes | ✅ CLOSED | yuanyp8 |