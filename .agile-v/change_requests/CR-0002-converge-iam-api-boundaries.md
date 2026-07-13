# CR-0002 — Converge IAM API Security Boundaries

## Status

`IMPLEMENTED_PENDING_GATE_2 [C2]`

## Approved Gate 1 decisions

1. Casdoor Organization → SpiceDB Zone is the single root model; PR #40 closed the legacy Organization control-plane gap.
2. `IAMDirectoryService` is the read-only directory facade.
3. `IAMIdentityAdminService` is the canonical User/Group mutation surface.
4. Group authorization resources use Organization-qualified IDs: `group:<org_id>/<group_id>`.
5. Raw Relationship mutation is not a product-facing capability. Runtime projection keeps INTERNAL plural APIs; administrative repair remains under `IAMAuthorizationAdminService` with global repair permission.

## Changes

- remove Group and membership writes from `IAMDirectoryService`;
- implement the missing read-only `GetGroup` adapter;
- align directory read policies to Zone permissions and service-level checks;
- remove AUTHORIZED singular Relationship write/delete RPCs;
- retain INTERNAL plural relationship projection APIs;
- fix membership removal to delete the Organization-qualified Group relationship;
- add regression and projection tests;
- regenerate Proto, transport, Gateway and deployment artifacts.

## Acceptance criteria

- generated public routes contain one Group write surface only;
- no public/authorized singular raw tuple mutation route exists;
- directory read policies contain no removed `iam:org:*` resource;
- removing membership deletes `group:<org_id>/<group_id>#member@user:<id>`;
- `make proto-check`, `go test ./...` and `make build` pass;
- generated-file drift is clean.

## Remaining Gate 2 work

Real Casdoor, SpiceDB, PostgreSQL and Envoy Gateway integration evidence remains required before release readiness.
