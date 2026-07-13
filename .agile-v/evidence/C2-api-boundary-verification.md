# C2 API Boundary Verification Evidence

## Change request

`CR-0002 — Converge IAM API Security Boundaries`

## Verified implementation scope

- `IAMDirectoryService` retains read-only User, Casdoor Organization and Group directory APIs.
- `IAMIdentityAdminService` is the sole generated User/Group mutation surface.
- Directory read policies use concrete `zone:<org_id>` permissions and service-level checks.
- AUTHORIZED singular raw Relationship write/delete RPCs are removed.
- INTERNAL plural Relationship projection APIs remain.
- AuthZ Admin relationship repair APIs remain protected by global admin permissions.
- `GetGroup` now delegates to the configured identity provider after `view_groups` authorization.
- User membership removal deletes the Organization-qualified `group:<org_id>/<group_id>#member@user:<id>` relationship.

## Automated verification executed on GitHub Actions

The C2 generation runner completed the following successfully before writing the verified product tree:

```text
make KERNEL_VERSION=v0.4.3 tools
make KERNEL_VERSION=v0.4.3 api
make KERNEL_VERSION=v0.4.3 deploy
go mod tidy
make KERNEL_VERSION=v0.4.3 proto-check
go test ./... -count=1
make build
git diff --check
```

The first contract-check attempt identified one obsolete `google/protobuf/empty.proto` import after removal of the Directory write RPCs. The import was removed and the complete verification sequence then passed.

## Regression evidence added

- `internal/service/api_surface_contract_test.go`
  - prevents Group writes from returning to `IAMDirectoryService`;
  - requires Group writes to remain in `IAMIdentityAdminService`;
  - prevents removed `iam:org:*` directory authorization resources;
  - prevents singular raw Tuple mutation RPCs from returning;
  - requires INTERNAL plural projection APIs.
- `internal/data/identity_membership_projection_test.go`
  - proves removal targets the Organization-qualified Group membership relationship.

## Gate decision

`C2 SOURCE_AND_BUILD_GATE_PASS`

This closes the API-boundary P0 findings from C1 at source, generated-contract, unit-test and build levels.

## Remaining Gate 2 evidence

The following remain required before IAM release readiness:

- real Casdoor directory and mutation tests;
- real SpiceDB permission and projection tests;
- PostgreSQL durable projection/retry tests;
- Envoy Gateway route exposure and header-spoofing tests;
- durable audit and observability verification;
- performance, multi-replica reliability and rollback validation.
