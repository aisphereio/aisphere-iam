# IAM Backend AuthN Mode

IAM still uses Casdoor as the only token/session issuer. IAM does not mint platform JWTs.

For the full-flow authn test, IAM can run in backend JWT verification mode:

```yaml
security:
  authn:
    mode: casdoor_jwt
```

In this mode IAM verifies the same Casdoor access token forwarded by Gateway. It uses Kernel `authn/oidcx`, validates issuer/audience/expiry/owner, and then executes the IAM handler.

For normal internal deployment, IAM can switch back to:

```yaml
security:
  authn:
    mode: gateway_trusted
```

In that mode IAM trusts Gateway-injected `X-Aisphere-*` principal headers. Use this only when network policy / mTLS prevents clients from bypassing Gateway.

AuthZ can be short-circuited for authn testing with:

```yaml
security:
  authz:
    dev_allow_all: true
```
