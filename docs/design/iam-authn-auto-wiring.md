# IAM AuthN Auto Wiring

IAM should not manually parse JWTs or trusted headers. It uses Kernel's automatic boundary runtime.

## Default mode

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
```

`internal/server/access.go` calls `securityx.NewRuntime` and mounts its generated middleware before accessx. In `gateway_trusted` mode this middleware restores `authn.Principal` from `x-aisphere-*` headers.

## Security note

In `gateway_trusted` mode, IAM disables the Gateway-to-backend internal token check (see `internal/server/access.go:37-40`). This is intentional because Envoy Gateway's `ClientTrafficPolicy` already strips spoofed headers. However, this means:

1. IAM Service must be ClusterIP only.
2. NetworkPolicy must allow only Envoy Gateway Pods to reach IAM.
3. Internal service calls must use a separate service token mechanism.