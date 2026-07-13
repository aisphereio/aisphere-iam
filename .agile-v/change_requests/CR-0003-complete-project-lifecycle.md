# CR-0003 — Complete Project Lifecycle

## Status

`IMPLEMENTED_PENDING_VERIFICATION [C3]`

## Scope

- implement Project update and archive operations that were previously exposed but returned `Unimplemented`;
- use `FieldMask` for explicit PATCH semantics, including field clearing;
- persist and return Project metadata;
- preserve immutable Project identity, Zone, slug, creator and ownership projection;
- enforce Principal Zone scope on Project reads and Project Capability operations;
- reject updates and capability mutations after archive;
- make archive idempotent;
- add business, service and contract regression tests.

## Mutable fields

- `display_name`
- `description`
- `visibility`
- `labels`
- `annotations`
- `metadata`

`org_id`, `slug`, `created_by`, owner relationships and lifecycle timestamps are not client-mutable through UpdateProject.

## Acceptance criteria

- `UpdateProject` and `ArchiveProject` no longer return `Unimplemented`;
- omitted update mask preserves backward-compatible non-zero-field patch behavior;
- explicit mask supports clearing description/maps/metadata;
- unknown or immutable mask paths are rejected;
- cross-Zone Project access is rejected;
- archived Project update and capability mutation are rejected;
- archive is idempotent;
- generation, contract checks, all Go tests, build and generated drift checks pass.
