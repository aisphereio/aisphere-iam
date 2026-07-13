# Aisphere IAM — Agile V Framework

This directory is the Agile V verification and traceability workspace for the Aisphere IAM backend.

## Pilot objective

Cycle `C1` applies Agile V to an existing codebase. The first objective is **requirements recovery from implementation**:

1. understand the current system before changing it;
2. recover candidate requirements from Proto contracts, service/biz/data implementation, authorization schema, tests and CI;
3. identify mismatches between documented architecture, API contracts and executable implementation;
4. establish a requirement → implementation → test evidence chain;
5. avoid declaring a capability release-ready without runtime evidence.

## Framework structure

```
.agile-v/
├── README.md                          ← This file
├── STATE.md                           ← Current phase/stage/status
├── DECISION_LOG.md                    ← Append-only decision log
├── APPROVALS.md                       ← Human Gate approval records
├── CONTROL_MATRIX.yaml                ← Operating control map
├── POLICY.yaml                        ← Policy-as-code
├── TRACE_LOG.md                       ← Append-only trace spans
├── EVAL_RESULTS.md                    ← Eval flywheel + eval_gate_status
├── CHECKPOINTS.md                     ← Durable Human Gate interrupts
├── RISK_REGISTER.md                   ← Risk register
├── change_requests/
│   └── CR-0001-recover-iam-requirements.md
├── understanding/
│   ├── system_overview.md
│   └── understanding_gate_decision.md
├── requirements/
│   └── requirements.md
├── traceability/
│   ├── implementation_traceability_matrix.md
│   └── traceability_gaps.md
├── phases/
│   ├── 01-specify/   (PLAN.md, SUMMARY.md)
│   ├── 02-constrain/ (PLAN.md, SUMMARY.md)
│   ├── 03-orchestrate/ (PLAN.md, SUMMARY.md)
│   ├── 04-prove/     (PLAN.md, SUMMARY.md)
│   ├── 05-evolve/    (PLAN.md, SUMMARY.md)
│   └── 06-verify/    (PLAN.md, SUMMARY.md)
└── cycles/
    └── C1/           (frozen archive of C1 artifacts)
```

## SCOPE-V Phases

| Phase | Purpose | Status |
|-------|---------|--------|
| 01-Specify | Convert intent into traceable requirements | ✅ COMPLETE |
| 02-Constrain | Apply domain constraints and validation | ⏳ PENDING |
| 03-Orchestrate | Synthesize artifacts from approved REQs | ⏳ PENDING |
| 04-Prove | Provide evidence per risk level | ⏳ PENDING |
| 05-Evolve | Learn from failures, update knowledge | ⏳ PENDING |
| 06-Verify | Independent verification against REQs | ⏳ PENDING |

## Gates

| Gate | Status |
|------|--------|
| Gate 0 — System Understanding | ✅ PASS_WITH_FINDINGS |
| Gate 1 — Requirement Approval | ⏳ PENDING |
| Gate 2 — Verification Evidence | ❌ NOT_STARTED |

## Requirement statuses

| Status | Meaning |
|--------|---------|
| OBSERVED_IMPLEMENTED | Executable implementation was found |
| PARTIAL_IMPLEMENTATION | Some layers exist, but full path is incomplete |
| CONTRACT_ONLY | Proto/API contract exists, but implementation is missing |
| ARCHITECTURE_REQUIRED | Required by architecture, but main does not comply |
| UNIT_EVIDENCE | Unit or contract test evidence exists |
| INTEGRATION_EVIDENCE | Real dependency or end-to-end evidence exists |
| RELEASE_READY | Full evidence chain complete |
| DEPRECATED | Must not remain in target product contract |