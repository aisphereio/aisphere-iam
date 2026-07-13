# CR-0004 — Close Resource Binding Lifecycle

## Status

`IMPLEMENTED_PENDING_VERIFICATION [C4]`

## Release decisions

- implement the complete Bind → List → Unbind lifecycle for internal resource relationships;
- implement filtered, paginated external-resource binding lookup;
- remove `MoveResource` and `DeleteResource` from the first release contract until hierarchy, dependent-resource, hard-delete and projection-compensation rules are approved;
- retain `ArchiveResource` as the supported non-destructive lifecycle operation.

## Acceptance criteria

- Unbind archives the binding fact and deletes the exact projected relationship through the durable outbox path;
- repeated Unbind is idempotent;
- binding lists honor source, target, relation, status and pagination filters;
- external binding lists honor resource/provider/external identity/sync status and pagination filters;
- generated routes no longer expose Move/Delete operations;
- no ResourceService RPC in the release contract returns `Unimplemented`;
- generation, contract checks, all Go tests, build, drift and container build pass.
