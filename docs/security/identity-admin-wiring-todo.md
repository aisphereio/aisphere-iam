# Identity Admin Wiring TODO

This branch adds the service implementation but leaves the final generated wiring for local completion after `make api`.

## Local follow-up

1. Create the service in `cmd/aisphere-iam/main.go`:

```go
identityAdminService := service.NewIAMIdentityAdminService(deps)
```

2. Add the generated Gateway module to route registration:

```go
v1.IAMIdentityAdminServiceKernelModule(),
```

3. Pass the service to HTTP server construction and register it in `internal/server/http.go`:

```go
v1.RegisterIAMIdentityAdminServiceHTTPServer(srv, identityAdminSvc)
```

4. Pass the service to gRPC server construction and register it in `internal/server/grpc.go`:

```go
v1.RegisterIAMIdentityAdminServiceServer(srv, identityAdminSvc)
```

5. Run local generation and verification:

```bash
make api
make proto-check
make test
make build
```
