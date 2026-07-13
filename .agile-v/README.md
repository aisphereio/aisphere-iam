# Aisphere IAM — Agile V Pilot

This directory is the verification and traceability workspace for the Aisphere IAM backend.

## Pilot objective

Cycle `C1` applies Agile V to an existing codebase. The first objective is **requirements recovery from implementation**:

1. understand the current system before changing it;
2. recover candidate requirements from Proto contracts, service/biz/data implementation, authorization schema, tests and CI;
3. identify mismatches between documented architecture, API contracts and executable implementation;
4. establish a requirement → implementation → test evidence chain;
5. avoid declaring a capability release-ready without runtime evidence.

This cycle does **not** change IAM business behavior.

## Baseline

- Repository: `aisphereio/aisphere-iam`
- Baseline branch: `main`
- Baseline commit when Cycle C1 started: `46c8785861392c15388b250e6ae6c245efb6bdc9`
- Recovery change request: `CR-0001`
- Important in-flight change: PR `#40` removes the legacy second Organization control-plane model. C1 records the main-branch behavior separately from the intended architecture.

## Artifact layout

```text
.agile-v/
├── README.md
├── change_requests/
│   └── CR-0001-recover-iam-requirements.md
├── understanding/
│   ├── system_overview.md
│   └── understanding_gate_decision.md
├── requirements/
│   └── requirements.md
└── traceability/
    ├── implementation_traceability_matrix.md
    └── traceability_gaps.md
```

## Requirement recovery statuses

| Status | Meaning |
|---|---|
| `OBSERVED_IMPLEMENTED` | Executable implementation was found. Runtime behavior still needs evidence unless explicitly linked. |
| `PARTIAL_IMPLEMENTATION` | Some layers exist, but the full business path is incomplete or an RPC is unimplemented. |
| `CONTRACT_ONLY` | Proto/API contract exists, but implementation evidence is missing or incomplete. |
| `ARCHITECTURE_REQUIRED` | Required by the accepted architecture contract, but current `main` conflicts with it. |
| `UNIT_EVIDENCE` | Unit or contract test evidence exists. |
| `INTEGRATION_EVIDENCE` | Real dependency or end-to-end execution evidence exists. |
| `RELEASE_READY` | Requirement, implementation, negative cases, integration evidence and operational gates are complete. |
| `DEPRECATED` | Surface exists historically but must not remain part of the target product contract. |

`OBSERVED_IMPLEMENTED` is not equivalent to `RELEASE_READY`.

## Agile V gates used by this pilot

### Gate 0 — System understanding

The repository must have a reviewable system overview, dependency map, known constraints and an explicit confidence decision.

### Gate 1 — Requirement approval

Recovered requirements are initially `Candidate`. They become `Approved` only after business and architecture review. Recovery must not silently convert accidental implementation behavior into a product requirement.

### Gate 2 — Verification evidence

Release readiness requires evidence from the applicable layers:

```text
Proto contract
→ service/biz/data implementation
→ unit/contract tests
→ Casdoor/SpiceDB/PostgreSQL integration tests
→ HTTP/gRPC/Gateway tests
→ audit/observability evidence
→ build and deployment gates
```

## Repository verification gate

The current repository-defined gate is:

```bash
make api
make deploy
make proto-check
go test ./... -count=1
make build
docker build .
```

Generated-file drift must also be clean. These checks prove build consistency; they do not by themselves prove the IAM business flows are production-ready.
