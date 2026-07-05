# IAM AuthN Auto Wiring

IAM should not manually parse JWTs or trusted headers. It uses Kernel's
automatic boundary runtime.

## Default mode

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
  internal_call:
    enabled: true
    header: X-Aisphere-Internal-Token
    token: "${GATEWAY_TO_IAM_INTERNAL_TOKEN}"
```

`internal/server/access.go` calls `securityx.NewAuthnBoundaryRuntime` and mounts
its generated middleware before accessx. In `gateway_trusted` mode this
middleware validates the Gateway internal token and restores `authn.Principal`
from `X-Aisphere-*` headers.

Authz can stay `dev_allow_all` while testing the AuthN chain, then be switched
back to IAM AuthzService/SpiceDB.
