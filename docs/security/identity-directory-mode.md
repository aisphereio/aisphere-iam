# IAM Identity Directory Mode

IAM supports two identity-directory modes under `security.authn.identity_mode`.

## Modes

| Mode | Meaning | Runtime behavior |
| --- | --- | --- |
| `casdoor_local` | Casdoor is the local identity directory. | IAM may use `authn.IdentityAdmin` for normal user, organization, application, and group management after IAM/accessx/SpiceDB authorization. |
| `external_oidc` | The upstream identity source is federated and read-only from IAM's perspective. | IAM wraps `authn.IdentityAdmin` with a read-only adapter. Directory reads continue to work; user, organization, application, and group mutations fail before reaching the provider. |

An empty value defaults to `casdoor_local` for backward compatibility.

## Config shape

```yaml
security:
  authn:
    mode: gateway_trusted
    provider: casdoor
    identity_mode: casdoor_local
```

For a federated directory:

```yaml
security:
  authn:
    mode: gateway_trusted
    provider: casdoor
    identity_mode: external_oidc
```

## Why this exists

This guard makes the intended product boundary enforceable in code:

```text
casdoor_local:
  IAM can manage local Casdoor users, organizations, applications, and multi-level groups.

external_oidc:
  IAM only reads and projects users, organizations, and groups.
  IAM must not mutate the upstream identity source.
```

This is independent of Gateway route exposure. Gateway decides which generated proto routes are externally reachable; `identity_mode` decides whether identity mutations are allowed even after the request reaches IAM.
