# Operations

## Configuration

IAM uses the Kernel `configx` system: YAML config file + environment variable overrides.

### Config files

| File | Purpose |
|---|---|
| `configs/config.yaml` | Default config (production/CI). Secrets via env vars. |
| `configs/config.local.yaml` | Local development config. Hardcoded values for convenience. |

**Local development must use `config.local.yaml`** because `config.yaml` references environment variables for `client_secret`.

### Config structure (`internal/conf/conf.go`)

```go
type Bootstrap struct {
    Service      ServiceConfig      // name, version, env
    Server       ServerConfig       // HTTP + gRPC addresses, timeouts, CORS
    Log          logx.Config        // structured logging, access log, redact
    Data         DataConfig         // database, migration, cache, object store
    Security     SecurityConfig     // authn, authz, internal_call
    ControlPlane ControlPlaneConfig // defaults reconciliation, bootstrap admins
    Audit        AuditConfig        // audit store
    Metrics      MetricsConfig      // Prometheus metrics
    DTM          dtmx.Config        // distributed transaction manager
}
```

### Environment variables

| Variable | Config path | Purpose |
|---|---|---|
| `POSTGRES_DSN` | `data.database.config.dsn` | PostgreSQL connection string |
| `CASDOOR_CLIENT_ID` | `security.authn.casdoor.client_id` | Casdoor OIDC client ID |
| `CASDOOR_CLIENT_SECRET` | `security.authn.casdoor.client_secret` | Casdoor OIDC client secret |
| `CASDOOR_M2M_CLIENT_ID` | `security.authn.casdoor.admin.client_id` | Casdoor M2M admin client ID |
| `CASDOOR_M2M_CLIENT_SECRET` | `security.authn.casdoor.admin.client_secret` | Casdoor M2M admin client secret |
| `INTERNAL_TOKEN` | `security.internal_call.token` | Internal service token |
| `SPICEDB_TOKEN` | `security.authz.spicedb.preshared_key` | SpiceDB preshared key |
| `DTM_BRANCH_SECRET` | `dtm.branch_secret` | DTM branch callback secret |

## Local development

### Prerequisites

- Go 1.25+
- PostgreSQL (local or remote)
- Casdoor (local or remote)
- SpiceDB (local or remote)

### Setup

```powershell
# Install codegen tools
make tools

# Generate API code
make api

# Run proto checks
make proto-check

# Run tests
make test
```

### Run

```powershell
# Set environment variables
$env:POSTGRES_DSN="postgres://postgres:password@host:port/aisphere_iam?sslmode=disable"
$env:SPICEDB_TOKEN="your-spicedb-token"

# Start with local config
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

### Verify

```powershell
# Health check
curl http://127.0.0.1:18080/healthz

# Get current user
curl http://127.0.0.1:18080/v1/iam/me
```

## Build & Deploy

### Docker

```bash
# Build image
docker build -t aisphere-iam .

# Or use make
make docker
```

The Dockerfile:
1. Uses `golang:1.25.8-alpine` as builder
2. Runs `make tools && make api && make deploy` during build
3. Produces a minimal `alpine:3.20` runtime image
4. Exposes ports 18080 (HTTP), 19080 (gRPC), 19180 (metrics)

### Kubernetes manifests

| File | Kind | Purpose |
|---|---|---|
| `deploy/namespace.yaml` | Namespace | `aisphere` namespace |
| `deploy/configmap.yaml` | ConfigMap | IAM config YAML |
| `deploy/secret.yaml` | Secret | Credentials (placeholders) |
| `deploy/deployment.yaml` | Deployment | App deployment (1 replica) |
| `deploy/service.yaml` | Service | ClusterIP service |
| `deploy/networkpolicy.yaml` | NetworkPolicy | Restrict ingress to Envoy Gateway + same-namespace + Prometheus |
| `deploy/kustomization.yaml` | Kustomization | Resource grouping |
| `deploy/generated/gateway/` | HTTPRoute, etc. | Generated Gateway API manifests |

### Deploy

```bash
make tools
make api
make deploy
make deploy-apply
```

`make deploy-apply` runs:
```bash
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -R -f deploy/generated
```

### SpiceDB schema deployment

The SpiceDB schema is managed as a ConfigMap:

```bash
make spicedb-schema-configmap
```

This creates/updates `aisphere-iam-spicedb-schema` ConfigMap from `configs/spicedb/aisphere.schema.zed`. The IAM deployment mounts it at `/app/configs/spicedb/aisphere.schema.zed`.

## CI/CD

### Docker build (`docker-acr.yml`)

- Trigger: push to `main`, tags `v*`, PRs to `main`
- Builds and pushes to Aliyun ACR (`registry.cn-beijing.aliyuncs.com/ainfracn/aisphere-iam`)
- Uses Aliyun Docker Hub mirror for faster base image pulls
- Tags: `latest`, branch, tag, PR, semver, commit SHA

### OpenWiki update (`openwiki-update.yml`)

- Scheduled daily at 08:00 UTC
- Runs `openwiki code --update --print`
- Creates a PR with updated documentation

## Database migrations

Migrations use **goose** and live in `migrations/`:

| Migration | Description |
|---|---|
| `000001_iam_resource_control_plane.sql` | Core tables: organizations, projects, capabilities, resource types, resources, bindings, role templates, grants, outbox events |
| `000002_iam_local_users.sql` | Local user table (legacy) |
| `000003_iam_orgs_add_soft_delete.sql` | Soft delete for organizations |
| `000004_iam_identity_mirror_and_audit.sql` | Identity mirror and audit tables |

Migrations are applied automatically at startup when `data.migration.enabled: true`.

## Security

### NetworkPolicy

The `deploy/networkpolicy.yaml` restricts IAM ingress to:
- Envoy Gateway pods (ports 18080, 19080)
- Same-namespace pods (port 18080, for DTM callbacks)
- Prometheus pods (port 19180)

### Secret management

Secrets are stored in `deploy/secret.yaml` with placeholder values. In production, these should be managed externally (e.g., SealedSecrets, External Secrets Operator, or manual replacement).

### Internal call token

In `gateway_trusted` mode, the internal call token is **disabled** — trust is provided by network isolation (NetworkPolicy). This is enforced in `internal/server/access.go`:

```go
if strings.EqualFold(cfg.Authn.Mode, securityx.AuthnModeGatewayTrusted) {
    internalCall.Enabled = false
    internalCall.Token = ""
}
```

## Key source files

| File | Purpose |
|---|---|
| `configs/config.yaml` | Default configuration |
| `configs/config.local.yaml` | Local development configuration |
| `deploy/deployment.yaml` | Kubernetes Deployment |
| `deploy/networkpolicy.yaml` | Network security policy |
| `deploy/secret.yaml` | Secret template |
| `deploy/configmap.yaml` | ConfigMap template |
| `Dockerfile` | Container build |
| `Makefile` | Build, test, deploy targets |
| `migrations/*.sql` | Database migrations |
| `.github/workflows/docker-acr.yml` | CI/CD pipeline |
| `.github/workflows/openwiki-update.yml` | Documentation update workflow |