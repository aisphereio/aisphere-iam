# Automated Traceability Matrix (ATM) — Aisphere IAM

> Cycle: C1 | Generated: 2026-07-13

## REQ → ART Coverage

| Domain | REQs | ART entries | Coverage |
|--------|:----:|:-----------:|:--------:|
| Authentication | 4 | 4 | 100% |
| Identity Directory | 7 | 9 | 100% |
| Directory Projection | 7 | 7 | 100% |
| Runtime Authorization | 8 | 8 | 100% |
| Authorization Admin | 5 | 5 | 100% |
| Project & Capability | 8 | 8 | 100% |
| Resource | 7 | 7 | 100% |
| Grant | 6 | 6 | 100% |
| Engineering | 8 | 8 | 100% |
| **Total** | **60** | **63** | **100%** |

## ART → VER (Test) Coverage

| Evidence Level | Count | REQs |
|:--------------:|:-----:|------|
| UNIT_EVIDENCE | 12 | AUTHN-001, AUTHN-003, DIR-006, PROJ-001~003, AUTHZ-RT-002, 003, 005, 008, PROJECT-002, 003 |
| CI_EVIDENCE | 3 | ENG-001, 002, 003 |
| OBSERVED (no test) | 39 | All remaining implemented REQs |
| CONTRACT_ONLY | 1 | AUTHZ-ADMIN-005 |
| ARCHITECTURE_REQUIRED | 2 | AUTHN-004, ENG-005 |

**Traceability Completeness:** 24% (15/62 REQs with test evidence)

## Dangling Artifacts

| Check | Result |
|-------|--------|
| ART without REQ | **0** ✅ |
| REQ without ART | **0** ✅ |
| Code without REQ | **0** ✅ |