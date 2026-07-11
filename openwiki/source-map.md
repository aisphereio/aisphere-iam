# Source Map

## Top-level layout

```
aisphere-iam/
├── api/                    # Proto contracts and generated code
│   ├── iam/v1/              # Primary IAM API (iam.proto, identity_admin.proto)
│   ├── iam/grant/v1/        # Grant-specific messages
│   ├── iam/project/v1/      # Project-specific messages
│   └── iam/resource/v1/     # Resource-specific messages
├── client/
│   └── authzgrpc/           # gRPC client for external services
├── cmd/
│   └── aisphere-iam/        # Application entrypoint (main.go)
├── configs/
│   ├── config.yaml          # Default configuration
│   ├── config.local.yaml    # Local development configuration
│   ├── casdoor.pub          # Casdoor public key (local dev fallback)
│   ├── resource/
│   │   └── defaults.yaml    # Built-in control-plane defaults
│   └── spicedb/
│       ├── aisphere.schema.zed  # SpiceDB ReBAC schema
│       └── README.md            # Schema workflow documentation
├── deploy/
│   ├── configmap.yaml       # Kubernetes ConfigMap
│   ├── config.yaml          # Deployment config reference
│   ├── deployment.yaml      # Kubernetes Deployment
│   ├── examples/            # Example Gateway security policies
│   ├── generated/
│   │   └── gateway/         # Generated Gateway API manifests
│   ├── kustomization.yaml   # Kustomization
│   ├── namespace.yaml       # Namespace
│   ├── networkpolicy.yaml   # Network security policy
│   ├── secret.yaml          # Secret template
│   └── service.yaml         # ClusterIP Service
├── docs/
│   ├── archive/             # Archived documentation
│   ├── deploy-architecture.md  # Frontend/backend deployment architecture
│   ├── deploy.md            # Deployment guide
│   ├── design/              # Design documents
│   ├── envoy-casdoor-oidc.md  # Envoy Gateway + Casdoor OIDC integration
│   ├── iam-authz-spicedb-bootstrap.md  # SpiceDB bootstrap model
│   ├── kernel-compliance.md # Kernel framework compliance
│   ├── openapi/             # Generated OpenAPI specs
│   ├── run-local.md         # Local development guide
│   └── security/            # Security documentation (archived)
├── internal/
│   ├── biz/                 # Business logic (use cases)
│   │   ├── defaults/        # Defaults reconciliation
│   │   ├── grant/           # Role template and grant use cases
│   │   ├── graph/           # ReBAC projection adapter
│   │   ├── idgen/           # ID generation
│   │   ├── project/         # Organization/project/capability use cases
│   │   ├── projection/      # Outbox event → SpiceDB projection
│   │   └── resource/        # Resource type and instance use cases
│   ├── conf/                # Configuration structs
│   ├── data/                # Data layer (DB, authn, authz, cache, etc.)
│   ├── server/              # HTTP/gRPC server construction, middleware
│   └── service/             # Service layer (proto implementation)
├── migrations/              # Database migrations (goose)
├── .github/workflows/       # CI/CD workflows
│   ├── docker-acr.yml       # Docker build and push
│   └── openwiki-update.yml  # Scheduled documentation update
├── buf.gen.yaml             # Buf code generation config
├── buf.gen.deploy.yaml      # Buf deploy generation config
├── buf.yaml                  # Buf lint/breaking config
├── Dockerfile               # Container build
├── go.mod / go.sum          # Go module dependencies
└── Makefile                 # Build, test, deploy targets
```

## Key files by concern

### Entrypoint
| File | Purpose |
|---|---|
| `cmd/aisphere-iam/main.go` | Application entrypoint, dependency wiring, server startup |

### Configuration
| File | Purpose |
|---|---|
| `internal/conf/conf.go` | Configuration struct definitions |
| `configs/config.yaml` | Default configuration |
| `configs/config.local.yaml` | Local development configuration |

### API contracts
| File | Purpose |
|---|---|
| `api/iam/v1/iam.proto` | Primary API contract (7 services) |
| `api/iam/v1/identity_admin.proto` | Identity admin API contract |
| `buf.gen.yaml` | Code generation configuration |

### Server
| File | Purpose |
|---|---|
| `internal/server/http.go` | HTTP server construction |
| `internal/server/grpc.go` | gRPC server construction |
| `internal/server/access.go` | Middleware chain, skip policy resolver |
| `internal/server/identity_admin_auto.go` | Identity admin service wiring |

### Data layer
| File | Purpose |
|---|---|
| `internal/data/data.go` | Resource initialization (DB, authn, authz, cache) |
| `internal/data/authz_bootstrap.go` | SpiceDB schema bootstrap |
| `internal/data/identity_mode.go` | Identity mode guard, projection dispatcher |

### Business logic
| File | Purpose |
|---|---|
| `internal/biz/project/service.go` | Organization/project/capability use cases |
| `internal/biz/resource/service.go` | Resource type and instance use cases |
| `internal/biz/grant/service.go` | Role template and grant use cases |
| `internal/biz/projection/manager.go` | Outbox event → SpiceDB projection |
| `internal/biz/graph/projector.go` | ReBAC relationship projection adapter |
| `internal/biz/defaults/loader.go` | Defaults file loading and reconciliation |
| `internal/biz/idgen/idgen.go` | ID generation |

### Service layer
| File | Purpose |
|---|---|
| `internal/service/iam.go` | IAMAuthService, IAMDirectoryService, IAMPermissionService, IAMAuthorizationAdminService, ProjectService, ResourceService, GrantService |
| `internal/service/directory_projection.go` | Directory projection operations |

### Client
| File | Purpose |
|---|---|
| `client/authzgrpc/client.go` | gRPC client for external services |

### Deployment
| File | Purpose |
|---|---|
| `deploy/deployment.yaml` | Kubernetes Deployment |
| `deploy/networkpolicy.yaml` | Network security policy |
| `deploy/secret.yaml` | Secret template |
| `deploy/configmap.yaml` | ConfigMap template |
| `Dockerfile` | Container build |
| `Makefile` | Build, test, deploy targets |

### SpiceDB
| File | Purpose |
|---|---|
| `configs/spicedb/aisphere.schema.zed` | ReBAC schema |
| `configs/spicedb/README.md` | Schema workflow documentation |

### Defaults
| File | Purpose |
|---|---|
| `configs/resource/defaults.yaml` | Built-in capabilities, resource types, role templates |

### Migrations
| File | Purpose |
|---|---|
| `migrations/000001_iam_resource_control_plane.sql` | Core control-plane tables |
| `migrations/000002_iam_local_users.sql` | Legacy local users |
| `migrations/000003_iam_orgs_add_soft_delete.sql` | Soft delete |
| `migrations/000004_iam_identity_mirror_and_audit.sql` | Identity mirror and audit |

### Tests
| File | Purpose |
|---|---|
| `internal/service/iam_test.go` | Service-layer tests |
| `internal/server/access_test.go` | Skip policy resolver tests |
| `internal/server/identity_admin_contract_test.go` | Authz policy contract tests |
| `internal/data/identity_mode_test.go` | Identity projection tests |
| `client/authzgrpc/client_test.go` | gRPC client tests |