# Identity Admin Auto Registration

Identity admin is now wired through the IAM server registration layer instead of being manually passed from `cmd/aisphere-iam/main.go`.

## Automatic registration points

1. HTTP server

```go
registerIdentityAdminHTTP(srv, resources)
```

`registerIdentityAdminHTTP` creates `IAMIdentityAdminService` from `resources.Identity` and registers the generated HTTP server when the identity provider is configured.

2. RPC server

```go
registerIdentityAdminRPC(srv, resources)
```

`registerIdentityAdminRPC` creates the same service from `resources.Identity` and registers the generated RPC server when the identity provider is configured.

3. Gateway route registry

```go
server.IAMGatewayModules()
```

`cmd/aisphere-iam/main.go` now asks the server package for the full generated IAM module catalog instead of listing service modules one by one.

## Local follow-up

`make api` is still required locally because `IAMIdentityAdminService` generated Go types come from `api/iam/v1/identity_admin.proto`.

```bash
make api
make proto-check
make test
make build
```

After generation, the identity admin service should be available in all three places automatically:

- HTTP binding
- RPC binding
- Gateway route registry
