# Legacy Local Users API

The plain HTTP `/v1/users` local-user API has been removed from HTTP route registration.

## Why it was legacy

`/v1/users` was an early compatibility endpoint backed by IAM's local PostgreSQL user table. It was not generated from proto, did not participate in generated Gateway route metadata, and did not fit the current identity architecture.

## Current model

User and identity-directory management should use generated IAM proto contracts backed by Casdoor:

```text
Casdoor / OIDC
  -> IAMDirectoryService for identity reads
  -> IAMIdentityAdminService for identity writes
  -> AuthZ / SpiceDB for Aisphere resource permissions
```

`IAMIdentityAdminService` is the generated management surface for user and application-group operations:

```text
CreateUser / UpdateUser / DisableUser / DeleteUser
CreateGroup / UpdateGroup / DeleteGroup
AssignUserToGroup / RemoveUserFromGroup
```

The generated proto route carries `aisphere.access.v1.policy`, so protoc/kernel generation produces the HTTP binding, Gateway metadata, request-info resolver, access resolver, and audit metadata from the same contract.

Application-layer groups remain valid. They are managed through the identity admin path and projected into AuthZ relationships for resource authorization.

## Identity mode behavior

`casdoor_local`:

```text
User, identity organization, application, group, and group membership writes are allowed after AuthZ.
```

`external_oidc`:

```text
Upstream users and identity organizations are read-only.
Local application groups and group membership remain writable after AuthZ.
```

The runtime identity mode guard enforces that boundary after the generated AuthZ check passes.

## Follow-up cleanup

The route has been removed first to avoid exposing a second user-management surface. The remaining local-user repository and migrations can be removed in a later cleanup once no tests or development fixtures depend on them.

After this contract lands, run:

```bash
make api
make proto-check
make test
make build
```

Then wire the generated `IAMIdentityAdminService` server implementation into IAM HTTP/gRPC server registration and Gateway route registration.
