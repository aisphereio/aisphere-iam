# C4 Resource Binding Lifecycle Verification Evidence

## Change request

`CR-0004 — Close Resource Binding Lifecycle`

## Release contract decision

`MoveResource` and `DeleteResource` were removed from the first-release API contract rather than given incomplete implementations. Their correct behavior depends on unresolved product and consistency rules for hierarchy cycles, descendants, hard deletion, external systems, grants and SpiceDB compensation.

`ArchiveResource` remains the supported non-destructive Resource lifecycle operation.

## Implemented binding closure

- Resource bindings can be listed by source, target, relation and status with stable pagination fields.
- Unbind loads the original binding, reconstructs the exact SpiceDB relationship, persists an archived binding state and delete outbox event in one repository transaction, then dispatches the relationship deletion.
- Unbind is idempotent for an already archived binding.
- Delete projection payload retains the original relationship for compensation.
- External resource bindings can be listed by internal Resource, provider, external type, external ID and sync status with pagination.
- PostgreSQL and in-memory repositories implement the same binding lifecycle/query contract.

## Automated verification

The stacked PR verification runner completed successfully:

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

## Regression evidence

- `internal/biz/resource/lifecycle_test.go`
  - proves Bind writes a relationship;
  - proves Unbind archives the fact and deletes the relationship;
  - proves repeated Unbind is safe;
  - proves filtered external-binding lookup.
- `internal/service/resource_binding_contract_test.go`
  - prevents Move/Delete RPCs from returning before their requirements are approved;
  - requires Unbind and external-binding lookup in the release contract;
  - prevents Resource lifecycle `Unimplemented` stubs.

## Gate decision

`C4 SOURCE_AND_BUILD_GATE_PASS`

This closes the Resource `CONTRACT_ONLY` findings for the first release contract. Real PostgreSQL transaction rollback, SpiceDB failure recovery, deployed Gateway exposure and audit evidence remain Gate 2 requirements.
