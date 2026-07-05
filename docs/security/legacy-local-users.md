# Legacy Local Users API

The plain HTTP `/v1/users` local-user API has been removed from HTTP route registration.

## Why it was legacy

`/v1/users` was an early compatibility endpoint backed by IAM's local PostgreSQL user table. It was not generated from proto, did not participate in generated Gateway route metadata, and did not fit the current identity architecture.

## Current model

User and identity-directory management should use the IAM identity/directory contract backed by Casdoor:

```text
Casdoor / OIDC
  -> IAM identity directory facade
  -> AuthZ / SpiceDB for Aisphere resource permissions
```

Application-layer groups remain valid. They are managed through the identity admin path and projected into AuthZ relationships for resource authorization.

## Follow-up cleanup

The route has been removed first to avoid exposing a second user-management surface. The remaining local-user repository and migrations can be removed in a later cleanup once no tests or development fixtures depend on them.
