# API Overview

IAM's API surface is defined by protobuf contracts and generated into gRPC, HTTP (grpc-gateway), authz policies, gateway manifests, and OpenAPI specs.

## Proto contracts

### `api/iam/v1/iam.proto`

The primary API contract defining 7 services:

| Service | Exposure | Key RPCs |
|---|---|---|
| `IAMAuthService` | INTERNAL / AUTHENTICATED | `VerifyToken`, `GetMe` |
| `IAMDirectoryService` | AUTHORIZED | `GetUser`, `ListUsers`, `GetOrganization`, `ListGroups`, `GetGroup` |
| `IAMPermissionService` | INTERNAL | `CheckPermission`, `BatchCheckPermissions`, `WriteRelationships`, `DeleteRelationships`, `ReadRelationships`, `LookupResources`, `LookupSubjects` |
| `IAMAuthorizationAdminService` | AUTHORIZED | `ReadSchema`, `PublishSchema`, `ExplainPermission`, `CheckRelationshipDrift` |
| `ProjectService` | AUTHORIZED | `CreateOrganization`, `CreateProject`, `RegisterCapability`, `SetProjectCapability` |
| `ResourceService` | AUTHORIZED | `RegisterResourceType`, `UpsertResource`, `BindResource`, `BindExternalResource` |
| `GrantService` | AUTHORIZED | `RegisterRoleTemplate`, `GrantAccess`, `RevokeAccess`, `ExplainAccess` |

### `api/iam/v1/identity_admin.proto`

Defines `IAMIdentityAdminService` with user and group CRUD operations:

| RPC | Exposure | Risk |
|---|---|---|
| `CreateUser` | AUTHORIZED | high |
| `UpdateUser` | AUTHORIZED | high |
| `DisableUser` | AUTHORIZED | high |
| `DeleteUser` | AUTHORIZED | critical |
| `CreateGroup` | AUTHORIZED | high |
| `UpdateGroup` | AUTHORIZED | high |
| `DeleteGroup` | AUTHORIZED | high |
| `AssignUserToGroup` | AUTHORIZED | high |
| `RemoveUserFromGroup` | AUTHORIZED | high |

### Sub-protos

| Path | Contents |
|---|---|
| `api/iam/grant/v1/` | Grant-specific messages |
| `api/iam/project/v1/` | Project-specific messages |
| `api/iam/resource/v1/` | Resource-specific messages |

## Access policy annotations

Every RPC is annotated with `aisphere.access.v1.policy`:

```protobuf
option (aisphere.access.v1.policy) = {
  exposure: AUTHORIZED
  authz: { action: "read" resource: "iam:org:{org_id}:user:{user_id}" audience: "iam-service" mode: CHECK_ONLY }
  audit: { enabled: true event: "iam.user.get" risk: "low" }
};
```

### Exposure levels

| Level | Description |
|---|---|
| `PUBLIC` | No authn/authz required (e.g., healthz, login redirect) |
| `AUTHENTICATED` | Any authenticated user (e.g., GetMe) |
| `AUTHORIZED` | Authenticated + resource-level authorization check |
| `INTERNAL` | Only callable by trusted platform services (e.g., permission checks) |

### Authz modes

| Mode | Description |
|---|---|
| `CHECK_ONLY` | Generated middleware checks the policy against SpiceDB |
| `SELF_CHECK` | Skip generated check; service performs its own authorization |

## Generated code

The `buf.gen.yaml` generates from `api/`:

| Plugin | Output | Purpose |
|---|---|---|
| `protoc-gen-go` | `api/**/*.pb.go` | Go message types |
| `protoc-gen-go-grpc` | `api/**/*_grpc.pb.go` | gRPC server/client stubs |
| `protoc-gen-go-http` | `api/**/*_http.pb.go` | Kernel HTTP transport bindings |
| `protoc-gen-grpc-gateway` | `api/**/*.pb.gw.go` | grpc-gateway HTTP handlers |
| `protoc-gen-go-errors` | `api/**/*_errors.pb.go` | Error definitions |
| `protoc-gen-go-authz` | `api/**/*_authz.pb.go` | Authz policy metadata |
| `protoc-gen-go-gateway` | `api/**/*_gateway.pb.go` | Gateway API manifest generation |
| `protoc-gen-go-kernel` | `api/**/*_kernel.pb.go` | Kernel integration code |
| `protoc-gen-openapiv2` | `docs/openapi/` | OpenAPI specs |

## Gateway API manifests

`make deploy` generates Kubernetes Gateway API manifests from proto annotations:

| Directory | Listener |
|---|---|
| `deploy/generated/gateway/public/` | Public listener (no authn) |
| `deploy/generated/gateway/authenticated/` | Authenticated listener (OIDC required) |
| `deploy/generated/gateway/internal/` | Internal listener (cluster-local) |

## gRPC client for external services

The `client/authzgrpc` package provides a gRPC client for other Aisphere services to call IAM's permission service:

```go
client, err := authzgrpc.New(authzgrpc.Config{
    Endpoint: "aisphere-iam.aisphere:19080",
    Insecure: true,
})
decision, err := client.Check(ctx, authz.CheckRequest{
    Resource:   authz.ObjectRef{Type: "zone", ID: "aisphere"},
    Permission: "view_users",
    Subject:    authz.SubjectRef{Type: "user", ID: "user-uuid"},
})
```

The client automatically propagates the caller's principal via trusted headers.

## Key source files

| File | Purpose |
|---|---|
| `api/iam/v1/iam.proto` | Primary API contract |
| `api/iam/v1/identity_admin.proto` | Identity admin API contract |
| `buf.gen.yaml` | Code generation configuration |
| `buf.yaml` | Buf lint/breaking config |
| `client/authzgrpc/client.go` | gRPC client for external services |
| `internal/server/access.go` | Skip policy resolver for generated authz |