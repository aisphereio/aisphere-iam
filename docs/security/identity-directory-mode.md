# IAM Identity Directory Mode

IAM supports two identity-directory modes under `security.authn.identity_mode`.

## Modes

| Mode | Meaning | Runtime behavior |
| --- | --- | --- |
| `casdoor_local` | Casdoor is the local identity directory. | IAM may use `authn.IdentityAdmin` for user, identity organization, application, group, and group membership management after IAM/accessx/SpiceDB authorization. |
| `external_oidc` | The upstream user and identity organization source is federated and read-only from IAM's perspective. | IAM blocks user, identity organization, and application mutations before they reach the provider. Application-layer groups, multi-level groups, and user-to-group membership remain writable because IAM owns them for local authorization projection. |

An empty value defaults to `casdoor_local` for backward compatibility.

## Config shape

```yaml
security:
  authn:
    mode: gateway_trusted
    provider: casdoor
    identity_mode: casdoor_local
```

For a federated user directory with local application groups:

```yaml
security:
  authn:
    mode: gateway_trusted
    provider: casdoor
    identity_mode: external_oidc
```

## Boundary

This guard makes the intended product boundary enforceable in code:

```text
casdoor_local:
  IAM can manage local Casdoor users, identity organizations, applications,
  multi-level groups, and group membership.

external_oidc:
  IAM only reads and projects upstream users and identity organizations.
  IAM must not mutate upstream users or upstream identity organizations.
  IAM can still manage Aisphere-owned application groups and group membership.
```

## Why groups stay writable in external_oidc

Groups are not treated as upstream user-directory ownership in this mode. IAM uses groups as an application-layer authorization construct:

```text
external OIDC user
  -> IAM/Casdoor local application group membership
  -> SpiceDB relationship projection
  -> Aisphere resource authorization
```

This allows Customer EC / external OIDC deployments to keep users and identity organizations read-only while still supporting local application authorization groups, multi-level group trees, and group-based access control.

This is independent of Gateway route exposure. Gateway decides which generated proto routes are externally reachable; `identity_mode` decides whether identity mutations are allowed after the request reaches IAM.
