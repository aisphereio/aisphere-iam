# Risk Register — Aisphere IAM

| ID | RISK | LIKELIHOOD | IMPACT | MITIGATION | OWNER | STATUS |
|----|------|-----------|--------|------------|-------|--------|
| RISK-IAM-001 | SpiceDB unavailable causes authorization fail-open | Low | Critical | Kernel authz defaults to DenyAll; DevAllowAll only in dev | yuanyp8 | ✅ MITIGATED |
| RISK-IAM-002 | PostgreSQL succeeds but SpiceDB projection fails | Medium | High | DTM Saga with apply/compensate; retry worker; drift detection | yuanyp8 | ⏳ PARTIAL |
| RISK-IAM-003 | Gateway trust boundary bypassed (spoofed headers) | Low | Critical | Envoy NetworkPolicy/mTLS; Kernel strips external identity headers | yuanyp8 | ⏳ UNTESTED |
| RISK-IAM-004 | Expired Grant continues to authorize | Medium | Medium | Expiry represented in model; no executor/cleanup worker yet | yuanyp8 | ❌ OPEN |
| RISK-IAM-005 | Cross-zone data leakage in list APIs | Low | High | ListProjects filters by Principal org_id; no integration test | yuanyp8 | ⏳ UNTESTED |
| RISK-IAM-006 | Projection event lost on process crash | Low | High | Durable event store; retry worker; no restart recovery test | yuanyp8 | ⏳ UNTESTED |
| RISK-IAM-007 | Duplicate Group mutation contracts cause inconsistent behavior | Medium | Medium | IAMDirectoryService and IAMIdentityAdminService both define Group writes | yuanyp8 | ❌ OPEN (GAP-IAM-002) |