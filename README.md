# Aisphere Kernel Layout

Standalone full-feature service layout for `github.com/aisphereio/kernel/cmd/kernel`.

Default mode is **full**: config, logging, metrics, DB/cache/object storage wiring, audit, DTM, HTTP/gRPC transports, proto-first Todo CRUD, and governance code generation examples.

Use MVP when you want the smallest runnable service skeleton:

```powershell
kernel new skill-service --mvp
```

Use feature disable when you want to remove optional layout parts:

```powershell
kernel new skill-service --disable iam
kernel new skill-service --disable gateway,dtmx
```

## Use

```powershell
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel new skill-service
cd skill-service
make tools
make api
make proto-check
make test
make run
```

## Included defaults

- Features: `dbx,cachex,objectstorex,authn,authz,auditx,metricsx,logx,configx`
- Disabled features: `__KERNEL_DISABLED_FEATURES__`
- Profile: `__KERNEL_PROFILE__`
- DB: `dbx` with `postgres`
- Cache: `cachex` with `redis`
- Object storage: `objectstorex` with `minio`
- Authn: `casdoor`
- Authz: `spicedb`
- Audit: `auditx` memory recorder by default
- Logging: `logx` console output for local development
- Metrics: shared `metricsx.Manager`, optional admin `/metrics` server
- DTM: optional `dtmx.Manager`
- Config: `configx` file source
- Transports: Kernel HTTP and gRPC servers with access log and metrics hooks
- API example: protobuf-first Todo CRUD with HTTP binding and optional governance annotations
- Kernel version for generated Makefile tools: `__KERNEL_VERSION__`

External dependencies are present in `configs/config.yaml`, but DB, cache, object storage, authn, authz, and DTM are disabled by default so the service starts without local Postgres, Redis, Minio, Casdoor, SpiceDB, or DTM.

## Layout

```text
api/                 Protobuf APIs and generated HTTP/gRPC bindings
cmd/server/          Application entrypoint, renamed to cmd/<service> by kernel new
configs/             Local config with Kernel module defaults
internal/conf/        Config DTOs scanned by configx
internal/server/      Kernel HTTP and gRPC server construction
internal/service/     Transport-facing Todo service
internal/biz/         Use cases, domain contracts, errorx errors
internal/data/        Repositories and Kernel resource initialization
.kernel/              Layout profile/feature overlays consumed by kernel new
```

## Generate

```bash
make tools
make api
make proto-check
```

## Verify

```bash
make verify
```
