# IAM Backend AuthN Mode

IAM uses Casdoor as the only token/session issuer. IAM does not mint platform JWTs.

## Default mode: gateway_trusted

IAM runs in `gateway_trusted` mode by default, trusting Envoy Gateway-injected `x-aisphere-*` principal headers:

```yaml
security:
  authn:
    mode: gateway_trusted
```

Use this only when NetworkPolicy prevents clients from bypassing Envoy Gateway.

## Security boundary

`gateway_trusted` requires:

1. IAM Service is ClusterIP only (no NodePort/LoadBalancer).
2. NetworkPolicy allows only Envoy Gateway Pods to reach IAM HTTP port.
3. Internal service calls use service token, not external Gateway headers.