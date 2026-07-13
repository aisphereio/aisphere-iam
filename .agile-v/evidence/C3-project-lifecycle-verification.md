# C3 Project Lifecycle Verification Evidence

## Change request

`CR-0003 — Complete Project Lifecycle`

## Closed C1 findings

- `REQ-IAM-PROJECT-005 — Update a Project`: moved from `CONTRACT_ONLY` to implemented with automated unit/service/contract evidence.
- `REQ-IAM-PROJECT-006 — Archive a Project`: moved from `CONTRACT_ONLY` to implemented with automated unit/service/contract evidence.

## Implemented behavior

- `UpdateProject` uses `google.protobuf.FieldMask` for explicit PATCH behavior.
- Supported mutable fields are `display_name`, `description`, `visibility`, `labels`, `annotations`, and `metadata`.
- Unknown or immutable update-mask paths are rejected.
- Without a mask, non-zero fields retain backward-compatible patch behavior.
- Project metadata is persisted and returned.
- Project Zone, slug and creator identity remain immutable.
- Project reads and Project Capability operations require the current Kernel Principal's `org_id` to match the Project Zone.
- Archive is idempotent.
- Archived Projects reject further updates and Capability mutations.
- The in-memory control-plane repository now conforms to the production repository's variadic outbox contract, allowing it to be used in lifecycle tests.

## Repository gate executed successfully

A GitHub Actions verification runner executed and passed:

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

## Additional mainline issue corrected

The main branch contained `IMPORT_NO_UNUSED` in `buf.yaml` as an excluded lint rule, but that rule identifier is not supported by the installed Buf version. C3 removes the invalid exception. Unused Proto imports must be corrected in source rather than suppressed through a non-existent rule.

## Evidence added

- `internal/biz/project/lifecycle_test.go`
  - validates mutable fields;
  - preserves immutable Project identity;
  - validates idempotent archive;
  - rejects archived update and Capability mutation;
  - rejects cross-Zone access.
- `internal/service/project_lifecycle_test.go`
  - validates FieldMask clearing and metadata response;
  - validates creator response mapping;
  - validates cross-Zone denial;
  - rejects unsupported/immutable mask paths.
- `internal/service/project_lifecycle_contract_test.go`
  - prevents Project lifecycle stubs from returning;
  - requires the update-mask contract.

## Gate decision

`C3 SOURCE_AND_BUILD_GATE_PASS`

This is not yet production release evidence. PostgreSQL migration/runtime behavior, deployed HTTP/gRPC behavior, durable audit records and real Gateway/SpiceDB integration remain Gate 2 work.
