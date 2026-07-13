# CR-0001 — Recover IAM Requirements from Existing Implementation

## Status

`IN_PROGRESS [C1]`

## Stakeholder directive

- Requested by: Aisphere project owner
- Date: 2026-07-13
- Directive: Apply Agile V to the IAM backend as a pilot, beginning by reverse-engineering requirements from the existing implementation.

## Problem statement

The IAM backend has grown across identity directory integration, authorization projection, runtime permission APIs, control-plane resources, grants, Gateway integration and deployment automation. Architecture documents alone no longer provide a reliable view of:

- what is actually implemented;
- which Proto surfaces form complete business flows;
- which capabilities have test evidence;
- which APIs conflict with the accepted architecture;
- which capabilities are safe to release.

## Objective

Create a reviewable initial source of truth connecting:

```text
observed behavior
→ candidate requirement
→ implementation evidence
→ existing test evidence
→ verification gap
```

## Scope

### Included

- IAM AuthN adapter and principal handling;
- Casdoor-backed directory reads and writes;
- identity mode boundaries;
- Group and membership authorization projection;
- runtime authorization APIs and gRPC client;
- authorization administration APIs;
- Project and Capability control plane;
- generic Resource control plane;
- Role Template and Grant lifecycle;
- Proto/access/audit contracts;
- build, CI and deployment verification gates.

### Excluded from C1

- changing IAM business behavior;
- merging or replacing PR #40;
- generating missing integration tests;
- declaring production readiness;
- frontend requirements recovery;
- Hub, Runtime or Gateway repository changes.

## Inputs inspected

- `README.md`
- `docs/architecture-boundaries.md`
- `api/iam/v1/iam.proto`
- `api/iam/v1/identity_admin.proto`
- `api/iam/project/v1/project.proto`
- service implementations under `internal/service/`
- identity projection implementation under `internal/data/identity_mode.go`
- selected existing tests and CI workflow
- recent merged and open pull requests, especially PR #40

## Required outputs

- Gate 0 system overview;
- Gate 0 decision and confidence;
- candidate requirement catalogue with stable `REQ-IAM-*` identifiers;
- initial implementation traceability matrix;
- explicit list of traceability and verification gaps.

## Acceptance criteria

- Every recovered requirement states observable behavior and verification criteria.
- Architecture intent is not confused with current main-branch behavior.
- Unimplemented RPCs are marked as gaps, not completed capabilities.
- Existing unit tests are linked where found.
- Missing Casdoor, SpiceDB, PostgreSQL, Gateway and end-to-end evidence is explicitly recorded.
- No production-readiness claim is made from compilation or unit tests alone.

## Human Gate 1 decision

Pending review of `.agile-v/requirements/requirements.md`.
