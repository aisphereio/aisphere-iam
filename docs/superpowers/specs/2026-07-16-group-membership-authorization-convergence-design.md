# Group Membership Authorization Convergence

## Problem

The organization workbench renders Casdoor groups as organization nodes under a zone. Membership mutations currently check `group:{org_id}/{group_id}#manage`.

This produces inconsistent behavior when:

1. an existing group is missing its zone-qualified SpiceDB topology, especially `group:{org_id}/{group_id}#zone@zone:{org_id}`;
2. a caller can create a group through `zone#create_groups`, but the production generated service path does not project the caller as `group#owner`;
3. a role grants the existing `group#manage_members` capability, while the membership RPC checks the broader `group#manage` permission.

The visible symptom is that the same operator can add members to some organization nodes but receives permission denied on others.

## Approaches Considered

### 1. Repair production relationships only

Run directory projection reconciliation for the affected zone without changing the service contract.

This repairs historical data but leaves the disconnected creator-owner path and the unused `manage_members` capability in place. Future role assignments can reproduce the problem.

### 2. Change membership RPCs to zone-level authorization

Authorize `AssignUserToGroup` and `RemoveUserFromGroup` with `zone:{org_id}#manage_groups`.

This avoids missing group topology during authorization, but discards valid group-scoped owner, manager, parent inheritance, and custom-role semantics. It also grants broader zone-level authority than the operation requires.

### 3. Converge topology, creator ownership, and membership semantics

This is the selected approach.

- Keep group-scoped resources with zone-qualified IDs.
- Make membership RPCs check `group#manage_members`.
- Preserve existing zone group-manager behavior by allowing `group#manage_members` through both `zone#manage_groups` and `zone#manage_users`.
- Infer the creator relationship from the authenticated Kernel principal inside the identity projection decorator, so every production caller uses the same owner projection path.
- Reconcile configured directory zones during startup through the existing projection dispatcher, with persisted retry when DTM or SpiceDB is temporarily unavailable.

This preserves least-privilege group-scoped authorization while repairing both existing and future data.

## Authorization Contract

The membership mutations use:

```text
group:{org_id}/{group_id}#manage_members@<current subject>
```

The SpiceDB permission becomes:

```zed
permission manage_members =
  owner
  + manager
  + zone->manage_groups
  + zone->manage_users
  + parent->manage_members
  + custom_binding->manage_members
```

Consequences:

- group owners and direct group managers can manage members;
- zone owners and administrators continue to inherit access;
- zone group managers retain the behavior they had through `group#manage`;
- zone user managers can manage membership without receiving group update or delete authority;
- custom roles can grant only `manage_members`;
- ordinary zone members receive no additional management permission.

`UpdateGroup` and `DeleteGroup` continue to require `group#manage`.

## Creator Ownership

`authzProjectingIdentityAdmin.CreateGroup` reads the authenticated principal from the request context and converts it to an AuthZ subject:

- `service` and `service_account` remain their respective subject types;
- all other authenticated principals become `user`;
- missing or anonymous principals do not create an owner relationship.

The existing `WithGroupOwner` test-only context seam is removed. Ownership is derived in the projection decorator, which is the common path for generated HTTP and gRPC services.

The projected relationships for a new group include:

```text
group:{org_id}/{group_id}#zone@zone:{org_id}
group:{org_id}/{group_id}#parent@group:{org_id}/{parent_id}   # when applicable
group:{org_id}/{group_id}#owner@<creator>
```

## Startup Directory Convergence

Add an explicit control-plane configuration:

```yaml
control_plane:
  directory_projection:
    reconcile_on_startup: true
    org_ids: [aisphere]
```

When enabled, resource initialization:

1. builds the desired directory relationships with the existing `BuildDirectoryProjectionRelationships`;
2. dispatches one idempotent write projection per configured organization;
3. logs submission failures without starting the HTTP/gRPC servers in a falsely healthy state when no durable retry store exists;
4. when the database-backed projection store exists, retains the failed event for the existing retry worker and allows startup to continue with a warning.

Configuration files used by local, test, and deployment workflows declare their intended organization IDs explicitly. No organization name is guessed from repository names or UI labels.

## Error Handling

- Invalid or empty startup organization IDs are ignored after trimming and deduplication.
- Failure to read Casdoor directory data is returned as a startup error because no repair event can be constructed.
- Dispatch failure is fatal when no database-backed retry store exists.
- Dispatch failure is non-fatal when the event is durably persisted; the retry worker owns recovery.
- Membership authorization denial remains a normal permission-denied response and does not attempt mutation-time relationship repair.

## Testing

Tests are written before production changes.

1. Generated access resolver tests prove membership RPCs require `manage_members`.
2. Schema/permission tests prove:
   - zone group managers retain membership access;
   - zone user managers gain membership-only access;
   - ordinary members do not gain access.
3. Service-to-projection tests create a group with a Kernel principal in context and prove the creator becomes `group#owner` without calling `WithGroupOwner`.
4. Startup convergence tests prove configured zones dispatch qualified `zone`, `parent`, and membership relationships.
5. Failure tests distinguish durable retry mode from non-durable startup failure.
6. Existing group CRUD, projection, generated contract, and permission-manifest tests remain green.

## Delivery

The change is proto-first:

1. update `group_admin.proto`;
2. run `make api`;
3. update the SpiceDB schema and resource contract tests;
4. implement creator ownership and startup convergence;
5. update Agile V requirement, artifact, and test traceability entries;
6. run `make proto-check`, targeted Go tests, `make traceability-check`, and relevant frontend generated-contract tests.

No hand-written replacement route or direct SpiceDB repair write is introduced.

## Success Criteria

- The same authorized operator can manage membership for every group in the configured zone, including groups created before zone-qualified projection was introduced.
- A group creator can immediately manage the created group when creator ownership is part of the request context.
- `manage_members` works without granting group update or delete authority.
- Startup automatically converges configured directory zones and has a durable recovery path.
- Existing frontend membership requests keep the same URL and payload contract.
